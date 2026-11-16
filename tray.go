package main

import (
	"encoding/base64"
	"encoding/binary"
	"net/url"
	"time"
)

const (
	trayFresh   = 3 * time.Minute
	trayTipMax  = 127
	trayAppName = "Replado Connector"
)

type trayState struct {
	online bool
	text   string
}

func currentTrayState(c *Config, st Status, now time.Time) trayState {
	if c == nil || c.Secret == "" {
		return trayState{text: "Nicht angemeldet"}
	}
	host := c.Server
	if u, err := url.Parse(c.Server); err == nil && u.Host != "" {
		host = u.Host
	}
	if st.LastError != "" {
		return trayState{text: "Keine Verbindung: " + st.LastError}
	}
	if t, err := time.Parse(time.RFC3339, st.LastContact); err == nil && now.Sub(t) < trayFresh {
		return trayState{online: true, text: "Verbunden mit " + host}
	}
	return trayState{text: "Verbindet mit " + host + " …"}
}

func trayTip(s trayState) string {
	r := []rune(trayAppName + "\n" + s.text)
	if len(r) > trayTipMax {
		r = append(r[:trayTipMax-1], '…')
	}
	return string(r)
}

func iconFile(online bool) []byte {
	src := iconOffline
	if online {
		src = iconOnline
	}
	data, _ := base64.StdEncoding.DecodeString(src)
	return data
}

func iconImage(ico []byte, size int) []byte {
	if len(ico) < 6 {
		return nil
	}
	count := int(binary.LittleEndian.Uint16(ico[4:6]))
	var best []byte
	bestSize := 0
	for i := 0; i < count; i++ {
		e := 6 + 16*i
		if e+16 > len(ico) {
			return best
		}
		w := int(ico[e])
		if w == 0 {
			w = 256
		}
		n := int(binary.LittleEndian.Uint32(ico[e+8:]))
		off := int(binary.LittleEndian.Uint32(ico[e+12:]))
		if off < 0 || n < 0 || off+n > len(ico) {
			continue
		}
		better := best == nil || (w >= size && (bestSize < size || w < bestSize)) || (bestSize < size && w > bestSize)
		if better {
			best, bestSize = ico[off:off+n], w
		}
	}
	return best
}
