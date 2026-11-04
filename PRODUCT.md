# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

- **Archiv- und Sammlungsteams** in Museen: korrigieren regelmäßig und lange am Stück Transkripte von Audioaufnahmen (Zeitzeugen-Interviews, Rundgänge, Gespräche, Bühnenmitschnitte).
- **Ehrenamtliche**: korrigieren gelegentlich, ohne technisches Vorwissen, oft ältere Menschen.
- **Forschende und Kurator:innen**: lesen, hören, suchen Stellen, zitieren und markieren; korrigieren eher wenig.

Alle arbeiten am Schreibtisch, hören dabei die Aufnahme und lesen mit.

## Product Purpose

Aus Audioaufnahmen im Museumskontext automatisch ein lesbares, zeitgenaues Transkript mit Sprechern, Sprachen und Geräusch-Markern erzeugen und es Menschen leicht machen, es dort zu korrigieren, wo die Maschine scheitert (Mundart, Plattdeutsch, Namen, Fremdsprache, Gesang). Erfolg heißt: Das Ergebnis liest sich für einen Menschen richtig, und jede menschliche Korrektur bleibt erhalten und wird zur Referenz.

## Positioning

Starke Automatik ohne Zutun, gekoppelt an eine Korrekturschicht, die von der Maschine getrennt bleibt: Neu berechnen überschreibt nie menschliche Arbeit, und korrigierte Passagen werden zu echten Referenztranskripten für Bewertung und späteres Training.

## Operating Context

- Go-App „Transcribe by Moliri“ (`cmd/transcribe`, portable Windows-EXE mit eingebetteter Oberfläche); läuft lokal bzw. auf einem CPU-vServer, Oberfläche im Browser bzw. App-Fenster: Bibliothek (Startseite, mit den laufenden Aufträgen), Editor, Live und Einstellungen als SvelteKit-Oberfläche aus `frontend/`.
- Pipeline in Go (`pipeline`, einmalig per `cmd/tx`) mit `--speakers`, `--language`, `--quality` und Presets (auto, interview, gespraech, vortrag, musik), die nur die Sprecherzahl festlegen; die Oberfläche bietet deshalb nur Sprecher und Sprachen an.
- Ergebnisse je Aufnahme in `results/<datei>/`: `_machine.json` (Maschine), `_annotations.json` (Menschen), `_final.json` (zusammengeführt), Versionen in `_annotations_history/`.
- Material: lange Aufnahmen (bis über 75 Minuten), Mundart, Plattdeutsch, mehrsprachige Passagen, Musik und Applaus.

## Capabilities and Constraints

- Öffentlich als freie Software unter der Mozilla Public License 2.0 (Repo `github.com/moliri-app/transcribe`); Drittkomponenten und Modelle mit Lizenz und Namensnennung in `NOTICE`.
- Oberflächensprache Deutsch.
- Korrekturen: Sprecher benennen, zusammenlegen, Sprache pro Sprecher oder Stelle, Text ersetzen und einfügen, automatisch gespeichert, rückgängig machbar.
- Clips: Stellen mit Titel und Schlagworten markieren, einzeln oder nacheinander abspielen, einzeln oder als Zusammenschnitt exportieren (Audio, Video, Text). Das Transkript geht als Text, Untertitel (VTT) oder HTML hinaus.
- Oberfläche mit Build-Schritt: SvelteKit (Svelte 5, `adapter-static`) in `frontend/`, `pnpm build` schreibt nach `web/dist`, das per `go:embed` in die Binärdatei kommt; `build.ps1` baut erst das Frontend, dann Go. Die HTTP-API ist dieselbe wie zur Zeit der statischen Seiten.

## Brand Commitments

Moliri-Look: Name „Transcribe by Moliri“ in App-Leiste und Fenstertitel, Moliri-Tokens (Schrift, Grautöne, Signalgrün, Radien) und, wo sinnvoll, dieselben Komponenten wie Moliri. Kein Haus-Design eines Museums: Das Werkzeug soll für viele Häuser taugen. Wo Moliris Gestaltung kleiner oder kontrastärmer ist als die Pflichten unter „Accessibility & Inclusion“, gewinnen diese.

## Evidence on Hand

Testaufnahmen im lokalen Datenordner, nicht im Repo (Radiobeitrag mit schwäbischen O-Tönen, Kabarett-Mitschnitt, KI-Diskussion, Podcast, Zeitzeugen-Rundgänge Papierfabrik, gelesener Artikel). Im Repo liegt nur das gestellte, synthetisch gesprochene Interview in `e2e/testdata/` für die Oberflächentests. Keine Nutzerstudien, keine echten Referenztranskripte bisher.

## Product Principles

1. Die Aufnahme ist die Wahrheit: Hören und Mitlesen stehen im Zentrum, alles andere unterstützt das.
2. Menschliche Arbeit geht nie verloren und gewinnt immer gegen die Maschine.
3. Unsicherheit zeigen statt verstecken: wo die Maschine zweifelt, soll man es sehen und schnell hinkommen.
4. Für Gelegenheitsnutzer verständlich, für Vielnutzer schnell.

## Accessibility & Inclusion

Ehrenamtliche, oft ältere Nutzer: gut lesbare Schriftgrößen, ausreichender Kontrast, große Klickziele, keine Funktionen nur per Tastenkürzel oder nur per Hover.
