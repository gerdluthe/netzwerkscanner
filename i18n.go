package main

// Language handling: English strings in the code are the keys, German is the
// default display language.

var lang = "de"

func setLanguage(l string) {
	if l != "en" {
		l = "de"
	}
	lang = l
	platformLanguageHook()
}

// T translates an English UI string into the current language.
func T(s string) string {
	if lang == "de" {
		if t, ok := de[s]; ok {
			return t
		}
	}
	return s
}

var de = map[string]string{
	// main window / menus
	" – Network Scanner":                    " – Netzwerkscanner",
	"&File":                                 "&Datei",
	"&View":                                 "&Ansicht",
	"&Actions":                              "A&ktionen",
	"&Reports":                              "&Berichte",
	"&Options":                              "&Optionen",
	"&Bookmarks":                            "&Lesezeichen",
	"&Help":                                 "&Hilfe",
	"IPv4 From":                             "IPv4 von",
	"To":                                    "bis",
	"Start Scanning":                        "Scan starten",
	"Stop Scanning":                         "Scan stoppen",
	"Filter:":                               "Filter:",
	"Apply":                                 "Anwenden",
	"Clear":                                 "Löschen",
	"Prev":                                  "Zurück",
	"Next":                                  "Weiter",
	"Ready":                                 "Bereit",
	"Threads":                               "Threads",
	"Threads: 0":                            "Threads: 0",
	"Threads: %d":                           "Threads: %d",
	"Devices: 0 / 0":                        "Geräte: 0 / 0",
	"Devices: %d / %d":                      "Geräte: %d / %d",
	"Devices: %d / %d (filtered: %d)":       "Geräte: %d / %d (gefiltert: %d)",
	"Scanning... %d%%":                      "Scanne... %d%%",
	"Paused (%d%%)":                         "Pausiert (%d%%)",
	"Stopping...":                           "Wird gestoppt...",
	"Rescanning %d...":                      "Erneuter Scan: %d...",
	"Scan time: %s":                         "Scandauer: %s",
	"%d of %d  ·  %s":                       "%d von %d  ·  %s",
	"+%d more":                              "+%d weitere",
	"Not found: ":                           "Nicht gefunden: ",
	"Saved ":                                "Gespeichert: ",
	"Detecting external IP address...":      "Externe IP-Adresse wird ermittelt...",
	"Additional ranges / CIDR / host names": "Weitere Bereiche / CIDR / Hostnamen",
	"Auto detect local IP range":            "Lokalen IP-Bereich automatisch erkennen",
	"Paste IP range from clipboard":         "IP-Bereich aus Zwischenablage einfügen",
	"Add current range to bookmarks":        "Aktuellen Bereich zu Lesezeichen hinzufügen",

	// actions
	"New Window":                             "Neues Fenster",
	"Clear Results":                          "Ergebnisse löschen",
	"Load Results...":                        "Ergebnisse laden...",
	"Save Results...":                        "Ergebnisse speichern...",
	"Import IP/Host Name List...":            "IP-/Hostnamen-Liste importieren...",
	"Save to File...":                        "In Datei speichern...",
	"Load from File...":                      "Aus Datei laden...",
	"Reset to Default":                       "Auf Standard zurücksetzen",
	"Current Config":                         "Aktuelle Konfiguration",
	"Exit":                                   "Beenden",
	"Auto Expand":                            "Automatisch aufklappen",
	"Collapse All":                           "Alle zuklappen",
	"Expand All":                             "Alle aufklappen",
	"Shared Folders":                         "Freigegebene Ordner",
	"Setup Filter":                           "Filter einrichten",
	"Visible Columns...":                     "Sichtbare Spalten...",
	"Grid Lines":                             "Gitternetzlinien",
	"Alternating Row Colours":                "Abwechselnde Zeilenfarben",
	"Highlight New Devices":                  "Neue Geräte hervorheben",
	"Remove Sorting":                         "Sortierung aufheben",
	"Legend":                                 "Legende",
	"Highlight Current Line":                 "Aktuelle Zeile hervorheben",
	"Clear Highlight":                        "Hervorhebung entfernen",
	"Quick Host Filter":                      "Schnellfilter",
	"Tree":                                   "Baum",
	"Rescan All Displayed Devices":           "Alle angezeigten Geräte erneut scannen",
	"Rescan Previously Undiscovered Devices": "Bisher nicht gefundene Geräte erneut scannen",
	"Send Wake-On-LAN Signal":                "Wake-On-LAN-Signal senden",
	"Save MAC to WOL Manager":                "MAC im WOL-Manager speichern",
	"Send Message...":                        "Nachricht senden...",
	"Remote Shutdown...":                     "Fernherunterfahren...",
	"DHCP Server Discovery...":               "DHCP-Server suchen...",
	"UPnP Device Discovery...":               "UPnP-Geräte suchen...",
	"mDNS Device Discovery...":               "mDNS-Geräte suchen...",
	"Find Duplicate MAC Addresses...":        "Doppelte MAC-Adressen suchen...",
	"Delete Inactive Devices":                "Inaktive Geräte löschen",
	"Find...":                                "Suchen...",
	"Find Next":                              "Weitersuchen",
	"Compare Scans...":                       "Scans vergleichen...",
	"Device Summary...":                      "Geräteübersicht...",
	"Program Options...":                     "Programmoptionen...",
	"Auto Detect Local IP Range":             "Lokalen IP-Bereich automatisch erkennen",
	"Detect External IP Address":             "Externe IP-Adresse ermitteln",
	"Detect IP Range from Host Name...":      "IP-Bereich aus Hostname ermitteln...",
	"Paste IP from Clipboard":                "IP aus Zwischenablage einfügen",
	"Wake-On-LAN Manager...":                 "Wake-On-LAN-Manager...",
	"Friendly Names / Comments...":           "Anzeigenamen / Kommentare...",
	"Ignored IP Addresses...":                "Ignorierte IP-Adressen...",
	"Additional Ranges...":                   "Weitere Bereiche...",
	"Add Current Range...":                   "Aktuellen Bereich hinzufügen...",
	"Manage Bookmarks...":                    "Lesezeichen verwalten...",
	"Help":                                   "Hilfe",
	"About...":                               "Über...",
	"IP Address":                             "IP-Adresse",
	"Open Device":                            "Gerät öffnen",
	"Copy":                                   "Kopieren",
	"As Web (HTTP)":                          "Als Web (HTTP)",
	"As Secure Web (HTTPS)":                  "Als sicheres Web (HTTPS)",
	"As File Server (FTP)":                   "Als Dateiserver (FTP)",
	"As SSH...":                              "Per SSH...",
	"As Windows Share (Explorer)":            "Als Windows-Freigabe (Explorer)",
	"As Remote Desktop":                      "Per Remotedesktop",
	"MAC Address":                            "MAC-Adresse",
	"Host Name":                              "Hostname",
	"Selected Rows":                          "Ausgewählte Zeilen",
	"All Rows":                               "Alle Zeilen",
	"Properties":                             "Eigenschaften",
	"Rescan Device":                          "Gerät erneut scannen",
	"Setup Filter by This Value":             "Nach diesem Wert filtern",
	"Assign Friendly Name...":                "Anzeigenamen vergeben...",
	"Delete from List":                       "Aus Liste löschen",
	"Explore":                                "Öffnen",
	"Map Network Drive...":                   "Netzlaufwerk verbinden...",
	"Copy Path":                              "Pfad kopieren",
	"Wake-On-LAN":                            "Wake-On-LAN",
	"Language":                               "Sprache",

	// columns
	"IP address":     "IP-Adresse",
	"MAC address":    "MAC-Adresse",
	"Response Time":  "Antwortzeit",
	"Host name":      "Hostname",
	"Device Type":    "Gerätetyp",
	"MAC Vendor":     "Hersteller",
	"Open Ports":     "Offene Ports",
	"Workgroup":      "Arbeitsgruppe",
	"Logged-on User": "Angemeldeter Benutzer",
	"Ping Method":    "Ping-Methode",
	"HTTP Server":    "HTTP-Server",
	"Web Page Title": "Webseitentitel",
	"Comment":        "Kommentar",
	"Last Scan":      "Letzter Scan",
	"Port %d":        "Port %d",

	// filter operations
	"contains":         "enthält",
	"does not contain": "enthält nicht",
	"equals":           "ist gleich",
	"not equal":        "ist ungleich",
	"starts with":      "beginnt mit",
	"ends with":        "endet mit",
	"is empty":         "ist leer",
	"is not empty":     "ist nicht leer",
	"greater than":     "größer als",
	"less than":        "kleiner als",

	// share kinds
	"Shared printer":       "Freigegebener Drucker",
	"Administrative share": "Administrative Freigabe",
	"Device":               "Gerät",
	"Standard share":       "Standardfreigabe",

	// device types
	"This computer":           "Dieser Computer",
	"Router (FRITZ!Box)":      "Router (FRITZ!Box)",
	"AVM FRITZ! device":       "AVM FRITZ!-Gerät",
	"Home Assistant":          "Home Assistant",
	"Raspberry Pi":            "Raspberry Pi",
	"IoT device (ESP)":        "IoT-Gerät (ESP)",
	"Shelly IoT device":       "Shelly IoT-Gerät",
	"Victron device":          "Victron-Gerät",
	"IP camera":               "IP-Kamera",
	"IP camera / media":       "IP-Kamera / Medien",
	"Printer":                 "Drucker",
	"NAS":                     "NAS",
	"Sonos speaker":           "Sonos-Lautsprecher",
	"Amazon device":           "Amazon-Gerät",
	"Google device":           "Google-Gerät",
	"Apple device":            "Apple-Gerät",
	"Router / Gateway":        "Router / Gateway",
	"Network device":          "Netzwerkgerät",
	"Windows computer":        "Windows-Computer",
	"Smart TV":                "Smart-TV",
	"Mobile device":           "Mobilgerät",
	"Computer":                "Computer",
	"MQTT broker":             "MQTT-Broker",
	"Modbus device":           "Modbus-Gerät",
	"Linux/Unix host":         "Linux/Unix-Host",
	"Linux / embedded device": "Linux / Embedded-Gerät",
	"Windows device":          "Windows-Gerät",

	// messages
	"Cannot create main window: ": "Hauptfenster kann nicht erstellt werden: ",
	"Shared folder enumeration is disabled.\nEnable it in Options → Shares and rescan.": "Die Suche nach Freigaben ist deaktiviert.\nAktivieren Sie sie unter Optionen → Freigaben und scannen Sie erneut.",
	"No active IPv4 network adapter found.":                                             "Kein aktiver IPv4-Netzwerkadapter gefunden.",
	"Select a network adapter:":                                                         "Netzwerkadapter auswählen:",
	"The subnet %s is very large. Continue?":                                            "Das Subnetz %s ist sehr groß. Fortfahren?",
	"The clipboard does not contain text.":                                              "Die Zwischenablage enthält keinen Text.",
	"No IP address found in the clipboard.":                                             "In der Zwischenablage wurde keine IP-Adresse gefunden.",
	"Detect IP Range from Host Name":                                                    "IP-Bereich aus Hostname ermitteln",
	"Host name or IP address:":                                                          "Hostname oder IP-Adresse:",
	"Cannot resolve ":                                                                   "Auflösung nicht möglich: ",
	"Could not detect the external IP address:\n":                                       "Die externe IP-Adresse konnte nicht ermittelt werden:\n",
	"\nHost name: ":                                                                     "\nHostname: ",
	"External IP Address":                                                               "Externe IP-Adresse",
	"Your external IP address is ":                                                      "Ihre externe IP-Adresse lautet ",
	"\n\nCopy to clipboard?":                                                            "\n\nIn die Zwischenablage kopieren?",
	"There are no results to save.":                                                     "Es gibt keine Ergebnisse zum Speichern.",
	"Save Results":                                                                      "Ergebnisse speichern",
	"Load Results":                                                                      "Ergebnisse laden",
	"XML document (*.xml)|*.xml|JSON (*.json)|*.json|CSV - comma separated values (*.csv)|*.csv|HTML document (*.html)|*.html|Text file (*.txt)|*.txt": "XML-Dokument (*.xml)|*.xml|JSON (*.json)|*.json|CSV - getrennte Werte (*.csv)|*.csv|HTML-Dokument (*.html)|*.html|Textdatei (*.txt)|*.txt",
	"Scan results (*.xml;*.json)|*.xml;*.json|All files (*.*)|*.*":                                                                                     "Scan-Ergebnisse (*.xml;*.json)|*.xml;*.json|Alle Dateien (*.*)|*.*",
	"Scan results (*.xml;*.json)|*.xml;*.json":                                                                                                         "Scan-Ergebnisse (*.xml;*.json)|*.xml;*.json",
	"Cannot save: ":            "Speichern nicht möglich: ",
	"Import IP/Host Name List": "IP-/Hostnamen-Liste importieren",
	"Text files (*.txt;*.csv)|*.txt;*.csv|All files (*.*)|*.*": "Textdateien (*.txt;*.csv)|*.txt;*.csv|Alle Dateien (*.*)|*.*",
	"%d entries imported. Click Start Scanning to scan them.":  "%d Einträge importiert. Klicken Sie auf „Scan starten“, um sie zu scannen.",
	"Save Configuration":                          "Konfiguration speichern",
	"Load Configuration":                          "Konfiguration laden",
	"Configuration (*.json)|*.json":               "Konfiguration (*.json)|*.json",
	"Reset all settings to their default values?": "Alle Einstellungen auf Standardwerte zurücksetzen?",
	"No duplicate MAC addresses found in the current results.\n(Several IPs on one MAC are typical for routers, VMs or proxy ARP.)": "In den aktuellen Ergebnissen wurden keine doppelten MAC-Adressen gefunden.\n(Mehrere IPs auf einer MAC sind typisch für Router, VMs oder Proxy-ARP.)",
	"Duplicate MAC Addresses": "Doppelte MAC-Adressen",
	"Vendor":                  "Hersteller",
	"Group":                   "Gruppe",
	"You are going to scan %d addresses. This may take a long time. Continue?": "Sie scannen %d Adressen. Das kann lange dauern. Fortfahren?",
	"SSH":                                  "SSH",
	"User name (leave empty for default):": "Benutzername (leer lassen für Standard):",
	"Map Network Drive":                    "Netzlaufwerk verbinden",
	" to drive:":                           " mit Laufwerk verbinden:",

	// generic dialogs
	"OK":                "OK",
	"Cancel":            "Abbrechen",
	"Close":             "Schließen",
	"Refresh":           "Aktualisieren",
	"Copy All":          "Alle kopieren",
	"Copy Selected":     "Auswahl kopieren",
	"Open in Browser":   "Im Browser öffnen",
	"%d item(s)":        "%d Eintrag/Einträge",
	"%d item(s) found":  "%d Eintrag/Einträge gefunden",
	"Searching...":      "Suche läuft...",
	"Error: ":           "Fehler: ",
	"Additional Ranges": "Weitere Bereiche",
	"One entry per line. Supported formats:\r\n  192.168.2.1-192.168.2.254   ·   10.0.0.0/24   ·   192.168.5.10   ·   host name\r\nThese are scanned together with the From/To range.": "Ein Eintrag pro Zeile. Unterstützte Formate:\r\n  192.168.2.1-192.168.2.254   ·   10.0.0.0/24   ·   192.168.5.10   ·   Hostname\r\nDiese werden zusammen mit dem Von/Bis-Bereich gescannt.",
	"Ignored IP Addresses": "Ignorierte IP-Adressen",
	"These addresses (single IPs, ranges or CIDR) are skipped during scans:": "Diese Adressen (einzelne IPs, Bereiche oder CIDR) werden beim Scan übersprungen:",
	"Visible Columns":                                                      "Sichtbare Spalten",
	"  (when enabled in options)":                                          "  (wenn in den Optionen aktiviert)",
	"Device is online (responded to ping, ARP or TCP)":                     "Gerät ist online (antwortet auf Ping, ARP oder TCP)",
	"New device – seen for the first time":                                 "Neues Gerät – zum ersten Mal gesehen",
	"Device did not respond (shown when 'Display inactive devices' is on)": "Gerät antwortet nicht (sichtbar bei „Inaktive Geräte anzeigen“)",
	"Device has shared folders / child items – click to expand":            "Gerät hat Freigaben / Unterelemente – zum Aufklappen klicken",
	"Shared folder": "Freigegebener Ordner",
	"Open TCP port (when 'Show ports as tree nodes' is on)": "Offener TCP-Port (bei „Offene Ports als Baumknoten anzeigen“)",
	"About ":           "Über ",
	" Network Scanner": " Netzwerkscanner",
	"Version ":         "Version ",
	" (64-bit)":        " (64-Bit)",
	"Fast multi-threaded IPv4 scanner: ping, ARP, DNS/NetBIOS,\nMAC vendors, TCP ports, SMB shares, Wake-on-LAN,\nUPnP / mDNS / DHCP discovery.": "Schneller IPv4-Scanner mit vielen Threads: Ping, ARP, DNS/NetBIOS,\nMAC-Hersteller, TCP-Ports, SMB-Freigaben, Wake-on-LAN,\nUPnP- / mDNS- / DHCP-Suche.",
	"Icons: Icons8 Flat Color Icons and Microsoft Fluent UI (MIT).\nMAC vendor data: IEEE OUI registry.":                                         "Icons: Icons8 Flat Color Icons und Microsoft Fluent UI (MIT).\nHerstellerdaten: IEEE-OUI-Register.",
	" Help":                        " Hilfe",
	"Share":                        "Freigabe",
	"Type":                         "Typ",
	"Remark":                       "Bemerkung",
	"Status":                       "Status",
	"Online":                       "Online",
	"No response":                  "Keine Antwort",
	"Detected by":                  "Erkannt per",
	"Response time":                "Antwortzeit",
	"NetBIOS name":                 "NetBIOS-Name",
	"User":                         "Benutzer",
	"MAC vendor":                   "Hersteller",
	"Device type":                  "Gerätetyp",
	"Open TCP ports":               "Offene TCP-Ports",
	"HTTP server":                  "HTTP-Server",
	"Web page title":               "Webseitentitel",
	"Shares":                       "Freigaben",
	"Last scan":                    "Letzter Scan",
	"First seen":                   "Zuerst gesehen",
	"Properties – ":                "Eigenschaften – ",
	"Assign Friendly Name":         "Anzeigenamen vergeben",
	"Friendly name / comment for ": "Anzeigename / Kommentar für ",
	" (bound to its MAC address when known):": " (an die MAC-Adresse gebunden, falls bekannt):",
	"Friendly Names / Comments":               "Anzeigenamen / Kommentare",
	"One entry per line:  MAC-or-IP = name":   "Ein Eintrag pro Zeile:  MAC-oder-IP = Name",
	"Add Bookmark":                            "Lesezeichen hinzufügen",
	"Bookmark name:":                          "Name des Lesezeichens:",
	"Manage Bookmarks":                        "Lesezeichen verwalten",
	"One bookmark per line:  name | from - to | optional extra ranges (comma separated)": "Ein Lesezeichen pro Zeile:  Name | von - bis | optionale weitere Bereiche (durch Komma getrennt)",
	": MAC address unknown":                                ": MAC-Adresse unbekannt",
	"Wake-On-LAN signal sent to %d device(s).":             "Wake-On-LAN-Signal an %d Gerät(e) gesendet.",
	"Wake-On-LAN Manager":                                  "Wake-On-LAN-Manager",
	"%d MAC address(es) added to the Wake-On-LAN Manager.": "%d MAC-Adresse(n) zum Wake-On-LAN-Manager hinzugefügt.",
	"Name":                                "Name",
	"Broadcast / target":                  "Broadcast / Ziel",
	"Broadcast (optional)":                "Broadcast (optional)",
	"Add":                                 "Hinzufügen",
	"Update":                              "Aktualisieren",
	"Remove":                              "Entfernen",
	"Wake Up Selected":                    "Auswahl aufwecken",
	"Signal sent to %d device(s).":        "Signal an %d Gerät(e) gesendet.",
	"Remote Shutdown":                     "Fernherunterfahren",
	"Devices:":                            "Geräte:",
	"Action:":                             "Aktion:",
	"Shut down":                           "Herunterfahren",
	"Restart":                             "Neu starten",
	"Cancel pending shutdown":             "Geplantes Herunterfahren abbrechen",
	"Timeout (sec):":                      "Wartezeit (Sek.):",
	"Message:":                            "Nachricht:",
	"Force running applications to close": "Laufende Anwendungen zwangsweise schließen",
	"Uses Windows shutdown.exe; requires administrative rights on the target.": "Verwendet Windows shutdown.exe; erfordert Administratorrechte auf dem Zielgerät.",
	"Command sent successfully.": "Befehl erfolgreich gesendet.",
	"Send Message":               "Nachricht senden",
	"Message to display on the selected Windows computer(s) (uses msg.exe):": "Nachricht, die auf den ausgewählten Windows-Computern angezeigt wird (verwendet msg.exe):",
	"UPnP Device Discovery": "UPnP-Gerätesuche",
	"Manufacturer":          "Hersteller",
	"Model":                 "Modell",
	"Server":                "Server",
	"Location":              "Adresse (URL)",
	"mDNS Device Discovery": "mDNS-Gerätesuche",
	"Instance":              "Instanz",
	"Service":               "Dienst",
	"Host":                  "Host",
	"Port":                  "Port",
	"DHCP Server Discovery": "DHCP-Server-Suche",
	"DHCP server":           "DHCP-Server",
	"Offered IP":            "Angebotene IP",
	"Subnet mask":           "Subnetzmaske",
	"Router":                "Router",
	"DNS servers":           "DNS-Server",
	"Domain":                "Domäne",
	"Lease time":            "Lease-Dauer",
	"Perform a scan or load results first. The current results are compared with a saved scan.": "Bitte zuerst scannen oder Ergebnisse laden. Die aktuellen Ergebnisse werden mit einem gespeicherten Scan verglichen.",
	"Select a saved scan to compare with":                                                       "Gespeicherten Scan zum Vergleich auswählen",
	"New":                                                                                       "Neu",
	"Went offline":                                                                              "Offline gegangen",
	"Came online":                                                                               "Online gekommen",
	"Changed":                                                                                   "Geändert",
	"Name ":                                                                                     "Name ",
	"Ports [":                                                                                   "Ports [",
	"Missing":                                                                                   "Fehlt",
	"not in current results":                                                                    "nicht in den aktuellen Ergebnissen",
	"No differences found.":                                                                     "Keine Unterschiede gefunden.",
	"Compare Scans":                                                                             "Scans vergleichen",
	"Compare Scans – current vs. ":                                                              "Scans vergleichen – aktuell vs. ",
	"Change":                                                                                    "Änderung",
	"Details":                                                                                   "Details",
	"(unknown)":                                                                                 "(unbekannt)",
	"Total":                                                                                     "Gesamt",
	"Online devices":                                                                            "Online-Geräte",
	"Shared resources":                                                                          "Freigegebene Ressourcen",
	"Open port":                                                                                 "Offener Port",
	"Device Summary":                                                                            "Geräteübersicht",
	"Category":                                                                                  "Kategorie",
	"Value":                                                                                     "Wert",
	"Count":                                                                                     "Anzahl",

	// options dialog
	"Options":                 "Optionen",
	"General":                 "Allgemein",
	"Additional":              "Erweitert",
	"Workstation":             "Arbeitsstation",
	"Ports":                   "Ports",
	"View":                    "Ansicht",
	"Max. threads":            "Max. Threads",
	"Ping timeout, ms":        "Ping-Timeout, ms",
	"Attempts":                "Versuche",
	"Method":                  "Methode",
	"ICMP only":               "Nur ICMP",
	"ARP only (local subnet)": "Nur ARP (lokales Subnetz)",
	"Both ICMP and ARP":       "ICMP und ARP",
	"Always analyse device (even if ping failed)":                      "Gerät immer analysieren (auch wenn Ping fehlschlägt)",
	"Show successful ping method":                                      "Erfolgreiche Ping-Methode anzeigen",
	"Display inactive devices":                                         "Inaktive Geräte anzeigen",
	"Display Time To Live (TTL)":                                       "Time To Live (TTL) anzeigen",
	"Scan in random order":                                             "In zufälliger Reihenfolge scannen",
	"Detect duplicate MAC addresses":                                   "Doppelte MAC-Adressen erkennen",
	"Show last-scan time":                                              "Zeitpunkt des letzten Scans anzeigen",
	"Highlight devices seen for the first time":                        "Erstmals gesehene Geräte hervorheben",
	"Resolve host names":                                               "Hostnamen auflösen",
	"DNS, then NetBIOS":                                                "DNS, dann NetBIOS",
	"DNS only":                                                         "Nur DNS",
	"NetBIOS only":                                                     "Nur NetBIOS",
	"Resolve MAC addresses (ARP / NetBIOS)":                            "MAC-Adressen ermitteln (ARP / NetBIOS)",
	"Look up network card vendor":                                      "Hersteller der Netzwerkkarte ermitteln",
	"Recognise device type (offline heuristics)":                       "Gerätetyp erkennen (offline, heuristisch)",
	"Grab HTTP server banner and page title":                           "HTTP-Server-Kennung und Seitentitel abrufen",
	"Look up LAN group/domain (NetBIOS)":                               "Arbeitsgruppe/Domäne ermitteln (NetBIOS)",
	"Look up logged-on users (NetBIOS)":                                "Angemeldete Benutzer ermitteln (NetBIOS)",
	"Check for open TCP ports (one or more, comma- or dash-separated)": "Auf offene TCP-Ports prüfen (einer oder mehrere, mit Komma oder Bindestrich getrennt)",
	"Examples:  80,443   ·   21-23,80,443,445,3389,8080   ·   1-1024":  "Beispiele:  80,443   ·   21-23,80,443,445,3389,8080   ·   1-1024",
	"Display service names rather than port numbers":                   "Dienstnamen statt Portnummern anzeigen",
	"Show open ports as tree nodes under each device":                  "Offene Ports als Baumknoten unter jedem Gerät anzeigen",
	"Enumerate shared folders and printers (SMB)":                      "Freigegebene Ordner und Drucker auflisten (SMB)",
	"Show administrative shares (C$, ADMIN$ ...)":                      "Administrative Freigaben anzeigen (C$, ADMIN$ ...)",
	"Show shared printers":                                             "Freigegebene Drucker anzeigen",
	"Shares are listed as child nodes of each device. Uses your current Windows credentials.": "Freigaben werden als Unterknoten jedes Geräts angezeigt. Es werden Ihre aktuellen Windows-Anmeldedaten verwendet.",
	"MAC address format":      "MAC-Adressformat",
	"CSV file delimiter":      "CSV-Trennzeichen",
	"Display grid lines":      "Gitternetzlinien anzeigen",
	"Alternating row colours": "Abwechselnde Zeilenfarben",
	"Auto expand tree nodes":  "Baumknoten automatisch aufklappen",

	// errors from the scanner
	"invalid CIDR: %s":                           "ungültige CIDR-Angabe: %s",
	"invalid range: %s":                          "ungültiger Bereich: %s",
	"invalid IP address range: %s":               "ungültiger IP-Adressbereich: %s",
	"invalid IP address range: %s - %s":          "Ungültiger IP-Adressbereich: %s - %s",
	"invalid IP address: %s":                     "ungültige IP-Adresse: %s",
	"invalid MAC address: %s":                    "ungültige MAC-Adresse: %s",
	"cannot resolve host name: %s":               "Hostname kann nicht aufgelöst werden: %s",
	"please enter an IP address range to scan":   "Bitte einen IP-Adressbereich zum Scannen eingeben",
	"the range is too large (max. %d addresses)": "Der Bereich ist zu groß (max. %d Adressen)",
	"unsupported or damaged file: %v":            "Nicht unterstützte oder beschädigte Datei: %v",
	"cannot bind UDP port 68 (%v).\nRun the program as administrator or stop software that uses this port": "UDP-Port 68 kann nicht gebunden werden (%v).\nStarten Sie das Programm als Administrator oder beenden Sie Software, die diesen Port verwendet",

	"Filter results...": "Ergebnisse filtern...",

	// language switch
	"The language will change after restarting the program. Restart now?": "Die Sprache wird nach einem Neustart des Programms übernommen. Jetzt neu starten?",
}

