# Transcribe by Moliri

Transcribe by Moliri macht aus Audioaufnahmen ein lesbares, zeitgenaues Transkript mit Sprechern,
Sprachen und Geräusch-Markern und hilft dabei, es dort zu korrigieren, wo die Maschine scheitert:
Mundart, Plattdeutsch, Namen, Fremdsprache, Gesang. Gedacht ist es für Archiv- und Sammlungsteams,
Ehrenamtliche und Forschende in Museen, die Zeitzeugen-Interviews, Rundgänge oder Mitschnitte
erschließen.

- **Läuft lokal.** Eine portable Windows-EXE mit eingebauter Oberfläche; die Aufnahmen verlassen den
  Rechner nicht.
- **Korrekturen bleiben erhalten.** Menschliche Korrekturen liegen getrennt vom Maschinenergebnis.
  Neu berechnen überschreibt sie nie.
- **Hören und Mitlesen.** Der Editor spielt die Aufnahme ab und markiert die gesprochene Stelle; Sprecher
  benennen und zusammenlegen, Text ersetzen, Clips markieren und exportieren.

Die Oberfläche ist deutsch.

## Download

Die fertige Windows-EXE gibt es unter
[Releases](https://github.com/moliri-app/transcribe/releases). Sie braucht keine Installation: in einen
Ordner legen und starten. Die Daten, also Aufnahmen (`audio/`), Ergebnisse (`results/`), Modelle
(`models/`) und Einstellungen, liegen im Benutzerprofil unter `%LOCALAPPDATA%\Transcribe by Moliri`,
nicht neben der EXE: Die EXE lässt sich verschieben oder ersetzen, ohne dass etwas verloren geht oder
die Modelle neu geladen werden. In den Einstellungen („Speicher und Modelle“) lässt sich ein anderer
Ort wählen, etwa ein Laufwerk mit mehr Platz; die App zieht die Daten dann dorthin um und startet neu.
Hatte eine frühere Version die Daten noch in `data/` neben der EXE, bleiben sie dort, bis man sie in
den Einstellungen umzieht.

Beim ersten Start lädt die App aus dem Internet, was sie zum Erkennen braucht: die
sherpa-onnx-Laufzeit, FFmpeg und die Sprach-, Sprecher- und Geräuschmodelle (zusammen einige hundert
Megabyte bis wenige Gigabyte, je nach gewählter Qualität). Danach arbeitet sie ohne Internet.

Beim Start und danach einmal am Tag fragt die App bei GitHub nach, ob es ein neueres Release gibt.
Wenn ja, zeigt sie es in der App-Leiste, in den Einstellungen („Über Transcribe“) und im Menü des
Symbols im Infobereich. Erst ein Klick auf „Jetzt aktualisieren“ lädt die neue EXE, legt sie an die
Stelle der alten (die bis zum nächsten Start als `transcribe.old.exe` daneben liegt) und startet die
App neu, sobald keine Aufträge und Aufnahmen mehr laufen. Die Daten bleiben dabei unberührt. Selbst
gebaute EXEs (`build.ps1`) fragen nie nach.

### Linux und macOS

Für Linux und macOS liegt jedem Release ein Archiv mit dem Programm `transcribe` bei:
`transcribe-linux-x64.tar.gz`, `transcribe-linux-arm64.tar.gz`, `transcribe-macos-arm64.tar.gz`
(Apple Silicon) und `transcribe-macos-x64.tar.gz` (Intel). Entpacken und im Terminal starten:

```sh
tar -xzf transcribe-linux-x64.tar.gz
./transcribe
```

Die App öffnet sich in Chrome, Edge, Chromium oder Brave als eigenes Fenster, sonst im
Standardbrowser. Die Daten liegen unter Linux in `~/.local/share/transcribe`, unter macOS in
`~/Library/Application Support/Transcribe by Moliri`. Die Erkennung und unter Linux auch FFmpeg lädt
die App beim ersten Start wie unter Windows; unter macOS braucht sie ein installiertes FFmpeg (`brew
install ffmpeg`). Das Entpacken der Downloads braucht `tar` mit bzip2 und xz, was Desktop-Systeme
mitbringen. Das macOS-Programm ist nicht von Apple beglaubigt: Aus dem Browser geladen, startet es
erst nach `xattr -d com.apple.quarantine transcribe`. Symbol im Infobereich, Diktat per Tastenkürzel
und die Aktualisierung aus der App gibt es nur unter Windows; ein neues Release lädt man hier selbst.

### Erkennung über eine API

In den **Einstellungen** unter „Erkennung“ lässt sich statt der Modelle auf dem Rechner ein Dienst
über seine API nutzen:

- **Nur auf diesem Rechner**: wie bisher, keine Aufnahme verlässt den Rechner.
- **Nur über eine API**: Aufnahmen gehen zur Erkennung an den Dienst. Die App lädt dann keine
  Modelle, nur FFmpeg.
- **Beides**: je eine Vorgabe für Live und Diktieren und für ganze Aufnahmen; in der Bibliothek
  und beim Ablegen von Dateien lässt sich je Aufnahme wählen.

Zur Wahl stehen Dienste mit der OpenAI-kompatiblen Schnittstelle (`/v1/audio/transcriptions`:
OpenAI, Groq, Mistral oder ein eigener Server wie Speaches oder LocalAI, auch im eigenen Netz) und
ElevenLabs (Scribe). Adresse, Modell und Schlüssel lassen sich anpassen. Die OpenAI-kompatible
Schnittstelle unterscheidet keine Sprecher und bekommt lange Aufnahmen in Stücken von höchstens zehn
Minuten; live geht jede Äußerung nach einer Sprechpause einzeln hin. ElevenLabs unterscheidet
Sprecher, erkennt Geräusche wie Lachen und zeigt live den Text schon beim Sprechen.

Der Schlüssel liegt in `settings.json` im Datenordner. Für Tests gibt es einen Nachbau beider
Schnittstellen in `internal/sttapi/sttapitest` (`pnpm start --api` in `e2e/`).

## Starten

```powershell
.\transcribe.exe
```

Die App startet einen lokalen Server auf `http://localhost:31765` (ist der Port belegt, auf einem
freien) und öffnet die Oberfläche in einem eigenen Fenster (Edge, Chrome oder Brave im App-Modus, sonst der Standardbrowser).

| Option | Bedeutung |
| --- | --- |
| `--data <ordner>` | Datenordner statt des in den Einstellungen gewählten (auch über `TRANSCRIBE_DATA`); er lässt sich dann nicht in der App umziehen |
| `--port <zahl>` | anderer Port als 31765 |
| `--no-browser` | kein Fenster öffnen, nur den Server starten |
| `--background` | ohne Fenster starten, nur mit dem Symbol im Infobereich (so startet der Autostart) |
| `--threads <zahl>` | Threads für Aufträge ohne eigene Zahl (sonst etwa einer je Prozessorkern) |

Die Startseite ist die **Bibliothek**: Die transkribierten Aufnahmen stehen dort als Blätter, jedes
mit dem Anfang seines Transkripts, den Sprechern und den Korrekturen von Hand; ein Klick öffnet es im
Editor. Darüber steht, was gerade transkribiert wird, und was noch kein Transkript hat. Zu wählen
sind nur die Sprecher (meist automatisch) und die Sprachen; die Zahl der Threads wählt die App selbst.

**Pausieren** hält die Transkriptionen an, damit der Rechner frei für anderes ist: Die laufende bleibt
an ihrer Stelle stehen, mit ihren Modellen im Arbeitsspeicher, die wartenden beginnen nicht.
**Fortsetzen** macht dort weiter. Die Pause hält auch über einen Neustart, die unterbrochene
Transkription beginnt dann von vorn. Während einer Live-Transkription (Live-Seite, Diktieren oder ein
anderes Programm über die Live-API) pausieren die Transkriptionen von selbst und gehen danach weiter,
damit die Live-Anfrage den Rechner zuerst hat.

Unter Windows zeigt die App ein Symbol im Infobereich der Taskleiste: Ein Klick öffnet das Fenster,
ein Rechtsklick zeigt „Einstellungen“ und „Beenden“, das mit der App auch ihre Fenster schließt.
Läuft die App schon, holt ein zweiter Start ihr offenes Fenster nach vorn (auch ein minimiertes) und
öffnet nur dann eins, wenn keins offen ist.

### Ordner verbinden

Links in der Bibliothek stehen die Bereiche: alle Aufnahmen, die hochgeladenen (ins Fenster gezogen
oder hinzugefügt; sie werden in den Datenordner kopiert), die Live-Aufnahmen und die verbundenen
Ordner. Unter „Ordner verbinden“ wählt man einen Ordner (oder fügt seinen Pfad ein); alle Aufnahmen
darin und in seinen Unterordnern stehen dann in der Bibliothek, ohne kopiert zu werden, und neue
kommen von selbst dazu. Liegen neben der EXE Aufnahmen, schlägt die Bibliothek vor, diesen Ordner zu
verbinden. Trennen nimmt einen Ordner nur aus der Bibliothek; Transcribe löscht darin nie etwas.

Das Transkript einer Aufnahme aus einem verbundenen Ordner legt die App außerdem als Datei daneben
(`Interview.mp3.transcribe.json`, mit Korrekturen und Clips), und liest beim Laden, was dort liegt.
So arbeiten mehrere Menschen über einen geteilten Ordner (etwa Nextcloud) an denselben Transkripten.
Abschalten lässt sich das in den **Einstellungen** unter „Ordner“. Das Format steht in
[docs/sidecar.md](docs/sidecar.md).

### Mit Windows starten, im Hintergrund weiterlaufen

In den **Einstellungen** unter „Andere Programme“:

- **Im Hintergrund bereit** (anfangs an): Andere Programme dürfen die Live-API nutzen, und die
  Erkennung bleibt geladen, damit alles sofort startet (etwa 1 GB Arbeitsspeicher). Aus werden
  Programme abgewiesen, und nichts bleibt geladen; von Hand aufnehmen und diktieren geht trotzdem,
  es lädt die Erkennung dann beim Start.

- **Mit Windows starten** trägt die App für den angemeldeten Benutzer in den Autostart ein
  (`HKCU\…\CurrentVersion\Run`, ohne Administratorrechte; im Task-Manager unter „Autostart-Apps“).
  Sie startet dann beim Anmelden ohne Fenster, mit `--background`.
- **Nach dem Schließen weiterlaufen**: Schließt man das Fenster, bleibt die App im
  Infobereich und nimmt weiter Aufträge und Live-Anfragen an. Sonst beendet sie sich, sobald kein
  Auftrag und keine Live-Transkription mehr läuft.

Ein Windows-Dienst ist das bewusst nicht: Dienste laufen ohne angemeldeten Benutzer, ohne Symbol in
der Taskleiste, ohne Fenster und ohne Zugriff aufs Mikrofon.

## Live-Transkription

Die Seite **Live** macht aus Sprache sofort Text: selbst aufnehmen, übers Mikrofon, den Systemton
(was der Rechner abspielt, etwa die anderen in einem Online-Meeting) oder beides gemischt (auf
Wunsch mit gespeicherter Aufnahme, die sich danach vollständig mit Sprechern transkribieren lässt),
oder andere Programme wie Moliri verbinden. Die tragen `ws://localhost:31765/api/live` als Adresse
ein und schicken entweder selbst Ton, oder lassen Transcribe mithören und bekommen den Text laufend
zurück. Jede Anfrage bleibt als Notiz stehen. Das Protokoll steht in
[docs/live-api.md](docs/live-api.md).

Der Schalter **Im Hintergrund bereit** unten auf der Seite (derselbe wie in den Einstellungen) hält
die Erkennung geladen und lässt Programme zu, solange er an ist (so ist es anfangs): Eine Aufnahme
beginnt dann sofort, ohne Ladezeit, und belegt dafür etwa 1 GB Arbeitsspeicher. Aus werden Programme
abgewiesen, und nichts bleibt geladen; von Hand aufnehmen geht immer. Ein gewähltes Mikrofon nimmt
die App unter Windows über WASAPI auf, auch ein Bluetooth-Headset liefert so nach etwa einer Sekunde
Ton. Eine gespeicherte Aufnahme lässt sich neben ihrem Text abspielen; ein Klick auf die Zeit eines
Satzes spielt ab dort.

### Texterkennung für andere Programme

Programme wie Moliri lassen Transcribe auch den Text von Bildern lesen (OCR): `POST
http://localhost:31765/api/ocr` mit dem Bild, zurück kommen Zeilen und Wörter mit ihrer Lage im Bild.
Webseiten brauchen dieselbe Erlaubnis wie für die Live-Transkription (Seite **Live**, „Wer sich
verbinden darf“). Tesseract wird nicht mitgeliefert: Unter Windows lädt die App es beim ersten Lesen
herunter (etwa 6 MB, dazu je Sprache 2–11 MB), unter Linux und macOS liest das installierte
`tesseract`. Als zweite Erkennung liest PP-OCRv5 gedruckte lateinische Schrift genauer, auch auf der
Grafikkarte; seine Modelle (13 MB) kommen ebenfalls erst beim ersten Lesen. Unter Windows liest auf
Wunsch auch die Texterkennung von Windows selbst, mit den dort installierten Sprachen: sofort da und
sehr schnell, aber ohne Fraktur und Handschrift. Mit einem eigenen Schlüssel liest außerdem ein
KI-Dienst (OpenAI, Anthropic, Google Gemini, Mistral oder ein eigener Server wie Ollama), auch
Handschrift; das Bild geht dann an den Dienst. Er liest nur, wenn ein Programm ihn nennt oder er in
den Einstellungen als Vorgabe für Programme gewählt ist, nie von selbst; sein Schlüssel ist ein
anderer als der für die Sprache und liegt ebenfalls nur in `settings.json`. Ausprobieren und
einrichten lässt sich alles unter **Einstellungen → Texterkennung**. Das Protokoll steht in
[docs/ocr-api.md](docs/ocr-api.md).

### Suche nach Bedeutung für Moliri

Moliris Feldauswahl findet Felder auch nach ihrer Bedeutung („Künstler“ findet *Urheber*), mit dem
Sprachmodell EmbeddingGemma. Statt es in jeden Browser zu laden, kann Moliri Transcribe rechnen
lassen: `POST http://localhost:31765/api/embed` mit Texten, zurück kommen ihre Vektoren, dieselben
wie im Browser. Das Modell (295 MB) kommt erst, wenn Moliri oder die Einstellungen danach fragen,
und rechnet auf dem Prozessor, etwa eine Zehntelsekunde je Suchwort; geladen belegt es rund 1,4 GB
Arbeitsspeicher und gibt sie nach einer Viertelstunde ohne Anfrage wieder frei. Webseiten brauchen
dieselbe Erlaubnis wie für die Texterkennung. Das Protokoll steht in
[docs/embed-api.md](docs/embed-api.md).

### Filter in eigenen Worten für Moliri

Moliris Detailsuche macht aus einem Satz wie „Gemälde nach 1900 ohne Standort“ die Bedingungen
eines Filters. Statt das KI-Konto der Instanz zu fragen, kann Moliri Transcribe fragen: Ein
Sprachmodell auf dem Rechner (Qwen 2.5 7B auf dem Server-Programm von llama.cpp) liest den Satz und
den Feldkatalog des Projekts, beides verlässt den Rechner nicht. Modell und Programm (zusammen
4,5 GB) kommen erst, wenn Moliri oder die Einstellungen danach fragen. Auf dem Prozessor eines
Laptops dauert die erste Frage zu einem Projekt knapp zwei Minuten, jede weitere etwa eine halbe bis
eine Minute, weil das Modell den gelesenen Feldkatalog behält, auch über einen Neustart. Mit einer
NVIDIA-Grafikkarte läuft das Modell auf ihr (llama.cpp für CUDA, gut 0,6 GB dazu): auf einer RTX
A3000 im Laptop 14 s für die erste Frage, 2 bis 3 s für jede weitere. Andere Grafikkarten, auch die
in Intel- und AMD-Prozessoren eingebauten, nutzt das Modell über Vulkan, wenn eine Messung beim
ersten Start zeigt, dass sie schneller sind als der Prozessor. Die Frage
läuft im Hintergrund, Moliri zeigt, wie weit sie ist. Geladen belegt das Modell rund 5,5 GB
Arbeitsspeicher und gibt sie nach einer Viertelstunde ohne Frage wieder frei. Webseiten brauchen
dieselbe Erlaubnis wie für die Texterkennung. Das Protokoll steht in
[docs/filter-api.md](docs/filter-api.md).

### Diktieren in jedem Programm

Unter Windows diktiert man mit einem Tastenkürzel in jedes Programm: **Strg + Alt + Leertaste**
drücken und sprechen, dann noch einmal drücken. Transcribe schreibt das Gesagte dorthin, wo der
Cursor steht, und legt es außerdem in die Zwischenablage. **Esc** bricht ab. Ein kleines Fenster
über der Taskleiste zeigt wie ein Untertitel, dass Transcribe zuhört und was es schon verstanden
hat; ein Klick darauf öffnet das Diktat auf der Seite Live. Diktiert wird mit dem Mikrofon und der
Sprache der Seite Live; jedes Diktat steht danach auch dort.

In den **Einstellungen** unter „Diktieren in jedem Programm“ lässt sich ein anderes Kürzel wählen
(Strg + Umschalt + Leertaste, Strg + Alt + D) oder es ausschalten. Zwei Schalter bestimmen, wohin
der Text geht: „Am Cursor einfügen“ und „In die Zwischenablage“. Mit beiden wird über Strg + V
eingefügt; nur mit dem ersten tippt Transcribe den Text ein und lässt die Zwischenablage, wie sie
war; sind beide aus, bleibt das Diktat nur im Fenster und auf der Seite Live, als Notiz oder zum
Kopieren von Hand. Das Kürzel wirkt, solange Transcribe mit seinem Symbol im Infobereich
läuft, also auch im Hintergrund oder nach dem Autostart. In Programme, die als Administrator laufen,
kann Windows nicht einfügen lassen; dort liegt der Text in der Zwischenablage.

`cmd/tx` ist ein Kommandozeilenprogramm, das die Erkennung einmal auf einer Datei laufen lässt, ohne
Server: `go run ./cmd/tx -data <ordner> <datei in ordner/audio>`.

## Bauen

Voraussetzungen sind [Go](https://go.dev/dl/) 1.25 oder neuer und [Node](https://nodejs.org/) mit
[pnpm](https://pnpm.io/) für die Oberfläche. Es braucht keinen C-Compiler; die Erkennungsbibliothek
wird zur Laufzeit geladen.

```powershell
.uild.ps1                     # baut die Oberfläche (frontend/ -> web/dist), dann transcribe.exe
go test ./...                   # Tests
go run ./cmd/transcribe         # direkt starten, mit Konsolenausgabe
```

Auf Linux und macOS baut `go build ./cmd/transcribe` ein Programm für die jeweilige Plattform (vorher
`pnpm build` in `frontend/`).

Ein Tag `v*` baut über die GitHub-Action `.github/workflows/release.yml` Oberfläche, Windows-EXE und
die Programme für Linux und macOS und hängt sie an den Release des Tags. Von Hand gestartet (Actions →
Release → Run workflow, mit einem vorhandenen Tag) baut sie nur die Linux- und macOS-Programme dieses
Tags und legt sie seinem Release bei.

### Die Erkennung als Go-Modul

Die Erkennung selbst (die Engine) liegt in Paketen des Moduls `github.com/moliri-app/transcribe`, die
andere Programme importieren können. Moliris Server-Dienst nimmt sie so, fest auf eine Version
(`go get github.com/moliri-app/transcribe@vX.Y.Z`, dann `go mod vendor`).

| Paket | Inhalt |
|---|---|
| `pipeline` | eine Aufnahme erkennen: Geräusche, Sprecher, Text, Korrekturen (`Runner.Run`); Stellen neu erkennen (`Retranscribe`); Live-Streams (`OpenLive`) |
| `annotate` | die Korrekturen auf das Maschinenergebnis legen (`Apply`) |
| `export` | TXT, VTT, HTML und der Text von Clips |
| `model` | die JSON-Formen (`_machine.json`, `_final.json`, `model.Result`) |
| `models` | Modelle und Laufzeit finden und laden; `Set.Offline` lädt nie nach |
| `sherpa` | sherpa-onnx über purego |
| `audio` | Dekodieren mit ffmpeg |
| `ocr`, `ort` | Texterkennung im Format 1: Tesseract als Programm, PP-OCRv5 über onnxruntime; Testsatz in `ocr/ocrtest` |
| `embed` | Vektoren für die Suche nach Bedeutung: EmbeddingGemma über onnxruntime, mit Gemmas Tokenizer in Go |
| `jobs` | Optionen und Fortschritt eines Auftrags |
| `stream` | was ein Live-Stream liefert, gesprochene Satzzeichen |
| `fileutil`, `proc` | Hilfen |

Diese Pakete hängen von nichts aus `internal/` ab und bauen ohne C-Compiler auch für Linux
(`CGO_ENABLED=0 GOOS=linux go build ./pipeline ./ocr/...`); `TestEngineStandsOnItsOwn` in
`pipeline` prüft beides. Was nur die App braucht (Fenster, Einstellungen, Download von GPU-Laufzeit
und ffmpeg, Mikrofon, Warteschlange, Ablage im Datenordner und dessen Ort, `internal/datadir`), liegt in `internal/`;
`internal/runner` bindet die Engine an den Datenordner und die Einstellungen.

### Oberfläche

Die Oberfläche ist eine SvelteKit-App in `frontend/`. `pnpm build` schreibt sie nach `web/dist`, das
per `go:embed` in die EXE kommt (eingecheckt ist dort nur ein Platzhalter, damit `go build` auch ohne
Frontend-Build geht).

- **Seiten:** Welche Adressen die App ausliefert, steht in `internal/server/pages.go`; die Adressen
  der früheren statischen Seiten (`/editor.html` …) leiten dorthin weiter.
- **Entwickeln:** die EXE oder `go run ./cmd/transcribe` starten, dann in `frontend/` `pnpm dev`;
  Vite reicht `/api`, `/audio` und `/results` an `http://localhost:31765` weiter (anderer Port:
  `TRANSCRIBE_URL`).
- **Moliri-Komponenten** (Knöpfe, Schalter, Reiter, Dialoge, Stile und der Transkript-Editor) liegen
  gespiegelt unter `frontend/src/lib/moliri/` und werden nur in Moliri geändert. `pnpm sync:moliri`
  gleicht sie an, `pnpm sync:check` meldet Abweichungen; beides braucht einen Moliri-Checkout
  (`MOLIRI_SRC`). Woher der Editor seine Daten hat und wohin er speichert, sagt sein Speicher in
  `frontend/src/lib/editor/speicher.js` (die API der EXE); die Seite `/editor` verbindet beides.
- **Lizenzen** der mitgelieferten Pakete, Icons und Schriften zeigt die Seite `/lizenzen`; die
  Liste entsteht bei jedem Build (`pnpm licenses:generate`).

Die Oberfläche prüfen Playwright-Tests in `e2e/` (Node mit pnpm und `ffmpeg` im `PATH`):
`cd e2e; pnpm install; pnpm exec playwright install chromium; pnpm test`. `pnpm start` startet die
App mit den Testdaten zum Ausprobieren. Mehr dazu in [e2e/README.md](e2e/README.md).

## Lizenz

Der Quelltext steht unter der [Mozilla Public License 2.0](LICENSE). Drittkomponenten, Modelle und
Schriften stehen unter eigenen Lizenzen; welche das sind und welche Namensnennungen dazugehören, steht
in [NOTICE](NOTICE).

„Moliri“ und „Transcribe by Moliri“ sind Namen von Moliri. Die MPL gibt keine Rechte an ihnen: Wer
den Code weitergibt oder verändert, darf das gern tun, aber unter eigenem Namen.
