package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	pollTimeout   = 75 * time.Second
	resultTimeout = 60 * time.Second
	fullRescan    = 6 * time.Hour
	maxResponse   = 4 << 20
)

type Status struct {
	LastContact string `json:"last_contact,omitempty"`
	LastError   string `json:"last_error,omitempty"`
	ScannedAt   string `json:"scanned_at,omitempty"`
	Sources     int    `json:"sources"`
	Version     string `json:"version"`
}

var statusMu sync.Mutex

func statusPath() string { return filepath.Join(appDir(), "status.json") }

func writeStatus(change func(s *Status)) {
	statusMu.Lock()
	defer statusMu.Unlock()
	s := readStatusLocked()
	change(&s)
	s.Version = version
	b, _ := json.Marshal(s)
	os.MkdirAll(appDir(), 0700)
	ioutil.WriteFile(statusPath(), b, 0600)
}

func readStatus() Status {
	statusMu.Lock()
	defer statusMu.Unlock()
	return readStatusLocked()
}

func readStatusLocked() Status {
	var s Status
	if b, err := ioutil.ReadFile(statusPath()); err == nil {
		json.Unmarshal(b, &s)
	}
	return s
}

var (
	validHost = regexp.MustCompile(`^([A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?\.)*[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$|^[0-9a-fA-F:.]+$`)
	validPort = regexp.MustCompile(`^[0-9]{1,5}$`)
)

func isPrivateHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	for _, cidr := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"} {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

func normalizeServer(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errf("Bitte die Adresse Ihres Replado angeben.")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", errf("Die Adresse %q ist ungültig.", raw)
	}
	if !validHost.MatchString(u.Hostname()) || (u.Port() != "" && !validPort.MatchString(u.Port())) {
		return "", errf("Die Adresse %q ist ungültig.", raw)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errf("Bitte nur die Adresse angeben, ohne Zugangsdaten oder Zusätze.")
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !isPrivateHost(u.Hostname()) {
			return "", errf("Ohne Verschlüsselung (http) nur im eigenen Netz erlaubt. Bitte https verwenden.")
		}
	default:
		return "", errf("Die Adresse muss mit https:// beginnen.")
	}
	path := strings.TrimRight(u.Path, "/")
	path = strings.TrimSuffix(path, "/api")
	return u.Scheme + "://" + u.Host + path, nil
}

func certPool() *x509.CertPool {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	pool.AppendCertsFromPEM([]byte(rootPEM))
	return pool
}

func httpClient(timeout time.Duration) *http.Client {
	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSClientConfig:     &tls.Config{RootCAs: certPool(), MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout: 20 * time.Second,
		MaxIdleConns:        2,
		IdleConnTimeout:     90 * time.Second,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

type apiError struct {
	status int
	code   string
	msg    string
}

func (e *apiError) Error() string { return e.msg }

func serverMessage(body []byte) string {
	var r struct {
		Detail interface{} `json:"detail"`
	}
	if json.Unmarshal(body, &r) == nil {
		if s, ok := r.Detail.(string); ok && s != "" {
			return s
		}
	}
	text := strings.TrimSpace(string(body))
	if len(text) > 200 {
		text = text[:200]
	}
	return text
}

func post(cl *http.Client, endpoint, secret string, body, out interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "de")
	req.Header.Set("User-Agent", "replado-connector/"+version+" ("+runtime.GOOS+")")
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	resp, err := cl.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	rb, err := ioutil.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return err
	}
	if len(rb) > maxResponse {
		return errf("Die Antwort des Servers ist zu groß")
	}
	if resp.StatusCode == 204 {
		return nil
	}
	if resp.StatusCode/100 != 2 {
		msg := serverMessage(rb)
		if msg == "" {
			msg = resp.Status
		}
		return &apiError{status: resp.StatusCode, code: resp.Header.Get("X-Replado-Code"), msg: msg}
	}
	if out != nil && len(rb) > 0 {
		return json.Unmarshal(rb, out)
	}
	return nil
}

var errSecondFactor = errf("Bitte den Code aus Ihrer Authenticator-App eingeben.")

