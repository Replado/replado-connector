package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxFolders = 20

const confirmationText = "Ich bestätige, dass dieses Unternehmen berechtigt ist, die Daten in diesem Ordner zu nutzen, und dass ich die Lizenzbedingungen der Software, zu der diese Daten gehören, geprüft habe: Sie erlauben, dass ein anderes Programm die Daten liest. Der Replado Connector liest nur und ändert nichts."

func cleanPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return filepath.Clean(p)
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func inside(path, folder string) bool {
	p := strings.ToLower(filepath.Clean(path))
	f := strings.ToLower(filepath.Clean(folder))
	if p == f {
		return true
	}
	if !strings.HasSuffix(f, string(os.PathSeparator)) {
		f += string(os.PathSeparator)
	}
	return strings.HasPrefix(p, f)
}

func isRootLike(p string) bool {
	clean := filepath.Clean(p)
	if filepath.Dir(clean) == clean {
		return true
	}
	if home := homeDir(); home != "" && samePath(clean, home) {
		return true
	}
	for _, blocked := range systemDirs() {
		if inside(clean, blocked) {
			return true
		}
	}
	return false
}

func validateFolder(p string) (string, error) {
	clean := cleanPath(p)
	if clean == "" {
		return "", errf("Bitte einen Ordner angeben.")
	}
	st, err := os.Stat(clean)
	if err != nil || !st.IsDir() {
		return "", errf("Den Ordner %s gibt es nicht oder er ist nicht lesbar.", clean)
	}
	if isRootLike(clean) {
		return "", errf("Bitte den Ordner der Software wählen, nicht ein ganzes Laufwerk, den Benutzerordner oder einen Systemordner.")
	}
	return clean, nil
}

func addFolder(c *Config, p string, confirmed bool) error {
	clean, err := validateFolder(p)
	if err != nil {
		return err
	}
	if !confirmed {
		return errf("Bitte zuerst bestätigen, dass die Lizenz der Software das Auslesen erlaubt.")
	}
	for i, f := range c.Folders {
		if samePath(f.Path, clean) {
			c.Folders[i] = Folder{Path: clean, Confirmed: true, Confirmation: confirmationText, ConfirmedAt: time.Now().Format(time.RFC3339)}
			return nil
		}
	}
	if len(c.Folders) >= maxFolders {
		return errf("Es können höchstens %d Ordner freigegeben werden.", maxFolders)
	}
	c.Folders = append(c.Folders, Folder{Path: clean, Confirmed: true, Confirmation: confirmationText, ConfirmedAt: time.Now().Format(time.RFC3339)})
	return nil
}

func removeFolder(c *Config, p string) bool {
	for i, f := range c.Folders {
		if samePath(f.Path, p) {
			c.Folders = append(c.Folders[:i], c.Folders[i+1:]...)
			return true
		}
	}
	return false
}
