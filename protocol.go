package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxProtocolBytes = 1 << 20

type Entry struct {
	Time   string `json:"time"`
	Kind   string `json:"kind"`
	Tool   string `json:"tool,omitempty"`
	Source string `json:"source,omitempty"`
	Table  string `json:"table,omitempty"`
	Rows   int    `json:"rows,omitempty"`
	Ms     int64  `json:"ms,omitempty"`
	Text   string `json:"text,omitempty"`
	Error  string `json:"error,omitempty"`
}

var protocolMu sync.Mutex

func protocolPath() string { return filepath.Join(appDir(), "protokoll.jsonl") }

func record(e Entry) {
	protocolMu.Lock()
	defer protocolMu.Unlock()
	if e.Time == "" {
		e.Time = time.Now().Format(time.RFC3339)
	}
	p := protocolPath()
	if st, err := os.Stat(p); err == nil && st.Size() > maxProtocolBytes {
		os.Remove(p + ".alt")
		os.Rename(p, p+".alt")
	}
	os.MkdirAll(appDir(), 0700)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	b, _ := json.Marshal(e)
	f.Write(append(b, '\n'))
}

func readProtocol(limit int) []Entry {
	protocolMu.Lock()
	defer protocolMu.Unlock()
	var all []Entry
	for _, p := range []string{protocolPath() + ".alt", protocolPath()} {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1<<20)
		for sc.Scan() {
			var e Entry
			if json.Unmarshal(sc.Bytes(), &e) == nil {
				all = append(all, e)
			}
		}
		f.Close()
	}
	out := []Entry{}
	for i := len(all) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, all[i])
	}
	return out
}

func rowsOf(res interface{}) int {
	m, ok := res.(map[string]interface{})
	if !ok {
		return 0
	}
	for _, key := range []string{"rows", "hits", "sample"} {
		switch v := m[key].(type) {
		case []map[string]interface{}:
			return len(v)
		case []interface{}:
			return len(v)
		}
	}
	return 0
}
