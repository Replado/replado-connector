package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const updatePublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEr4Y6gIDcSQkMIIAvWUkLo1MqUcd8
zlrzIbrVsUnCz2CSQuWrr83wTVl9gUJ3jgn4m9hGlaB9IZis4uGYoyXeKw==
-----END PUBLIC KEY-----`

const (
	updateManifestName = "update.json"
	updateFirstCheck   = 3 * time.Minute
	updateEvery        = 6 * time.Hour
	updateMaxManifest  = 64 << 10
	updateMaxAsset     = 40 << 20
	updateMaxRedirects = 5
	updateCheckTimeout = 20 * time.Second
)

var (
	updateBase      = "https://github.com/marcelglaeser/replado-connector/releases/latest/download/"
	updateHosts     = []string{"github.com", ".githubusercontent.com"}
	updateKey       = mustUpdateKey(updatePublicKeyPEM)
	updateClient    = newUpdateClient
	checkNewBinary  = runVersion
	restartAfter    = restartAfterUpdate
	updateSleep     = time.Sleep
	installedTarget = func() string { return filepath.Join(appDir(), exeName()) }
)

type manifest struct {
	Version string            `json:"version"`
	Files   map[string]string `json:"files"`
}

type ecdsaSignature struct {
	R, S *big.Int
}

func mustUpdateKey(p string) *ecdsa.PublicKey {
	block, _ := pem.Decode([]byte(p))
	if block == nil {
		panic("Update-Schlüssel fehlt")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		panic(err)
	}
	return key.(*ecdsa.PublicKey)
}

func allowedUpdateHost(host string) bool {
	host = strings.ToLower(host)
	for _, h := range updateHosts {
		if host == h || (strings.HasPrefix(h, ".") && strings.HasSuffix(host, h)) {
			return true
		}
	}
	return false
}

func newUpdateClient() *http.Client {
	cl := httpClient(5 * time.Minute)
	cl.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= updateMaxRedirects {
			return errf("Zu viele Weiterleitungen")
		}
		if req.URL.Scheme != "https" || !allowedUpdateHost(req.URL.Hostname()) {
			return errf("Weiterleitung zu %s nicht erlaubt", req.URL.Host)
		}
		return nil
	}
	return cl
}

func versionParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	var out []int
	for _, p := range strings.Split(v, ".") {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil
		}
		out = append(out, n)
	}
	return out
}

func newerVersion(candidate, current string) bool {
	a, b := versionParts(candidate), versionParts(current)
	if a == nil || b == nil {
		return false
	}
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func verifyManifest(data, sig []byte, key *ecdsa.PublicKey) (*manifest, error) {
	var s ecdsaSignature
	rest, err := asn1.Unmarshal(sig, &s)
	if err != nil || len(rest) > 0 || s.R == nil || s.S == nil {
		return nil, errf("Signatur des Updates ist ungültig")
	}
	h := sha256.Sum256(data)
	if !ecdsa.Verify(key, h[:], s.R, s.S) {
		return nil, errf("Signatur des Updates passt nicht")
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil || versionParts(m.Version) == nil {
		return nil, errf("Update-Beschreibung ist ungültig")
	}
	return &m, nil
}

func assetName() string {
	switch runtime.GOOS {
	case "windows":
		if strings.HasPrefix(runtime.Version(), "go1.10") {
			return "Replado-Connector-Vista.exe"
		}
		return "Replado-Connector.exe"
	case "darwin":
		return "Replado-Connector-macOS.zip"
	case "linux":
		return "replado-connector-linux-" + runtime.GOARCH + ".tar.gz"
	}
	return ""
}

func limitedRead(r io.Reader, max int64) ([]byte, error) {
	data, err := ioutil.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, errf("Datei ist zu groß")
	}
	return data, nil
}

func extractBinary(name string, data []byte) ([]byte, error) {
	binary := "replado-connector"
	switch {
	case strings.HasSuffix(name, ".exe"):
		return data, nil
	case strings.HasSuffix(name, ".tar.gz"):
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		tr := tar.NewReader(gz)
		for {
			h, err := tr.Next()
			if err != nil {
				return nil, errf("Programm fehlt im Archiv")
			}
			if path := filepath.Base(h.Name); path == binary && h.Typeflag == tar.TypeReg {
				return limitedRead(tr, updateMaxAsset)
			}
		}
	case strings.HasSuffix(name, ".zip"):
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) == binary && !f.FileInfo().IsDir() {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return limitedRead(rc, updateMaxAsset)
			}
		}
		return nil, errf("Programm fehlt im Archiv")
	}
	return nil, errf("Unbekanntes Format: %s", name)
}

func fetch(cl *http.Client, name string, max int64) ([]byte, error) {
	u, err := url.Parse(updateBase + name)
	if err != nil {
		return nil, err
	}
	resp, err := cl.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errf("%s: Status %d", name, resp.StatusCode)
	}
	return limitedRead(resp.Body, max)
}

func latestManifest(cl *http.Client) (*manifest, error) {
	data, err := fetch(cl, updateManifestName, updateMaxManifest)
	if err != nil {
		return nil, err
	}
	sig, err := fetch(cl, updateManifestName+".sig", 1024)
	if err != nil {
		return nil, err
	}
	return verifyManifest(data, sig, updateKey)
}

func runVersion(path string) (string, error) {
	c := exec.Command(path, "version")
	hideWindow(c)
	var out bytes.Buffer
	c.Stdout = &out
	if err := c.Start(); err != nil {
		return "", err
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return "", err
		}
	case <-time.After(updateCheckTimeout):
		c.Process.Kill()
		return "", errf("Neue Version antwortet nicht")
	}
	return strings.TrimSpace(out.String()), nil
}

func replaceBinary(target string, binary []byte, want string) error {
	fresh := target + ".neu"
	old := target + ".alt"
	if err := ioutil.WriteFile(fresh, binary, 0700); err != nil {
		return err
	}
	if got, err := checkNewBinary(fresh); err != nil || got != want {
		os.Remove(fresh)
		if err == nil {
			err = errf("Neue Datei meldet Version %q statt %s", got, want)
		}
		return err
	}
	os.Remove(old)
	if err := os.Rename(target, old); err != nil {
		os.Remove(fresh)
		return err
	}
	if err := os.Rename(fresh, target); err != nil {
		os.Rename(old, target)
		os.Remove(fresh)
		return err
	}
	return nil
}

type askFunc func(target string) (bool, error)

func applyUpdate(cl *http.Client, current string, ask askFunc) (string, error) {
	name := assetName()
	if name == "" {
		return "", nil
	}
	m, err := latestManifest(cl)
	if err != nil {
		return "", err
	}
	if !newerVersion(m.Version, current) {
		return "", nil
	}
	want, ok := m.Files[name]
	if !ok {
		return "", errf("Keine Datei %s in Version %s", name, m.Version)
	}
	allowed, err := ask(m.Version)
	if err != nil || !allowed {
		return "", err
	}
	asset, err := fetch(cl, name, updateMaxAsset)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(asset)
	if hex.EncodeToString(sum[:]) != strings.ToLower(want) {
		return "", errf("Prüfsumme von %s passt nicht", name)
	}
	binary, err := extractBinary(name, asset)
	if err != nil {
		return "", err
	}
	if err := replaceBinary(installedTarget(), binary, m.Version); err != nil {
		return "", err
	}
	return m.Version, nil
}

func askReplado(cl *http.Client) askFunc {
	return func(target string) (bool, error) {
		c := loadConfig()
		if c.Secret == "" {
			return false, nil
		}
		var r struct {
			Allowed bool `json:"allowed"`
		}
		if err := post(cl, c.Server+"/api/connector/v1/update", c.Secret, map[string]string{"version": version, "target": target}, &r); err != nil {
			return false, err
		}
		return r.Allowed, nil
	}
}

func installedHere() bool {
	self, err := os.Executable()
	return err == nil && samePath(self, installedTarget())
}

func checkForUpdate() {
	if !installedHere() || loadConfig().Secret == "" {
		return
	}
	updated, err := applyUpdate(updateClient(), version, askReplado(httpClient(resultTimeout)))
	if err != nil {
		log.Printf("Update: %v", err)
		return
	}
	if updated == "" {
		return
	}
	log.Printf("Aktualisiert von %s auf %s", version, updated)
	record(Entry{Kind: "update", Text: "Aktualisiert von " + version + " auf " + updated})
	restartAfter(installedTarget())
}

var (
	updateRunning  int32
	runUpdateCheck = checkForUpdate
)

func checkForUpdateOnce() bool {
	if !atomic.CompareAndSwapInt32(&updateRunning, 0, 1) {
		return false
	}
	defer atomic.StoreInt32(&updateRunning, 0)
	runUpdateCheck()
	return true
}

func updateLoop() {
	updateSleep(updateFirstCheck)
	for {
		checkForUpdateOnce()
		updateSleep(updateEvery)
	}
}

func waitForInstance() bool {
	for i := 0; i < 60; i++ {
		if singleInstance() {
			return true
		}
		time.Sleep(time.Second)
	}
	return false
}
