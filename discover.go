package main

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type TableInfo struct {
	Name   string `json:"name"`
	Format string `json:"format"`
	Rows   int    `json:"rows"`
	Locked bool   `json:"password_protected,omitempty"`
	file   string
}

type Source struct {
	ID       string      `json:"id"`
	Path     string      `json:"path"`
	Software string      `json:"software,omitempty"`
	Hint     string      `json:"hint,omitempty"`
	Changed  string      `json:"last_changed"`
	Tables   []TableInfo `json:"tables"`
}

var skipDirs = map[string]bool{
	"windows": true, "$recycle.bin": true, "system volume information": true, "recycler": true,
	"temp": true, "tmp": true, "node_modules": true, ".git": true, "winsxs": true,
	"microsoft.net": true, "windowsapps": true, "msocache": true, "programdata\\microsoft": true,
	"inetcache": true, "temporary internet files": true, "cache": true,
}

const maxDepth = 9

var (
	sources   []*Source
	sourcesMu sync.Mutex
	lastScan  time.Time
)

func Scan(paths []string) []*Source {
	out := scanOnly(paths)
	sourcesMu.Lock()
	sources = out
	lastScan = time.Now()
	sourcesMu.Unlock()
	return out
}

func scanOnly(roots []string) []*Source {
	byDir := map[string][]TableInfo{}
	newest := map[string]time.Time{}
	for _, root := range roots {
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
		base := strings.Count(root, string(os.PathSeparator))
		filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
			if err != nil {
				if fi != nil && fi.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if fi.IsDir() {
				n := strings.ToLower(fi.Name())
				if p != root && (skipDirs[n] || strings.HasPrefix(n, ".")) {
					return filepath.SkipDir
				}
				if strings.Count(p, string(os.PathSeparator))-base > maxDepth {
					return filepath.SkipDir
				}
				return nil
			}
			if fi.Mode()&os.ModeSymlink != 0 || fi.Size() < 32 {
				return nil
			}
			var ti *TableInfo
			switch strings.ToLower(filepath.Ext(p)) {
			case ".db":
				if h, ok := sniffParadox(p); ok {
					ti = &TableInfo{Format: "paradox", Rows: h.numRecords, Locked: h.encrypted}
				}
			case ".dbf":
				if h, ok := sniffDBF(p); ok {
					ti = &TableInfo{Format: "dbf", Rows: h.numRecords}
				}
			}
			if ti != nil {
				ti.Name = strings.TrimSuffix(fi.Name(), filepath.Ext(fi.Name()))
				ti.file = p
				d := filepath.Dir(p)
				byDir[d] = append(byDir[d], *ti)
				if fi.ModTime().After(newest[d]) {
					newest[d] = fi.ModTime()
				}
			}
			return nil
		})
	}
	var out []*Source
	for d, ts := range byDir {
		total := 0
		for _, t := range ts {
			total += t.Rows
		}
		if total == 0 {
			continue
		}
		sort.Slice(ts, func(i, j int) bool { return ts[i].Rows > ts[j].Rows })
		h := sha1.Sum([]byte(strings.ToLower(d)))
		s := &Source{ID: hex.EncodeToString(h[:])[:8], Path: d, Tables: ts, Changed: newest[d].Format("2006-01-02")}
		if k := identify(ts); k != nil {
			s.Software, s.Hint = k.Name, k.Hint
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Software != "") != (out[j].Software != "") {
			return out[i].Software != ""
		}
		return out[i].Changed > out[j].Changed
	})
	return out
}

var (
	rootsMu sync.Mutex
	roots   []string
)

func setRoots(paths []string) {
	rootsMu.Lock()
	roots = append([]string(nil), paths...)
	rootsMu.Unlock()
}

func scanRoots() []string {
	rootsMu.Lock()
	defer rootsMu.Unlock()
	return append([]string(nil), roots...)
}

func allowedPath(p string) bool {
	for _, r := range scanRoots() {
		if inside(p, r) {
			return true
		}
	}
	return false
}

func getSources() []*Source {
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	return sources
}

func findSource(id string) *Source {
	if getSources() == nil {
		Scan(scanRoots())
	}
	for _, s := range getSources() {
		if (s.ID == id || strings.EqualFold(s.Path, id)) && allowedPath(s.Path) {
			return s
		}
	}
	return nil
}

func (s *Source) tableFile(name string) string {
	for _, t := range s.Tables {
		if strings.EqualFold(t.Name, name) {
			return t.file
		}
	}
	return ""
}
