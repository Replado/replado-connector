package main

import (
	"encoding/binary"
	"strings"
	"testing"
	"time"
)

func TestTrayStateFollowsTheConnection(t *testing.T) {
	now := time.Date(2026, 11, 16, 20, 0, 0, 0, time.UTC)
	c := &Config{Server: "https://firma.replado.com", Secret: "s"}
	cases := []struct {
		name   string
		c      *Config
		st     Status
		online bool
		text   string
	}{
		{"ohne Anmeldung", &Config{}, Status{}, false, "Nicht angemeldet"},
		{"frischer Kontakt", c, Status{LastContact: now.Add(-time.Minute).Format(time.RFC3339)}, true, "Verbunden mit firma.replado.com"},
		{"alter Kontakt", c, Status{LastContact: now.Add(-10 * time.Minute).Format(time.RFC3339)}, false, "Verbindet mit firma.replado.com …"},
		{"Fehler", c, Status{LastContact: now.Format(time.RFC3339), LastError: "Zeitüberschreitung"}, false, "Keine Verbindung: Zeitüberschreitung"},
	}
	for _, tc := range cases {
		got := currentTrayState(tc.c, tc.st, now)
		if got.online != tc.online || got.text != tc.text {
			t.Errorf("%s: %+v", tc.name, got)
		}
	}
}

func TestTrayTipStaysWithinTheWindowsLimit(t *testing.T) {
	tip := trayTip(trayState{text: "Keine Verbindung: " + strings.Repeat("ä", 300)})
	if n := len([]rune(tip)); n > trayTipMax {
		t.Fatalf("%d Zeichen", n)
	}
	if !strings.HasPrefix(tip, trayAppName+"\n") || !strings.HasSuffix(tip, "…") {
		t.Fatal(tip)
	}
}

func TestIconImagePicksTheMatchingSize(t *testing.T) {
	for _, online := range []bool{true, false} {
		ico := iconFile(online)
		for _, size := range []int{16, 20, 24, 32, 48} {
			img := iconImage(ico, size)
			if len(img) < 40 || int(binary.LittleEndian.Uint32(img[4:8])) != size {
				t.Fatalf("online=%v Größe %d nicht gefunden", online, size)
			}
		}
		if img := iconImage(ico, 40); int(binary.LittleEndian.Uint32(img[4:8])) != 48 {
			t.Fatal("bei Zwischengröße soll das nächstgrößere Bild kommen")
		}
		if img := iconImage(ico, 64); int(binary.LittleEndian.Uint32(img[4:8])) != 48 {
			t.Fatal("bei zu großer Anfrage soll das größte Bild kommen")
		}
	}
	if iconImage([]byte{0, 0, 1, 0, 1, 0}, 16) != nil || iconImage(nil, 16) != nil {
		t.Fatal("kaputte Datei darf nichts liefern")
	}
}