func login(cl *http.Client, server, email, password, code string) (*Config, error) {
	server, err := normalizeServer(server)
	if err != nil {
		return nil, err
	}
	host, _ := os.Hostname()
	var r struct {
		ConnectorID string  `json:"connector_id"`
		Secret      string  `json:"secret"`
		Known       []Known `json:"known"`
	}
	body := map[string]interface{}{
		"email": strings.TrimSpace(email), "password": password, "machine": host,
		"os": osVersion(), "arch": runtime.GOARCH, "version": version, "tools": toolDefs,
	}
	if strings.TrimSpace(code) != "" {
		body["code"] = strings.TrimSpace(code)
	}
	err = post(cl, server+"/api/connector/v1/login", "", body, &r)
	if ae, ok := err.(*apiError); ok && ae.status == 401 && ae.code == "zwei_faktor" {
		return nil, errSecondFactor
	}
	if err != nil {
		return nil, err
	}
	if r.Secret == "" {
		return nil, errf("Der Server hat keinen Zugang geliefert. Ist die Adresse richtig?")
	}
	old := loadConfig()
	if old.Secret != "" {
		post(cl, old.Server+"/api/connector/v1/logout", old.Secret, map[string]string{}, nil)
	}
	c, err := updateConfig(func(c *Config) error {
		c.Server, c.Account, c.ConnectorID, c.Secret, c.Known = server, strings.TrimSpace(email), r.ConnectorID, r.Secret, r.Known
		return nil
	})
	record(Entry{Kind: "anmeldung", Text: "Angemeldet bei " + server + " als " + strings.TrimSpace(email)})
	return c, err
}

func logout(cl *http.Client) error {
	c := loadConfig()
	if c.Secret != "" {
		post(cl, c.Server+"/api/connector/v1/logout", c.Secret, map[string]string{}, nil)
	}
	_, err := updateConfig(func(c *Config) error {
		c.Secret, c.ConnectorID, c.Account = "", "", ""
		return nil
	})
	record(Entry{Kind: "abmeldung", Text: "Abgemeldet"})
	return err
}

type call struct {
	ID   string          `json:"id"`
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args"`
}

type pollAnswer struct {
	Calls  []call  `json:"calls"`
	Known  []Known `json:"known"`
	Rescan bool    `json:"rescan"`
}

type scanner struct {
	key      string
	rescanAt int64
	at       time.Time
	reported string
}

func folderKey(paths []string) string { return strings.Join(paths, "\n") }

func (s *scanner) ensure(c *Config, force bool) {
	paths := c.confirmedPaths()
	key := folderKey(paths)
	if !force && key == s.key && c.RescanAt == s.rescanAt && time.Since(s.at) < fullRescan {
		return
	}
	setRoots(paths)
	s.key, s.rescanAt, s.at = key, c.RescanAt, time.Now()
	if len(paths) == 0 {
		Scan(nil)
		writeStatus(func(st *Status) {
			st.ScannedAt = ""
			st.Sources = 0
		})
		return
	}
	t := time.Now()
	found := Scan(paths)
	writeStatus(func(st *Status) {
		st.ScannedAt = time.Now().Format(time.RFC3339)
		st.Sources = len(found)
	})
	record(Entry{Kind: "einlesen", Text: plural(len(found), "Datenbank", "Datenbanken") + " in " + plural(len(paths), "Ordner", "Ordnern"), Ms: int64(time.Since(t) / time.Millisecond)})
}

func pollBody(c *Config, s *scanner) map[string]interface{} {
	body := map[string]interface{}{"version": version, "os": osVersion()}
	folders := []Folder{}
	for _, f := range c.Folders {
		if f.Confirmed {
			folders = append(folders, f)
		}
	}
	report := struct {
		Sources []*Source `json:"sources"`
		Folders []Folder  `json:"folders"`
	}{getSources(), folders}
	if report.Sources == nil {
		report.Sources = []*Source{}
	}
	b, _ := json.Marshal(report)
	if string(b) != s.reported {
		body["sources"] = report.Sources
		body["folders"] = report.Folders
	}
	return body
}

