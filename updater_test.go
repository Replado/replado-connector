package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func signed(t *testing.T, key *ecdsa.PrivateKey, data []byte) []byte {
	h := sha256.Sum256(data)
	r, s, err := ecdsa.Sign(rand.Reader, key, h[:])
	if err != nil {
		t.Fatal(err)
	}
	sig, err := asn1.Marshal(ecdsaSignature{r, s})
	if err != nil {
		t.Fatal(err)
	}
	return sig
}

func packed(t *testing.T, name string, binary []byte) []byte {
	var buf bytes.Buffer
	switch {
	case strings.HasSuffix(name, ".tar.gz"):
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		tw.WriteHeader(&tar.Header{Name: "replado-connector", Mode: 0755, Size: int64(len(binary)), Typeflag: tar.TypeReg})
		tw.Write(binary)
		tw.Close()
		gz.Close()
	case strings.HasSuffix(name, ".zip"):
		zw := zip.NewWriter(&buf)
		w, _ := zw.Create("replado-connector")
		w.Write(binary)
		zw.Close()
	default:
		buf.Write(binary)
	}
	return buf.Bytes()
}

type updateServer struct {
	files    map[string][]byte
	requests []string
}

func (u *updateServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	u.requests = append(u.requests, name)
	data, ok := u.files[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Write(data)
}

type updateFixture struct {
	server *updateServer
	dir    string
	target string
	key    *ecdsa.PrivateKey
	asked  []string
	close  func()
}

func newUpdateFixture(t *testing.T, newVersion string, binary []byte) *updateFixture {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	asset := packed(t, assetName(), binary)
	sum := sha256.Sum256(asset)
	m, _ := json.Marshal(manifest{Version: newVersion, Files: map[string]string{assetName(): hex.EncodeToString(sum[:])}})
	us := &updateServer{files: map[string][]byte{"update.json": m, "update.json.sig": signed(t, key, m), assetName(): asset}}
	srv := httptest.NewServer(us)
	dir, err := ioutil.TempDir("", "update")
	if err != nil {
		t.Fatal(err)
	}
	f := &updateFixture{server: us, dir: dir, target: filepath.Join(dir, exeName()), key: key}
	ioutil.WriteFile(f.target, []byte("alt"), 0700)

	oldBase, oldKey, oldTarget, oldCheck := updateBase, updateKey, installedTarget, checkNewBinary
	updateBase = srv.URL + "/"
	updateKey = &key.PublicKey
	installedTarget = func() string { return f.target }
	checkNewBinary = func(path string) (string, error) {
		data, _ := ioutil.ReadFile(path)
		return strings.TrimPrefix(string(data), "programm "), nil
	}
	f.close = func() {
		updateBase, updateKey, installedTarget, checkNewBinary = oldBase, oldKey, oldTarget, oldCheck
		srv.Close()
		os.RemoveAll(dir)
	}
	return f
}

func (f *updateFixture) ask(allowed bool) askFunc {
	return func(target string) (bool, error) {
		f.asked = append(f.asked, target)
		return allowed, nil
	}
}

func (f *updateFixture) content(t *testing.T) string {
	data, err := ioutil.ReadFile(f.target)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestNewerVersionComparesNumbers(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.4.0", "0.3.1", true},
		{"0.10.0", "0.9.9", true},
		{"v1.0", "0.9", true},
		{"0.3.1", "0.3.1", false},
		{"0.3.0", "0.3.1", false},
		{"0.3.1.1", "0.3.1", true},
		{"0.4.0-beta", "0.3.1", false},
		{"", "0.3.1", false},
	}
	for _, c := range cases {
		if got := newerVersion(c.a, c.b); got != c.want {
			t.Errorf("%s > %s: %v", c.a, c.b, got)
		}
	}
}

func TestOnlyGitHubHostsAreAllowedForDownloads(t *testing.T) {
	for host, want := range map[string]bool{
		"github.com":                           true,
		"objects.githubusercontent.com":        true,
		"release-assets.githubusercontent.com": true,
		"githubusercontent.com.evil.example":   false,
		"evilgithub.com":                       false,
		"github.com.evil.example":              false,
		"replado.com":                          false,
	} {
		if allowedUpdateHost(host) != want {
			t.Errorf("%s: erwartet %v", host, want)
		}
	}
}

func TestRealUpdateKeyIsAnECDSAKey(t *testing.T) {
	if updateKey == nil || updateKey.Curve != elliptic.P256() {
		t.Fatal("öffentlicher Schlüssel fehlt oder ist kein P-256")
	}
}

func TestManifestNeedsOurSignature(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	other, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	data := []byte(`{"version":"0.4.0","files":{}}`)
	if m, err := verifyManifest(data, signed(t, key, data), &key.PublicKey); err != nil || m.Version != "0.4.0" {
		t.Fatalf("gültige Signatur abgelehnt: %v", err)
	}
	if _, err := verifyManifest(data, signed(t, other, data), &key.PublicKey); err == nil {
		t.Fatal("fremder Schlüssel akzeptiert")
	}
	changed := []byte(`{"version":"9.9.9","files":{}}`)
	if _, err := verifyManifest(changed, signed(t, key, data), &key.PublicKey); err == nil {
		t.Fatal("veränderte Beschreibung akzeptiert")
	}
	if _, err := verifyManifest(data, []byte("kaputt"), &key.PublicKey); err == nil {
		t.Fatal("kaputte Signatur akzeptiert")
	}
}

func TestExtractBinaryFromEveryFormat(t *testing.T) {
	for _, name := range []string{"Replado-Connector.exe", "replado-connector-linux-amd64.tar.gz", "Replado-Connector-macOS.zip"} {
		got, err := extractBinary(name, packed(t, name, []byte("programm")))
		if err != nil || string(got) != "programm" {
			t.Errorf("%s: %q %v", name, got, err)
		}
	}
	if _, err := extractBinary("x.tar.gz", packed(t, "x.zip", []byte("p"))); err == nil {
		t.Fatal("falsches Archiv akzeptiert")
	}
	if _, err := extractBinary("x.dmg", []byte("p")); err == nil {
		t.Fatal("unbekanntes Format akzeptiert")
	}
}

func TestUpdateReplacesTheInstalledProgram(t *testing.T) {
	f := newUpdateFixture(t, "0.9.0", []byte("programm 0.9.0"))
	defer f.close()
	got, err := applyUpdate(http.DefaultClient, "0.4.0", f.ask(true))
	if err != nil || got != "0.9.0" {
		t.Fatalf("%q %v", got, err)
	}
	if f.content(t) != "programm 0.9.0" {
		t.Fatal("Programm wurde nicht ersetzt")
	}
	if old, _ := ioutil.ReadFile(f.target + ".alt"); string(old) != "alt" {
		t.Fatal("alte Version muss als .alt erhalten bleiben")
	}
	if _, err := os.Stat(f.target + ".neu"); !os.IsNotExist(err) {
		t.Fatal(".neu darf nicht liegen bleiben")
	}
	if len(f.asked) != 1 || f.asked[0] != "0.9.0" {
		t.Fatalf("Replado muss vorher gefragt werden: %v", f.asked)
	}
}

func TestNoUpdateWhenAlreadyCurrent(t *testing.T) {
	f := newUpdateFixture(t, "0.4.0", []byte("programm 0.4.0"))
	defer f.close()
	got, err := applyUpdate(http.DefaultClient, "0.4.0", f.ask(true))
	if err != nil || got != "" || len(f.asked) != 0 || f.content(t) != "alt" {
		t.Fatalf("%q %v %v", got, err, f.asked)
	}
}

func TestNoDownloadWhenRepladoSaysNo(t *testing.T) {
	f := newUpdateFixture(t, "0.9.0", []byte("programm 0.9.0"))
	defer f.close()
	got, err := applyUpdate(http.DefaultClient, "0.4.0", f.ask(false))
	if err != nil || got != "" || f.content(t) != "alt" {
		t.Fatalf("%q %v", got, err)
	}
	for _, r := range f.server.requests {
		if r == assetName() {
			t.Fatal("Programm darf ohne Freigabe nicht geladen werden")
		}
	}
}

func TestForeignSignatureStopsTheUpdate(t *testing.T) {
	f := newUpdateFixture(t, "0.9.0", []byte("programm 0.9.0"))
	defer f.close()
	other, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	f.server.files["update.json.sig"] = signed(t, other, f.server.files["update.json"])
	if _, err := applyUpdate(http.DefaultClient, "0.4.0", f.ask(true)); err == nil {
		t.Fatal("fremde Signatur akzeptiert")
	}
	if len(f.asked) != 0 || f.content(t) != "alt" {
		t.Fatal("bei fremder Signatur darf nichts passieren")
	}
}

func TestSwappedFileIsRejectedByTheChecksum(t *testing.T) {
	f := newUpdateFixture(t, "0.9.0", []byte("programm 0.9.0"))
	defer f.close()
	f.server.files[assetName()] = packed(t, assetName(), []byte("programm 0.9.0 manipuliert"))
	if _, err := applyUpdate(http.DefaultClient, "0.4.0", f.ask(true)); err == nil || !strings.Contains(err.Error(), "Prüfsumme") {
		t.Fatalf("ausgetauschte Datei akzeptiert: %v", err)
	}
	if f.content(t) != "alt" {
		t.Fatal("Programm darf nicht ersetzt werden")
	}
}

func TestNewProgramThatDoesNotStartKeepsTheOldOne(t *testing.T) {
	f := newUpdateFixture(t, "0.9.0", []byte("programm 0.8.0"))
	defer f.close()
	if _, err := applyUpdate(http.DefaultClient, "0.4.0", f.ask(true)); err == nil {
		t.Fatal("falsche Version akzeptiert")
	}
	if f.content(t) != "alt" {
		t.Fatal("altes Programm muss bleiben")
	}
	if _, err := os.Stat(f.target + ".neu"); !os.IsNotExist(err) {
		t.Fatal(".neu muss entfernt werden")
	}
}

func TestVersionCommandPrintsTheVersion(t *testing.T) {
	var out bytes.Buffer
	if err := runCommand([]string{"version"}, &out, nil); err != nil || strings.TrimSpace(out.String()) != version {
		t.Fatalf("%q %v", out.String(), err)
	}
}

func TestSettingsShowTheVersionOfTheUpdatedService(t *testing.T) {
	defer useAppDir(t)()
	u := &uiServer{}
	if s := u.state(); s["version"] != version || s["outdated"] != false {
		t.Fatalf("ohne Dienst: %v %v", s["version"], s["outdated"])
	}
	ioutil.WriteFile(statusPath(), []byte(`{"version":"99.0.0"}`), 0600)
	if s := u.state(); s["version"] != "99.0.0" || s["outdated"] != true {
		t.Fatalf("neuerer Dienst: %v %v", s["version"], s["outdated"])
	}
	ioutil.WriteFile(statusPath(), []byte(`{"version":"0.0.1"}`), 0600)
	if s := u.state(); s["version"] != version || s["outdated"] != false {
		t.Fatalf("älterer Dienst: %v %v", s["version"], s["outdated"])
	}
}

func TestSettingsSayTheDownloadIsNoLongerNeeded(t *testing.T) {
	defer useAppDir(t)()
	u := &uiServer{}
	if u.state()["downloaded"] != true {
		t.Fatal("Programm außerhalb des Installationsordners muss als Download gelten")
	}
	old := installedTarget
	defer func() { installedTarget = old }()
	self, _ := os.Executable()
	installedTarget = func() string { return self }
	if u.state()["downloaded"] != false {
		t.Fatal("installierte Kopie darf nicht als Download gelten")
	}
}
