# Replado Connector

Ein kleines Programm für Windows, macOS und Linux. Es macht Daten alter Software, die nur auf einem PC im
Unternehmen läuft (zum Beispiel eine Warenwirtschaft wie Topfakt), für die KI von
[Replado](https://replado.com) abfragbar. Die Datenbank wird dabei nie hochgeladen. Der Connector beantwortet nur
einzelne Fragen und liest dabei nur.

## Grundsätze

- **Nur lesen.** Der Connector öffnet Dateien nur zum Lesen und ändert nichts.
- **Nur freigegebene Ordner.** Er durchsucht nicht den ganzen Rechner, sondern nur die Ordner, die ein
  Administrator ausgewählt hat. Laufwerke, der Benutzerordner und Systemordner lassen sich nicht freigeben.
- **Bestätigung je Ordner.** Vor der Freigabe zeigt der Connector, was er im Ordner gefunden hat. Der
  Administrator bestätigt mit einem Haken, dass die Lizenz der Software das Auslesen durch ein anderes Programm
  erlaubt. Text, Zeitpunkt und Person speichert Replado.
- **Verbindung nur nach außen.** Der Connector baut die Verbindung selbst auf (HTTPS, Long-Polling). Am PC
  oder in der Firewall muss nichts geöffnet werden.
- **Kontrolle.** Ein Protokoll zeigt jede Abfrage mit Tabelle und Zeilenzahl, aber ohne Inhalte. In Replado
  kann ein Administrator jede Datenbank sperren, das Einlesen neu anstoßen oder den PC entfernen.

## Einrichtung

**Wohin?** Auf den Rechner, auf dem die Daten der Software liegen, und am besten auf einen, der immer läuft:
einen Server, ein NAS mit Linux oder den Büro-PC mit dem Datenordner. Ist der Connector aus, wartet Replado mit der
Frage und holt sie automatisch nach, sobald er wieder läuft. Auf den einzelnen Arbeitsplätzen wird er nicht gebraucht.

1. Die passende Datei aus den [Releases](../../releases/latest) laden:

   | Datei | System |
   |---|---|
   | `Replado-Connector.exe` | Windows 7, 8, 8.1, 10, 11 |
   | `Replado-Connector-Vista.exe` | Windows Vista |
   | `Replado-Connector-macOS.zip` | macOS 10.13 und neuer, Intel und Apple-Chip |
   | `replado-connector-linux-amd64.tar.gz`, `…-arm64.tar.gz` | Linux |

   `SHA256SUMS.txt` enthält die Prüfsummen.
2. Starten. Im Browser öffnet sich die Einrichtung. Sie ist nur auf diesem PC erreichbar
   (`127.0.0.1`, zufälliger Port und Schlüssel).
3. Mit der Adresse des eigenen Replado, E-Mail und Passwort anmelden. Das geht nur als Administrator. Ist die
   Zwei-Faktor-Anmeldung aktiv, fragt der Connector nach dem Code.
4. Den Datenordner der Software wählen, auf **Prüfen** klicken, die Lizenz bestätigen und den Ordner freigeben.

Nach der Anmeldung richtet sich der Connector als Hintergrunddienst des Benutzers ein, ohne Administratorrechte:

| System | Ort | Autostart |
|---|---|---|
| Windows | `%LOCALAPPDATA%\Replado` | `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` |
| macOS | `~/Library/Application Support/Replado` | LaunchAgent `com.replado.connector` |
| Linux | `~/.config/replado-connector` | systemd-Benutzerdienst, sonst XDG-Autostart |

Unter Windows zeigt ein Symbol neben der Uhr, ob der Connector mit Replado verbunden ist (farbig) oder nicht (grau).
Ein Klick öffnet die Einrichtung, ein Rechtsklick zeigt den Zustand und „Connector beenden“. Die Einrichtung ist außerdem
im Startmenü unter „Replado Connector“ zu finden.

### Ohne Bildschirm (Linux-Server)

```
replado-connector login --server https://replado.firma.de --email admin@firma.de
replado-connector folder-add /srv/warenwirtschaft/daten            # zeigt Fund und Bestätigungstext
replado-connector folder-add /srv/warenwirtschaft/daten --lizenz-bestaetigt
replado-connector status
replado-connector log
```

Das Passwort kommt aus der Eingabe oder aus `REPLADO_PASSWORD`. `replado-connector` ohne Angabe zeigt alle Befehle.

## Formate

- Paradox (`.DB`): Topfakt und viele Programme mit der Borland Database Engine. Passwortgeschützte Tabellen
  werden erkannt, aber nicht gelesen.
- dBase, FoxPro, Clipper (`.DBF`), noch ohne Memo-Dateien.

## Werkzeuge für die KI

`describe_table`, `query` (Filter `eq`, `ne`, `contains`, `gt`, `gte`, `lt`, `lte`, Sortierung, Limit) und
`search` (Volltext). Replado prüft jeden Aufruf vorher: Die Quelle muss in einem freigegebenen Ordner liegen und
darf nicht gesperrt sein, die Tabelle muss es geben, das Limit ist höchstens 50 Zeilen. Der Connector prüft
dasselbe noch einmal.

## Protokoll zwischen Connector und Replado

Alle Anfragen gehen vom Connector aus, `POST` mit JSON, nach der Anmeldung mit `Authorization: Bearer <zugang>`.
Replado speichert den Zugang nur als Hash.

| Pfad | Anfrage | Antwort |
|---|---|---|
| `/api/connector/v1/login` | `email`, `password`, `code?`, `machine`, `os`, `arch`, `version`, `tools` | `connector_id`, `secret`, `known` |
| `/api/connector/v1/poll` | `version`, `os`, `sources?`, `folders?` | nach höchstens 25 s: `calls`, `rescan?`, `known?` |
| `/api/connector/v1/result` | `id`, `result` oder `error` | 204 |
| `/api/connector/v1/update` | `version`, `target` | `allowed` |
| `/api/connector/v1/logout` | | 204 |

`sources` und `folders` kommen nur mit, wenn sich etwas geändert hat. 401 bei `poll` heißt: Der Connector wurde in
Replado entfernt. `known` enthält Beschreibungen bekannter Software, damit die KI die Tabellen versteht.

## Updates

Der installierte Connector sucht drei Minuten nach dem Start und danach alle sechs Stunden nach einer neuen Version:

1. Er lädt `update.json` und `update.json.sig` aus dem neuesten Release auf GitHub. Weiterleitungen sind nur zu
   `github.com` und `*.githubusercontent.com` erlaubt.
2. Er prüft die Signatur (ECDSA P-256 mit SHA-256) gegen den öffentlichen Schlüssel in `updater.go`. Ohne gültige
   Signatur passiert nichts. Den privaten Schlüssel hat nur der Herausgeber.
3. Ist die Version neuer, fragt er Replado (`/api/connector/v1/update`). Der Admin kann automatische Updates je PC
   unter Datenquellen abschalten. Replado vermerkt jedes Update im Audit-Log.
4. Er lädt die passende Datei, vergleicht ihre SHA-256-Prüfsumme mit `update.json`, startet die neue Datei probeweise
   mit `version` und tauscht sie erst dann aus. Die vorige Version bleibt als `.alt` liegen.
5. Er startet neu und vermerkt das Update in seinem Protokoll.

## Entwicklung

Der Code nutzt nur die Standardbibliothek und läuft mit Go 1.10 (für Vista) bis zur aktuellen Version.
Funktionen neuer als Go 1.10 bitte nicht verwenden.

```
make test        # go vet und Tests mit Race-Prüfung
make test-alt    # dieselben Tests mit Go 1.10 und Go 1.20 in Docker
make all         # Tests und alle Pakete nach dist/
make release     # wie all, dazu update.json und Signatur (Schlüssel in ~/.replado/connector-update-key.pem)
```

Zu einem Release gehören alle Dateien aus `dist/`, auch `update.json` und `update.json.sig`.

Zum Ausprobieren ohne Replado:

```
go build -o rc . && ./rc scan /pfad/zu/daten
./rc call query '{"source":"<id>","table":"Kunden","limit":3}' --path /pfad/zu/daten
./rc mcp /pfad/zu/daten
```

## Lizenz

MIT, siehe [LICENSE](LICENSE).