func pollOnce(long, short *http.Client, c *Config, s *scanner) error {
	body := pollBody(c, s)
	var r pollAnswer
	err := post(long, c.Server+"/api/connector/v1/poll", c.Secret, body, &r)
	if err != nil {
		s.reported = ""
		return err
	}
	if _, sent := body["sources"]; sent {
		b, _ := json.Marshal(struct {
			Sources interface{} `json:"sources"`
			Folders interface{} `json:"folders"`
		}{body["sources"], body["folders"]})
		s.reported = string(b)
	}
	writeStatus(func(st *Status) {
		st.LastContact = time.Now().Format(time.RFC3339)
		st.LastError = ""
	})
	if len(r.Known) > 0 {
		addKnown(r.Known)
		updateConfig(func(cc *Config) error {
			cc.Known = r.Known
			return nil
		})
	}
	if r.Rescan {
		s.ensure(c, true)
		s.reported = ""
	}
	var wg sync.WaitGroup
	for _, cl := range r.Calls {
		wg.Add(1)
		go func(cl call) {
			defer wg.Done()
			answer(short, c, cl)
		}(cl)
	}
	wg.Wait()
	return nil
}

func answer(cl *http.Client, c *Config, k call) {
	t := time.Now()
	var a struct {
		Source string `json:"source"`
		Table  string `json:"table"`
	}
	json.Unmarshal(k.Args, &a)
	out := map[string]interface{}{"id": k.ID}
	var res interface{}
	var err error
	if k.Tool == "list_sources" || !toolAllowed(k.Tool) {
		err = errf("Dieses Werkzeug ist über Replado nicht freigegeben")
	} else {
		res, err = safeCall(k.Tool, k.Args)
	}
	e := Entry{Kind: "abfrage", Tool: k.Tool, Source: a.Source, Table: a.Table, Ms: int64(time.Since(t) / time.Millisecond)}
	if err != nil {
		out["error"] = err.Error()
		e.Error = err.Error()
	} else {
		out["result"] = res
		e.Rows = rowsOf(res)
	}
	record(e)
	for i := 0; i < 3; i++ {
		perr := post(cl, c.Server+"/api/connector/v1/result", c.Secret, out, nil)
		if perr == nil {
			return
		}
		log.Printf("Ergebnis %s: %v", k.ID, perr)
		time.Sleep(2 * time.Second)
	}
}

func toolAllowed(name string) bool {
	switch name {
	case "describe_table", "query", "search":
		return true
	}
	return false
}

func runServe() {
	c := loadConfig()
	addKnown(c.Known)
	loadKnownFile(appDir())
	log.Printf("Start %s auf %s", version, osVersion())
	long := httpClient(pollTimeout)
	short := httpClient(resultTimeout)
	s := &scanner{}
	backoff := 5 * time.Second
	for {
		c = loadConfig()
		s.ensure(c, false)
		if c.Secret == "" {
			time.Sleep(5 * time.Second)
			continue
		}
		err := pollOnce(long, short, c, s)
		if err == nil {
			backoff = 5 * time.Second
			continue
		}
		if ae, ok := err.(*apiError); ok && ae.status == 401 {
			log.Printf("Zugang ungültig, Connector wurde in Replado entfernt")
			record(Entry{Kind: "abmeldung", Text: "Replado hat den Zugang beendet. Bitte neu anmelden."})
			updateConfig(func(cc *Config) error {
				if cc.Secret == c.Secret {
					cc.Secret, cc.ConnectorID = "", ""
				}
				return nil
			})
			writeStatus(func(st *Status) { st.LastError = "Zugang beendet, bitte neu anmelden" })
			continue
		}
		log.Printf("Verbindung: %v", err)
		writeStatus(func(st *Status) { st.LastError = err.Error() })
		time.Sleep(backoff)
		if backoff < 5*time.Minute {
			backoff *= 2
		}
	}
}

func safeCall(name string, args json.RawMessage) (res interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errf("interner Fehler beim Lesen: %v", r)
		}
	}()
	return callTool(name, args)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return itoa(n) + " " + many
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
