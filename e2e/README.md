# Verhaltensvertrag (Playwright)

Tests gegen die Oberfläche der EXE. Geschrieben gegen die früheren statischen Seiten, laufen sie
unverändert gegen die SvelteKit-Oberfläche, die sie abgelöst hat: Sie greifen nur über Rollen,
zugängliche Namen und sichtbaren Text zu, nie über CSS-Klassen.

## Laufen lassen

Voraussetzungen: Go, Node mit pnpm, `ffmpeg` und `ffprobe` im `PATH`.

```sh
cd e2e
pnpm install
pnpm exec playwright install chromium
pnpm test                      # alle Tests
pnpm test editor-clips         # eine Datei
pnpm start                     # die App mit den Testdaten zum Ausprobieren
pnpm start --api               # dasselbe, dazu ein Nachbau von Erkennungsdiensten (internal/sttapi/sttapitest)
```

`pnpm test` baut zuerst `cmd/transcribe` nach `e2e/.bin/` (die Oberfläche ist eingebettet, vorher
`pnpm build` in `frontend/`). Mit `TRANSCRIBE_EXE=<pfad>` laufen die Tests stattdessen gegen eine
fertige EXE.

Jeder Test startet seine eigene EXE (`--data <kopie> --no-browser --port <frei>`) auf einer frischen
Kopie von `testdata/`; die Tests laufen parallel und sehen nie die Korrekturen eines anderen. Die App
ist dabei offline (Proxy ins Leere): Laufzeit und Modelle liegen als leere Platzhalter im Datenordner,
damit der erste Start nichts herunterlädt. Eine echte Erkennung läuft in den Tests nie; ein
eingereihter Auftrag endet deshalb mit „Fehler“. Chromium läuft mit `--disable-audio-output`: die
Aufnahme spielt mit der richtigen Geschwindigkeit, aber ohne Soundkarte (mit vielen parallelen
Browsern blieb sie sonst hängen).

## Testdaten

`testdata/` ist ein Datenordner, wie die EXE ihn anlegt:

- `audio/Papierfabrik Interview.mp3` (35 s): ein gestelltes Zeitzeugen-Interview, gesprochen von den
  freien Piper-Stimmen Kerstin (Fragen) und Thorsten (Antworten), dazu
  `results/Papierfabrik Interview/_machine.json` und `_final.json` aus einem echten Lauf von `tx`
  (Preset interview, 2 Sprecher). Die Erkennung hörte „Habaga“ statt „Herr Berger“. Die Ergebnisse
  stammen aus einer früheren Fassung derselben Aufnahme; `make-audio.py` spricht sie neu und legt
  jeden Satz auf die Zeiten der Ergebnisse (Anleitung im Skript).
- `Papierfabrik mit Korrekturen`: dieselbe Aufnahme mit vorbereitetem `_annotations.json` (Sprecher
  Interviewerin und Herr Berger, „Habaga“ ersetzt, Clips „Anfang als Lehrling“ und „Filze waschen“).
- `audio/Neue Aufnahme.mp3`: noch nicht transkribiert.
- `results/_queue.json`: ein fertiger und ein gescheiterter Auftrag; `settings.json`: Vorgaben, die
  GPU-Frage ist schon beantwortet.

## Was die Oberfläche bieten muss

Die Namen und Rollen, über die die Tests gehen (alle Texte deutsch):

| Wo | Rolle und Name |
|---|---|
| App-Leiste | `navigation` „Hauptnavigation“ mit Links „Editor“, „Warteschlange“, „Live“, „Modelle vergleichen“, „Einstellungen“; die aktuelle Seite trägt `aria-current="page"`; unter 560 px Breite liegen „Live“, „Modelle vergleichen“ und „Einstellungen“ im `button` „Mehr“ (`aria-expanded`) als `menuitem`s eines `menu` |
| Editor | `combobox` „Aufnahme wählen“ (Optionen: Name ohne Endung), `region` „Transkript“, Wörter als Text, Sprechernamen als `button` |
| Abspielen | `button` „Abspielen …“ / „Anhalten …“, `combobox` „Geschwindigkeit“ (Option „2×“); das gerade gesprochene Wort trägt `aria-current="true"`; Strg+Klick auf ein Wort spielt ab dort |
| Stelle im Text | Klick auf ein Wort zeigt `toolbar` „Stelle im Text“ mit Sprecher, Sprache, „von Hand korrigiert“ und `button` „Ab hier“ / „Anhalten“ |
| Korrigieren | direkt in den Text tippen, Enter übernimmt, Esc verwirft; `status` mit „noch keine Korrekturen“, „Eingabe offen …“, „gespeichert …“, „Eingabe verworfen“; `button` „Rückgängig“ und Strg+Z |
| Seitenleiste | `list` „Sprecher“, „Clips“, „Korrekturen“ mit `listitem`s; Knöpfe „S2 ausblenden“, „alle zeigen“, „Korrektur entfernen“, „<Titel> bearbeiten“, „<Titel> exportieren“, „nur diesen Clip abspielen“ / „anhalten“, Checkboxen „<Titel> auswählen“, „alle auswählen“, „Nacheinander abspielen“, „Zusammenschnitt exportieren“, Schlagworte als `button` |
| Dialoge | `dialog` mit „Name“, `combobox` „Sprache“ (Suchfeld, dessen Vorschläge `option`s mit dem Namen vorn sind, siehe `chooseLanguage`) und „Zusammenlegen“, „Übernehmen“, „Abbrechen“; `button` „Neu berechnen“ (gesperrt, bis Sprache oder Zusammenlegen vorgemerkt ist); für eine Auswahl „Abspielen“, „Als Clip“, `combobox` „Sprecher“ und „Sprache“, „Schließen“; für Clips „Titel“, „Schlagworte“, „Ende“, „Clip anlegen“, „Löschen“, „Audio (MP3)“, „Text mit Titel und Schlagworten (.txt)“ |
| Export | `button` „Transkript exportieren“, dann `link` „Text (.txt)“, „Untertitel (.vtt)“, „HTML …“ |
| Warteschlange | Checkboxen je Aufnahme (Name der Datei), „Voreinstellung“, „Sprecher“, „In die Warteschlange“, „Datei hochladen“, `status`, `list` „Aufträge“ mit „Erneut“, „Entfernen“, Link „im Editor öffnen“ |
| Einstellungen | „Voreinstellung“, „Threads“, `switch` „Sofort mit diesen Vorgaben starten“, `status` „Gespeichert.“, Überschriften „Speicherorte“, „Modelle und Komponenten“, „Programm“ |

Gespeichertes prüfen die Tests in den Dateien des Datenordners (`_annotations.json`, `_final.json`,
`settings.json`); deren Format gehört zur unveränderten API.
