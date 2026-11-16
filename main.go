package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const version = "0.4.0"

const usage = `Replado Connector ` + version + `

Ohne Angabe öffnet sich die Einrichtung im Browser.

  replado-connector ui [--ohne-browser] [--ohne-dienst]
                                          Einrichtung im Browser (nur auf diesem PC erreichbar)
  replado-connector login --server URL --email E-MAIL [--code CODE] [--ohne-dienst]
                                          Passwort aus REPLADO_PASSWORD oder Eingabe,
                                          --ohne-dienst richtet keinen Autostart ein
  replado-connector logout
  replado-connector status
  replado-connector folders               Freigegebene Ordner anzeigen
  replado-connector folder-add PFAD [--lizenz-bestaetigt]
  replado-connector folder-remove PFAD
  replado-connector rescan                Freigegebene Ordner neu einlesen
  replado-connector log                   Protokoll anzeigen
  replado-connector install | uninstall
  replado-connector serve                 Hintergrunddienst

Zum Testen ohne Replado, nur lokal:
  replado-connector scan PFAD ...
  replado-connector call WERKZEUG JSON --path PFAD
  replado-connector mcp PFAD ...
`

func main() {
	initConsole()
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"ui"}
	}
	if err := runCommand(args, os.Stdout, os.Stdin); err != nil {
		fmt.Fprintln(os.Stderr, "Fehler:", err)
		os.Exit(1)
	}
}

func flagValue(args []string, name string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == name {
			return args[i+1]
		}
	}
	return ""
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

func positional(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") {
			if args[i] == "--server" || args[i] == "--email" || args[i] == "--code" || args[i] == "--path" {
				i++
			}
			continue
		}
		out = append(out, args[i])
	}
	return out
}