func helpText() string {
	if lang == "de" {
		return `SCHNELLSTART
1. IP-Bereich eingeben (IPv4 von / bis) oder "Automatisch erkennen" (Strg+I).
2. Auf "Scan starten" klicken (F5). Ergebnisse erscheinen schon während des Scans;
   Pause/Stopp jederzeit möglich.
3. Rechtsklick auf ein Gerät: öffnen per HTTP/HTTPS/FTP/SSH/RDP/Explorer, kopieren,
   Wake-on-LAN, erneut scannen, Anzeigenamen vergeben, herunterfahren, Nachricht, löschen.

WEITERE BEREICHE
Das grüne + öffnet "Weitere Bereiche": beliebig viele Bereiche, CIDR-Netze
(10.0.0.0/24), einzelne Adressen oder Hostnamen, einer pro Zeile.
Mit Lesezeichen lassen sich Bereiche speichern.

OPTIONEN (Strg+P)
Allgemein: Threads, Ping-Timeout, Methode (ICMP / ARP / beide), Versuche, inaktive Geräte.
Erweitert: Hostnamen (DNS / NetBIOS), MAC-Adressen, Hersteller, HTTP-Kennung.
Ports: zu prüfende TCP-Ports, z. B. 21-23,80,443,3389,8080.
Freigaben: freigegebene Ordner und Drucker auflisten (als Baumknoten).

TASTEN
F5 Start/Stopp · Strg+F Suchen · F3 Weitersuchen · Strg+L Filter · Strg+S Speichern · Strg+O Laden
Rechts/Links oder Klick auf den Punkt: auf-/zuklappen · Entf: aus Liste löschen

SPRACHE
Hilfe → Sprache: Deutsch oder English (wirkt nach Neustart).

KOMMANDOZEILE
netzscan.exe /range:192.168.1.1-192.168.1.254 /auto:"C:\scan.csv"   (xml, json, csv, html, txt)
netzscan.exe /config:"C:\meine.json"  verwendet eine gespeicherte Konfiguration.`
	}
	return `QUICK START
1. Enter an IP range (IPv4 From / To) or press "Auto detect" (Ctrl+I).
2. Click "Start Scanning" (F5). Results appear while scanning; Pause/Stop at any time.
3. Right-click a device: open as HTTP/HTTPS/FTP/SSH/RDP/Explorer, copy, Wake-on-LAN,
   rescan, assign a friendly name, remote shutdown, send message, delete.

MORE RANGES
The green + button opens "Additional Ranges": any number of ranges, CIDR blocks
(10.0.0.0/24), single addresses or host names, one per line. Use Bookmarks to save ranges.

OPTIONS (Ctrl+P)
General: threads, ping timeout, method (ICMP / ARP / both), attempts, inactive devices.
Additional: host names (DNS / NetBIOS), MAC addresses, vendor lookup, HTTP banners.
Ports: TCP ports to check, e.g. 21-23,80,443,3389,8080.
Shares: enumerate shared folders and printers (shown as tree nodes).

KEYS
F5 start/stop · Ctrl+F find · F3 next · Ctrl+L filter · Ctrl+S save · Ctrl+O load
Right/Left or click on the dot: expand / collapse · Del: remove from list

LANGUAGE
Help → Language: Deutsch or English (applies after restart).

COMMAND LINE
netzscan.exe /range:192.168.1.1-192.168.1.254 /auto:"C:\scan.csv"   (xml, json, csv, html, txt)
netzscan.exe /config:"C:\my.json"  uses a saved configuration.`
}
