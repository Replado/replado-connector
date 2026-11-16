package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func withoutWaiting(t *testing.T) (*[]time.Duration, func()) {
	waits := []time.Duration{}
	previous := startWait
	startWait = func(d time.Duration) { waits = append(waits, d) }
	return &waits, func() { startWait = previous }
}

func TestPostWaitsWhileTheDemoIsStarting(t *testing.T) {
	waits, restore := withoutWaiting(t)
	defer restore()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.Header().Set("Retry-After", "5")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"detail":"Die Demo startet gerade.","starting":true}`))
			return
		}
		w.Write([]byte(`{"connector_id":"7"}`))
	}))
	defer srv.Close()
	var out struct {
		ID string `json:"connector_id"`
	}
	if err := post(srv.Client(), srv.URL, "", map[string]string{}, &out); err != nil {
		t.Fatal(err)
	}
	if out.ID != "7" || calls != 3 || len(*waits) != 2 || (*waits)[0] != 5*time.Second {
		t.Fatalf("id %q, calls %d, waits %v", out.ID, calls, *waits)
	}
}

func TestPostGivesUpAfterTheStartTakesTooLong(t *testing.T) {
	waits, restore := withoutWaiting(t)
	defer restore()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "300")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"detail":"Die Demo startet gerade."}`))
	}))
	defer srv.Close()
	err := post(srv.Client(), srv.URL, "", map[string]string{}, nil)
	if err == nil || err.Error() != "Die Demo startet gerade." {
		t.Fatalf("err %v", err)
	}
	if len(*waits) != maxStartRetries || (*waits)[0] != maxStartWait {
		t.Fatalf("waits %v", *waits)
	}
}

func TestPostNeverShowsHTMLFromAForeignServer(t *testing.T) {
	_, restore := withoutWaiting(t)
	defer restore()
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable, http.StatusNotFound} {
		code := status
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(code)
			w.Write([]byte(`<!doctype html><html lang="de"><head><meta http-equiv="refresh" content="5"></head></html>`))
		}))
		var out map[string]interface{}
		err := post(srv.Client(), srv.URL, "", map[string]string{}, &out)
		srv.Close()
		if err == nil || strings.Contains(err.Error(), "<") || !strings.Contains(err.Error(), "kein Replado") {
			t.Fatalf("status %d: %v", code, err)
		}
	}
}