func runCommand(args []string, out io.Writer, in io.Reader) error {
	rest := args[1:]
	switch args[0] {
	case "ui":
		install := ensureInstalled
		if hasFlag(rest, "--ohne-dienst") {
			install = nil
		}
		return runUI(!hasFlag(rest, "--ohne-browser"), install)
	case "serve":
		running := singleInstance()
		if !running && hasFlag(rest, "--nach-update") {
			running = waitForInstance()
		}
		if !running {
			return nil
		}
		setupLog()
		startTray()
		go updateLoop()
		runServe()
		return nil
	case "version":
		fmt.Fprintln(out, version)
		return nil
	case "login":
		password := os.Getenv("REPLADO_PASSWORD")
		if password == "" {
			fmt.Fprint(out, "Passwort: ")
			line, _ := bufio.NewReader(in).ReadString('\n')
			password = strings.TrimRight(line, "\r\n")
		}
		_, err := login(httpClient(60*time.Second), flagValue(rest, "--server"), flagValue(rest, "--email"), password, flagValue(rest, "--code"))
		if err == errSecondFactor {
			return errf("Zwei-Faktor-Anmeldung aktiv: bitte mit --code CODE wiederholen.")
		}
		if err != nil {
			return err
		}
		fmt.Fprintln(out, "Angemeldet. Jetzt mit folder-add den Ordner der Software freigeben.")
		if hasFlag(rest, "--ohne-dienst") {
			return nil
		}
		return ensureInstalled()
	case "logout":
		return logout(httpClient(30 * time.Second))
	case "status":
		c := loadConfig()
		st := readStatus()
		if c.Secret == "" {
			fmt.Fprintln(out, "Nicht angemeldet.")
		} else {
			fmt.Fprintf(out, "Verbunden mit %s als %s\n", c.Server, c.Account)
		}
		fmt.Fprintf(out, "Letzter Kontakt: %s\nLetzter Fehler: %s\nEingelesen: %s, %d Datenbanken\n", st.LastContact, st.LastError, st.ScannedAt, st.Sources)
		return nil
	case "folders":
		for _, f := range loadConfig().Folders {
			fmt.Fprintf(out, "%s  (Lizenz bestätigt %s)\n", f.Path, f.ConfirmedAt)
		}
		return nil
	case "folder-add":
		paths := positional(rest)
		if len(paths) != 1 {
			return errf("Bitte genau einen Ordner angeben.")
		}
		clean, err := validateFolder(paths[0])
		if err != nil {
			return err
		}
		for _, s := range scanOnly([]string{clean}) {
			fmt.Fprintf(out, "Gefunden: %s  %s  (%d Tabellen)\n", nonEmpty(s.Software, "unbekannte Software"), s.Path, len(s.Tables))
		}
		fmt.Fprintln(out, "\n"+confirmationText)
		if !hasFlag(rest, "--lizenz-bestaetigt") {
			return errf("Zum Freigeben den Befehl mit --lizenz-bestaetigt wiederholen.")
		}
		if _, err := updateConfig(func(c *Config) error { return addFolder(c, clean, true) }); err != nil {
			return err
		}
		record(Entry{Kind: "ordner", Text: "Ordner freigegeben, Lizenz bestätigt: " + clean})
		fmt.Fprintln(out, "Freigegeben:", clean)
		return nil
	case "folder-remove":
		paths := positional(rest)
		if len(paths) != 1 {
			return errf("Bitte genau einen Ordner angeben.")
		}
		_, err := updateConfig(func(c *Config) error {
			if !removeFolder(c, paths[0]) {
				return errf("Diesen Ordner gibt es nicht in der Liste.")
			}
			return nil
		})
		if err == nil {
			record(Entry{Kind: "ordner", Text: "Freigabe entfernt: " + paths[0]})
		}
		return err
	case "rescan":
		_, err := updateConfig(func(c *Config) error {
			c.RescanAt = time.Now().Unix()
			return nil
		})
		return err
	case "log":
		for _, e := range readProtocol(100) {
			fmt.Fprintf(out, "%s  %-9s %s %s %s %d %s\n", e.Time, e.Kind, e.Tool, e.Table, e.Text, e.Rows, e.Error)
		}
		return nil
	case "install":
		return ensureInstalled()
	case "uninstall":
		logout(httpClient(30 * time.Second))
		return uninstall()
	case "scan":
		paths := positional(rest)
		if len(paths) == 0 {
			return errf("Bitte mindestens einen Ordner angeben.")
		}
		t := time.Now()
		found := scanOnly(paths)
		for _, s := range found {
			fmt.Fprintf(out, "%s  %-10s  %s  (geändert %s)\n", s.ID, nonEmpty(s.Software, "unbekannt"), s.Path, s.Changed)
			for _, ti := range s.Tables {
				fmt.Fprintf(out, "            %-8s %7d  %s\n", ti.Format, ti.Rows, ti.Name)
			}
		}
		fmt.Fprintf(out, "%d Quellen, %s\n", len(found), time.Since(t).Round(time.Millisecond))
		return nil
	case "call":
		pos := positional(rest)
		path := flagValue(rest, "--path")
		if len(pos) < 1 || path == "" {
			return errf("Aufruf: call WERKZEUG JSON --path PFAD")
		}
		raw := "{}"
		if len(pos) > 1 {
			raw = pos[1]
		}
		setRoots([]string{path})
		Scan([]string{path})
		res, err := callTool(pos[0], json.RawMessage(raw))
		if err != nil {
			return err
		}
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Fprintln(out, string(b))
		return nil
	case "mcp":
		paths := positional(rest)
		if len(paths) == 0 {
			return errf("Bitte mindestens einen Ordner angeben.")
		}
		setRoots(paths)
		addKnown(loadConfig().Known)
		loadKnownFile(appDir())
		runMCP()
		return nil
	}
	fmt.Fprint(out, usage)
	return errf("Unbekannter Befehl %q", args[0])
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func ensureInstalled() error {
	dir := appDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	target := filepath.Join(dir, exeName())
	if !samePath(self, target) {
		stopRunning()
		if err := copyFile(self, target); err != nil {
			return errf("Kopieren nach %s: %v", dir, err)
		}
	}
	if err := setAutostart(target); err != nil {
		return errf("Autostart: %v", err)
	}
	installShortcut(target)
	if startsItself() {
		return nil
	}
	c := exec.Command(target, "serve")
	c.Dir = dir
	hideWindow(c)
	return c.Start()
}

func uninstall() error {
	stopRunning()
	removeAutostart()
	removeShortcut()
	os.Remove(configPath())
	scheduleRemoveDir(appDir())
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".neu"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	os.Remove(dst)
	return os.Rename(tmp, dst)
}

func setupLog() {
	os.MkdirAll(appDir(), 0700)
	p := filepath.Join(appDir(), "connector.log")
	if st, err := os.Stat(p); err == nil && st.Size() > 2<<20 {
		os.Rename(p, p+".alt")
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		log.SetOutput(ioutil.Discard)
		return
	}
	log.SetOutput(f)
}
