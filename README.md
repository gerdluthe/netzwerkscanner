# NetzScan – Netzwerkscanner

Schneller, multithreaded IPv4-Netzwerkscanner für **Windows, macOS, Android und Linux**.
Oberfläche auf **Deutsch** (Standard) oder **Englisch**.

- Ping (ICMP) und ARP, alternativ TCP-Erkennung, wenn ICMP nicht erlaubt ist
- Hostnamen per DNS und NetBIOS, Arbeitsgruppe
- MAC-Adressen und Hersteller (IEEE-OUI-Datenbank eingebaut, offline)
- Offene TCP-Ports, HTTP-Server-Kennung und Webseitentitel
- Erkennung des Gerätetyps (Router, Drucker, NAS, IoT, Kamera …)
- Windows: Freigaben (SMB) als Baum, Remote-Shutdown, Nachricht senden, Netzlaufwerk verbinden
- Wake-on-LAN inkl. Manager, UPnP-, mDNS- und DHCP-Suche, externe IP
- Filter, Suche, Sortierung, sichtbare Spalten, Anzeigenamen, Lesezeichen, Scan-Vergleich
- Export: CSV, HTML, TXT, XML, JSON – Laden von XML/JSON
- Kommandozeile (Windows/Linux/macOS): `netzscan /range:192.168.1.1-192.168.1.254 /auto:scan.csv`

## Plattformen und Oberflächen

| Plattform | Oberfläche | Datei aus dem Build |
|-----------|------------|---------------------|
| Windows x64 | native Win32-Oberfläche (klassisches Look & Feel) | `NetzScan.exe` |
| macOS (Apple Silicon) | Fyne | `NetzScan-macOS-arm64.zip` (enthält `NetzScan.app`) |
| Android | Fyne | `NetzScan-Android.apk` |
| Linux x64 | Fyne | `NetzScan-Linux-x64.tar.gz` |

Plattform-Einschränkungen:

- **Android 10+** gibt Apps keine MAC-Adressen anderer Geräte preis → Spalten MAC/Hersteller bleiben leer.
  Wenn ICMP nicht erlaubt ist, erkennt NetzScan Geräte automatisch per TCP.
- **Freigaben (SMB)**, Remote-Shutdown und „Nachricht senden“ gibt es nur unter Windows.
- DHCP-Suche benötigt UDP-Port 68 (unter macOS/Linux meist nur mit Administratorrechten).

## Mit GitHub bauen (empfohlen)

1. Neues Repository auf GitHub anlegen und den Inhalt dieses Ordners hochladen
   (`git init && git add . && git commit -m "NetzScan" && git branch -M main && git remote add origin <URL> && git push -u origin main`).
2. GitHub startet automatisch den Workflow **Build** (`.github/workflows/build.yml`).
3. Nach ca. 10–15 Minuten: Tab **Actions** → letzter Lauf → unten bei **Artifacts** die Dateien für Windows, macOS, Android und Linux herunterladen.
4. Für eine Release mit Download-Seite ein Tag pushen:
   ```
   git tag v1.1.0
   git push origin v1.1.0
   ```
   Dann erscheint unter **Releases** eine Version mit allen Dateien. Die Versionsnummer des Tags wird in die Programme übernommen.

Hinweis: `go.sum` wird im Workflow per `go mod tidy` erzeugt. Wer reproduzierbare Builds möchte, führt einmal lokal `go mod tidy` aus und committet die entstandene `go.sum`.

### Installation der Builds

- **Windows:** `NetzScan.exe` starten. Die Datei ist nicht signiert – SmartScreen: „Weitere Informationen“ → „Trotzdem ausführen“.
- **macOS:** ZIP entpacken, `NetzScan.app` in „Programme“ ziehen. Da die App nicht signiert/notarisiert ist, beim ersten Start Rechtsklick → **Öffnen** (oder im Terminal: `xattr -dr com.apple.quarantine /Applications/NetzScan.app`).
- **Android:** APK aufs Handy kopieren und öffnen, „Installation aus unbekannten Quellen“ erlauben. Die APK ist mit einem Debug-Schlüssel signiert.
- **Linux:** entpacken, `./netzscan` starten.

## Lokal bauen

Voraussetzung: Go 1.24 oder neuer.

**Windows-Version** (geht von jedem Betriebssystem aus, ohne C-Compiler):

```
go run github.com/akavel/rsrc@v0.10.2 -manifest netzscan.manifest -ico app.ico -arch amd64 -o rsrc_windows_amd64.syso
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H windowsgui" -o NetzScan.exe .
```
(oder `./build-windows.sh`)

**macOS / Linux** (benötigt C-Compiler; Linux zusätzlich `libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev`):

```
go build -o netzscan .
# macOS-App-Bundle:
go install fyne.io/tools/cmd/fyne@v1.7.3
fyne package -os darwin -icon Icon.png -name NetzScan -app-id de.netzscan.app
```

**Android** (Android SDK + NDK, `ANDROID_HOME` und `ANDROID_NDK_HOME` gesetzt):

```
fyne package -os android -icon Icon.png -name NetzScan -app-id de.netzscan.app
```

## Projektaufbau

| Datei | Inhalt |
|-------|--------|
| `scanner.go`, `netbios.go`, `discovery.go`, `columns.go`, `export.go`, `settings.go`, `i18n.go` | gemeinsamer Kern (alle Plattformen) |
| `*_windows.go` | native Windows-Oberfläche (walk) und Win32-Funktionen (ICMP, ARP, Freigaben) |
| `gui_fyne.go`, `net_unix.go`, `arp_darwin.go`, `arp_linux.go` | Fyne-Oberfläche und Netzwerkfunktionen für macOS/Android/Linux |
| `third_party/walk` | angepasste Kopie der Bibliothek [lxn/walk](https://github.com/lxn/walk) (BSD-3) |
| `assets/` | Icons (PNG) und OUI-Herstellerdatenbank |
| `i18n.go` | Übersetzungen – neue Texte im Code als englischen Schlüssel mit `T("...")` schreiben und die deutsche Übersetzung in `de` ergänzen |

Einstellungen werden gespeichert unter `%APPDATA%\NetzScan\config.json` (Windows), `~/Library/Application Support/NetzScan` (macOS), `~/.config/NetzScan` (Linux) bzw. im App-Speicher (Android).

## Hinweis

NetzScan ist eine eigenständige Neuentwicklung. Es steht in keiner Verbindung zu SoftPerfect und enthält keinen Code und keine Grafiken des SoftPerfect Network Scanners.

Lizenz: siehe [LICENSE](LICENSE), Drittkomponenten: [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
