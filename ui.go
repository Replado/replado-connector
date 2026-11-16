package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	uiIdleTimeout = 10 * time.Minute
	maxUIBody     = 64 * 1024
	maxDirEntries = 500
)

type uiServer struct {
	token    string
	origin   string
	mu       sync.Mutex
	lastPing time.Time
	client   *http.Client
	install  func() error
}

func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (u *uiServer) touch() {
	u.mu.Lock()
	u.lastPing = time.Now()
	u.mu.Unlock()
}

func (u *uiServer) idle() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return time.Since(u.lastPing) > uiIdleTimeout
}

func securityHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	h.Set("X-Frame-Options", "DENY")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cache-Control", "no-store")
	h.Set("X-UA-Compatible", "IE=edge")
}

func (u *uiServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)
	if r.Host != strings.TrimPrefix(u.origin, "http://") {
		http.Error(w, "Falscher Host", http.StatusForbidden)
		return
	}
	if o := r.Header.Get("Origin"); o != "" && o != u.origin {
		http.Error(w, "Falscher Ursprung", http.StatusForbidden)
		return
	}
	switch r.URL.Path {
	case "/":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, uiHTML)
		return
	case "/app.js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		io.WriteString(w, uiJS)
		return
	case "/favicon.ico":
		w.WriteHeader(http.StatusNoContent)
		return
	case "/app.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		io.WriteString(w, uiCSS)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	given := r.Header.Get("X-Replado-Token")
	if len(given) != len(u.token) || subtle.ConstantTimeCompare([]byte(given), []byte(u.token)) != 1 {
		http.Error(w, "Nicht berechtigt", http.StatusUnauthorized)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Nur POST", http.StatusMethodNotAllowed)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "Nur JSON", http.StatusUnsupportedMediaType)
		return
	}
	u.touch()
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, maxUIBody+1))
	if err != nil || len(body) > maxUIBody {
		http.Error(w, "Anfrage zu groß", http.StatusRequestEntityTooLarge)
		return
	}
	res, err := u.handle(strings.TrimPrefix(r.URL.Path, "/api/"), body)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(res)
}

type uiRequest struct {
	Server    string `json:"server"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Code      string `json:"code"`
	Path      string `json:"path"`
	Confirmed bool   `json:"confirmed"`
}

func (u *uiServer) handle(action string, body []byte) (interface{}, error) {
	var req uiRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, errf("Ungültige Anfrage")
		}
	}
	switch action {
	case "ping":
		return map[string]bool{"ok": true}, nil
	case "state":
		return u.state(), nil
	case "login":
		if _, err := login(u.client, req.Server, req.Email, req.Password, req.Code); err != nil {
			if err == errSecondFactor {
				return map[string]interface{}{"second_factor": true, "message": err.Error()}, nil
			}
			return nil, err
		}
		if u.install != nil {
			if err := u.install(); err != nil {
				return nil, errf("Angemeldet, aber der Hintergrunddienst ließ sich nicht einrichten: %v", err)
			}
		}
		return u.state(), nil
	case "logout":
		if err := logout(u.client); err != nil {
			return nil, err
		}
		return u.state(), nil
	case "dirs":
		return listDirs(req.Path)
	case "preview":
		clean, err := validateFolder(req.Path)
		if err != nil {
			return nil, err
		}
		found := previewScan(clean)
		return map[string]interface{}{"path": clean, "sources": found, "confirmation": confirmationText}, nil
	case "add-folder":
		_, err := updateConfig(func(c *Config) error { return addFolder(c, req.Path, req.Confirmed) })
		if err != nil {
			return nil, err
		}
		record(Entry{Kind: "ordner", Text: "Ordner freigegeben, Lizenz bestätigt: " + cleanPath(req.Path)})
		return u.state(), nil
	case "remove-folder":
		_, err := updateConfig(func(c *Config) error {
			if !removeFolder(c, req.Path) {
				return errf("Diesen Ordner gibt es nicht in der Liste.")
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		record(Entry{Kind: "ordner", Text: "Freigabe entfernt: " + req.Path})
		return u.state(), nil
	case "rescan":
		if _, err := updateConfig(func(c *Config) error {
			c.RescanAt = time.Now().Unix()
			return nil
		}); err != nil {
			return nil, err
		}
		return u.state(), nil
	case "log":
		return readProtocol(200), nil
	}
	return nil, errf("Unbekannte Aktion")
}

func (u *uiServer) state() map[string]interface{} {
	c := loadConfig()
	folders := c.Folders
	if folders == nil {
		folders = []Folder{}
	}
	return map[string]interface{}{
		"version":      version,
		"os":           osVersion(),
		"server":       c.Server,
		"account":      c.Account,
		"connected":    c.Secret != "",
		"folders":      folders,
		"status":       readStatus(),
		"rescan_at":    c.RescanAt,
		"confirmation": confirmationText,
	}
}

func previewScan(path string) []*Source {
	found := scanOnly([]string{path})
	if found == nil {
		found = []*Source{}
	}
	return found
}

type dirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func listDirs(p string) (interface{}, error) {
	if strings.TrimSpace(p) == "" {
		var out []dirEntry
		for _, r := range browseRoots() {
			if st, err := os.Stat(r); err == nil && st.IsDir() {
				out = append(out, dirEntry{Name: r, Path: r})
			}
		}
		return map[string]interface{}{"path": "", "parent": "", "dirs": out}, nil
	}
	clean := cleanPath(p)
	infos, err := ioutil.ReadDir(clean)
	if err != nil {
		return nil, errf("Der Ordner lässt sich nicht öffnen.")
	}
	out := []dirEntry{}
	for _, fi := range infos {
		if !fi.IsDir() || strings.HasPrefix(fi.Name(), ".") || fi.Mode()&os.ModeSymlink != 0 {
			continue
		}
		out = append(out, dirEntry{Name: fi.Name(), Path: filepath.Join(clean, fi.Name())})
		if len(out) >= maxDirEntries {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	parent := filepath.Dir(clean)
	if parent == clean {
		parent = ""
	}
	return map[string]interface{}{"path": clean, "parent": parent, "dirs": out}, nil
}

func refreshService(c *Config, install func() error) error {
	if install == nil || c == nil || c.Secret == "" {
		return nil
	}
	return install()
}

func runUI(open bool, install func() error) error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	u := &uiServer{token: newToken(), origin: "http://" + ln.Addr().String(), client: httpClient(60 * time.Second), install: install}
	if err := refreshService(loadConfig(), install); err != nil {
		fmt.Println("Hintergrunddienst: " + err.Error())
	}
	u.touch()
	srv := &http.Server{Handler: u, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 90 * time.Second}
	address := u.origin + "/#" + u.token
	if runtime.GOOS == "windows" {
		address = u.origin + "/?t=" + u.token
	}
	go func() {
		for range time.Tick(30 * time.Second) {
			if u.idle() {
				srv.Close()
				return
			}
		}
	}()
	if open {
		if err := openBrowser(address); err != nil {
			go showMessage("Replado Connector", "Bitte diese Adresse im Browser öffnen:\n\n"+address)
		}
	}
	fmt.Println("Einstellungen: " + address)
	err = srv.Serve(ln)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
