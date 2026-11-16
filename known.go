package main

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"
)

type Known struct {
	Name     string   `json:"name"`
	Requires []string `json:"requires"`
	Hint     string   `json:"hint"`
}

var (
	knownMu sync.Mutex
	known   = []Known{
		{
			Name:     "Topfakt",
			Requires: []string{"Kunden", "Auftrag", "Atrpos", "Artikel", "Offpos"},
			Hint: "Faktura/Warenwirtschaft Topfakt (Paradox). " +
				"Kunden: Nr = Kundennummer; der Firmenname steht oft in 'Vorname', der Ansprechpartner in 'Name'; LetzterKontakt, Umsatz. " +
				"Auftrag: ein Beleg pro Zeile; Auftragnr, Kundennr (-> Kunden.Nr), Datum, RGDatum (Rechnungsdatum), LSDatum (Lieferschein), " +
				"Endbetrag (brutto), Endbetrag1Netto/Endbetrag2Netto (netto je Steuersatz), StorniertAm. " +
				"AuftragArt (aus den Daten abgeleitet, nicht bestätigt): 5 und 4 sind Rechnungen (5 am häufigsten, 4 meist mit Lieferschein), 6 hat negative Beträge (vermutlich Gutschrift), 1/2/3 sind selten und unklar. Für Umsätze nur 4 und 5 ohne StorniertAm zählen. " +
				"Atrpos: Belegpositionen; Auftragnr (-> Auftrag), Artikelnr (-> Artikel), Bezeichnung, Menge, EPreis, GPreisNetto. " +
				"Artikel: Artikelnr, Bezeichnung, VK1 (netto), EkPreis, Bestand. Offpos: offene Posten. " +
				"Felder mit Endung 'Wae' sind Altbeträge in DM, ignorieren.",
		},
	}
)

func loadKnownFile(dir string) {
	b, err := ioutil.ReadFile(filepath.Join(dir, "known.json"))
	if err != nil {
		return
	}
	var extra []Known
	if json.Unmarshal(b, &extra) == nil {
		addKnown(extra)
	}
}

func addKnown(extra []Known) {
	knownMu.Lock()
	defer knownMu.Unlock()
	for _, e := range extra {
		replaced := false
		for i := range known {
			if strings.EqualFold(known[i].Name, e.Name) {
				known[i] = e
				replaced = true
			}
		}
		if !replaced {
			known = append(known, e)
		}
	}
}

func identify(ts []TableInfo) *Known {
	have := map[string]bool{}
	for _, t := range ts {
		have[strings.ToLower(t.Name)] = true
	}
	knownMu.Lock()
	defer knownMu.Unlock()
	var best *Known
	for i := range known {
		k := &known[i]
		ok := len(k.Requires) > 0
		for _, r := range k.Requires {
			if !have[strings.ToLower(r)] {
				ok = false
				break
			}
		}
		if ok && (best == nil || len(k.Requires) > len(best.Requires)) {
			best = k
		}
	}
	return best
}
