---
name: Transcribe by Moliri
description: Sendemanuskript im Moliri-Look. Papier auf Nebelgrau, Tinte als einzige dunkle Fläche, Signalgrün für das, was gerade läuft, Rotstift für die menschliche Hand.
colors:
  ink: "hsl(210 4% 11%)"
  paper: "hsl(0 0% 98%)"
  mist: "hsl(156 12% 92%)"
  slate: "hsl(190 12% 61%)"
  ink-2: "hsl(195 9% 38%)"
  ink-3: "hsl(195 9% 41%)"
  rule-2: "hsl(190 12% 80%)"
  focus: "hsl(190 100% 35%)"
  signal: "hsl(95 100% 46%)"
  marker: "hsl(93 100% 70%)"
  signal-soft: "hsl(92 100% 95%)"
  signal-text: "hsl(97 100% 22%)"
  red: "hsl(354 76% 40%)"
  red-soft: "hsl(360 76% 95%)"
  red-on-ink: "hsl(358 76% 70%)"
  blue: "hsl(215 91% 45%)"
  blue-soft: "hsl(220 91% 95%)"
  blue-on-ink: "hsl(219 91% 75%)"
  doubt: "hsl(48 95% 35%)"
  doubt-on-ink: "hsl(48 95% 50%)"
  sel: "hsl(190 100% 35% / 0.14)"
  spk0: "#0b6f78"
  spk1: "#6f42c1"
  spk2: "#9a5b00"
  spk3: "#2b7a4b"
  spk4: "#93398a"
  spk5: "#5f7012"
  spk6: "#8a5a3c"
  spk7: "#434b5c"
  lang-en: "#0b6f78"
  lang-fr: "#6f42c1"
  lang-nl: "#b85c00"
  lang-it: "#2b7a4b"
  lang-es: "#8a6d00"
  lang-x: "#5c677d"
typography:
  page-title:
    fontFamily: "'Atkinson Hyperlegible Next Variable', 'Segoe UI', system-ui, sans-serif"
    fontSize: "2rem"
    fontWeight: 700
    lineHeight: 1.2
    letterSpacing: "-0.01em"
  recording-title:
    fontFamily: "'Atkinson Hyperlegible Next Variable', 'Segoe UI', system-ui, sans-serif"
    fontSize: "1.375rem"
    fontWeight: 700
    letterSpacing: "-0.01em"
  manuscript:
    fontFamily: "'Atkinson Hyperlegible Next Variable', 'Segoe UI', system-ui, sans-serif"
    fontSize: "1.25rem"
    fontWeight: 400
    lineHeight: 1.75
  body:
    fontFamily: "'Atkinson Hyperlegible Next Variable', 'Segoe UI', system-ui, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.6
  control:
    fontFamily: "'Atkinson Hyperlegible Next Variable', 'Segoe UI', system-ui, sans-serif"
    fontSize: "1rem"
    fontWeight: 600
    letterSpacing: "0.05em"
  nav:
    fontFamily: "'Atkinson Hyperlegible Next Variable', 'Segoe UI', system-ui, sans-serif"
    fontSize: "0.9375rem"
    fontWeight: 600
  meta:
    fontFamily: "'Atkinson Hyperlegible Next Variable', 'Segoe UI', system-ui, sans-serif"
    fontSize: "0.9375rem"
    fontWeight: 400
  cue:
    fontFamily: "'Atkinson Hyperlegible Next Variable', 'Segoe UI', system-ui, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    letterSpacing: "0.01em"
  label-speaker:
    fontFamily: "'Atkinson Hyperlegible Next Variable', 'Segoe UI', system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 700
    lineHeight: 1.3
    letterSpacing: "0.08em"
  label-section:
    fontFamily: "'Atkinson Hyperlegible Next Variable', 'Segoe UI', system-ui, sans-serif"
    fontSize: "0.8125rem"
    fontWeight: 700
    letterSpacing: "0.08em"
  label-lane:
    fontFamily: "'Atkinson Hyperlegible Next Variable', 'Segoe UI', system-ui, sans-serif"
    fontSize: "13px"
    fontWeight: 600
  time-clock:
    fontFamily: "'Atkinson Hyperlegible Next Variable', 'Segoe UI', system-ui, sans-serif"
    fontSize: "1.0625rem"
    fontWeight: 400
    fontFeature: "tnum"
  time-mark:
    fontFamily: "'Atkinson Hyperlegible Next Variable', 'Segoe UI', system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    fontFeature: "tnum"
rounded:
  word: "3px"
  thumb: "4px"
  field: "0.5rem"
  sheet: "0.75rem"
  tip: "0.875rem"
  pop: "1rem"
  bar: "1.75rem"
  pill: "2rem"
  round: "50%"
spacing:
  hit: "44px"
  xs: "0.25rem"
  sm: "0.5rem"
  md: "0.75rem"
  lg: "1.25rem"
  gutter: "1.75rem"
  margin-col: "10.5rem"
  text-col: "42rem"
  sidebar: "21.25rem"
  sheet-max: "1040px"
components:
  appbar:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.paper}"
    typography: "{typography.nav}"
    height: "60px"
  appbar-tab:
    backgroundColor: "transparent"
    textColor: "{colors.paper}"
    rounded: "{rounded.pill}"
    padding: "0 1rem"
    height: "{spacing.hit}"
  appbar-tab-active:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.pill}"
  badge:
    backgroundColor: "{colors.signal}"
    textColor: "{colors.ink}"
    rounded: "{rounded.pill}"
    height: "1.5rem"
  button-primary:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.paper}"
    typography: "{typography.control}"
    rounded: "{rounded.pill}"
    padding: "0.25rem 1rem"
  button-secondary:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    typography: "{typography.control}"
    rounded: "{rounded.pill}"
    padding: "0.25rem 1rem"
  button-tertiary:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    typography: "{typography.control}"
    rounded: "{rounded.pill}"
  button-positive:
    backgroundColor: "{colors.signal}"
    textColor: "{colors.ink}"
    rounded: "{rounded.pill}"
  button-negative:
    backgroundColor: "{colors.red}"
    textColor: "{colors.paper}"
    rounded: "{rounded.pill}"
  pill-toggle:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.pill}"
    height: "{spacing.hit}"
  field:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.field}"
    padding: "0 0.75rem"
    height: "{spacing.hit}"
  sheet:
    backgroundColor: "{colors.paper}"
    rounded: "{rounded.sheet}"
    padding: "1.5rem 1.75rem 1.75rem"
  status-running:
    backgroundColor: "{colors.signal}"
    textColor: "{colors.ink}"
    rounded: "{rounded.pill}"
  editor-head:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    typography: "{typography.recording-title}"
  player-bar:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.paper}"
    height: "4rem"
  button-play:
    backgroundColor: "{colors.signal}"
    textColor: "{colors.ink}"
    rounded: "{rounded.round}"
    size: "3.25rem"
  manuscript-sheet:
    backgroundColor: "{colors.paper}"
    rounded: "{rounded.sheet}"
    padding: "1rem 2.75rem 3rem"
    width: "{spacing.sheet-max}"
  speaker-name:
    textColor: "{colors.spk0}"
    typography: "{typography.label-speaker}"
  cue-line:
    textColor: "{colors.blue}"
    typography: "{typography.cue}"
  wordbar:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.paper}"
    rounded: "{rounded.bar}"
    padding: "0.3125rem"
  wordbar-play:
    backgroundColor: "{colors.signal}"
    textColor: "{colors.ink}"
    rounded: "{rounded.pill}"
    height: "{spacing.hit}"
  timeline-tip:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.paper}"
    rounded: "{rounded.tip}"
    width: "440px"
  speaker-menu:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.pop}"
    padding: "0.875rem"
    width: "19rem"
  sidebar:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    width: "{spacing.sidebar}"
---

# Design System: Transcribe by Moliri

## Overview

**Creative North Star: "Das Sendemanuskript im Moliri-Look"**

Das Transkript bleibt ein Manuskript, an dem eine Redaktion arbeitet. Es trägt jetzt die Kleider von Moliri: helles Papier (`paper`) liegt auf Nebelgrau (`mist`), Tinte (`ink`) ist die einzige dunkle Fläche und rahmt die Arbeit oben (App-Leiste) und unten (Player-Leiste) ein. Was gerade läuft, ist Signalgrün: der Abspielknopf, der Kopf der Zeitleiste, das gesprochene Wort, ein laufender Auftrag. Darunter liegen unverändert die redaktionellen Bedeutungen: Rotstift für die menschliche Hand, Regie-Blau für Musik und Applaus, Zweifel-Gelb für unsichere Wörter, acht Sprecherfarben.

Die Oberfläche ist dicht, aber nicht eng. Im Editor gehört jede Zeile Höhe dem Text: Titel und Stand stehen auf einer Zeile, leere Spuren der Zeitleiste entfallen, auf niedrigen Bildschirmen rücken die Spuren zusammen, der Player sitzt wie bei einem Musikspieler fest am unteren Rand. Der Lesetext ist groß (20 px, Zeilenhöhe 1,75), denn die Nutzer sind oft ältere Ehrenamtliche und arbeiten lange am Stück. Moliri-Komponenten werden übernommen, wo sie passen; wo Moliri kleiner oder kontrastärmer ist als PRODUCT.md verlangt (Klickziele ab 44 px, Bedienschrift ab 16 px, deutlicher Fokus), gewinnen diese Pflichten.

Abgelehnt bleibt der Standard-Editor mit grauer Fläche, blauem Allzweck-Akzent und Wellenform-Kasten als Mittelpunkt. Die Welle ist eine Spur unter mehreren.

**Wo die Werte stehen:**
- Moliri-Tokens in `frontend/src/lib/moliri/scss/colors.scss`, gespiegelt aus Moliri (nicht hier ändern, sondern in Moliri und dann `pnpm sync:moliri`). Alle Werte sind HSL-Tripel und werden als `hsl(var(--color-…))` benutzt.
- Grundstile, die Schriften (`--font-ui`, `--font-mono`), `--hit`, die Felder und die Anpassungen an Moliri-Komponenten in `frontend/src/app.scss`; die Schriftdateien bindet `frontend/src/routes/+layout.svelte` über Fontsource ein.
- Die redaktionellen Rollen des Editors als Variablen auf `.editor` in `TranscriptEditor.svelte` (gespiegelt aus Moliri nach `frontend/src/lib/moliri/components/files/transcript/editor/`), jede auf eine Moliri-Stufe gelegt. Die Bausteine daneben lesen nur diese Rollen (`var(--red)`, `var(--signal)` …), die Zeitleiste liest sie für den Canvas mit `getComputedStyle`.

**Die früheren Seiten:** Die statischen Seiten unter `web/` mit tintenblauem Kopf und eigenen Farbwerten sind entfernt; ihre Adressen (`/editor.html` …) leiten auf die neuen Seiten weiter. Neue Arbeit richtet sich nur nach dieser Datei.

**Key Characteristics:**
- Papier auf Nebelgrau; Tinte nur als Rahmen (App-Leiste, Player-Leiste) und für schwebende Leisten (Wortleiste, Laufband).
- Pillen: Knöpfe, Tabs, Schalter und Leisten sind voll gerundet (2rem), Abspielknöpfe sind Kreise.
- Signalgrün heißt „läuft“: Abspielen, gesprochenes Wort, aktiver Auftrag, Fortschritt.
- Redaktionsrot im Editor ausschließlich für die menschliche Hand, in Text und Zeitleiste.
- Mitlesen: Gesprochenes wird von Grau zu Tinte, das aktuelle Wort wird in Signalgrün überstrichen.
- Eine Schrift für alles: Atkinson Hyperlegible Next wie in Moliri, mitgeliefert. Zeiten mit Tabellenziffern; die Mono-Schwester nur für Code.
- Sprechernamen in Versalien mit Sperrung als Randspalte, wie in einem Rundfunkmanuskript.
- Nur ein helles Thema. Moliris `.theme-dark` ist gespiegelt, wird aber nirgends gesetzt.

## Colors

Moliris ruhige Papier-Tinte-Nebel-Palette mit einem Signalgrün, darauf vier redaktionelle Farben, die je genau eine Bedeutung tragen. Die Editor-Rollen heißen wie in der Tabelle unten und liegen auf Moliri-Stufen; nur Sprecher- und Sprachfarben sind eigene Hex-Werte.

### Primary
- **Signalgrün** (`signal`, Moliri `green-100`): was gerade läuft. Abspielknopf der Player-Leiste, „Ab hier“ in der Wortleiste, gespielter Teil des Positionsreglers, Punkt am Kopf des Abspielkopfs, Badge der aktiven Aufträge, laufender Auftrag, Fortschrittsbalken, Punkt vor dem gespeicherten Stand, das Wortsignal im Laufband, eingeschalteter Schalter, Fokusring auf Tinte. Immer mit Tinte als Schrift, nie selbst als Schrift auf Papier (1,5:1).
- **Textmarker** (`marker`, `green-50`): das gerade gesprochene Wort im Manuskript und die Textauswahl im Editor (`::selection`).
- **Signal-Hauch** (`signal-soft`, `green-10`): angekreuzte Aufnahme in der Bibliothek, Fläche hinter „fertig“ und „neu“.
- **Signal als Schrift** (`signal-text`, `green-300`, als `--color-text-success`): „fertig“ und andere Erfolgsmeldungen in Schrift auf Papier.

### Secondary
- **Redaktionsrot** (`red`, `red-200`): im Editor die menschliche Hand und nichts sonst. Von Hand korrigierte Wörter (Farbe plus 2 px Unterstreichung), von Hand gesetzte Zeitanker, die Rotstift-Spur und die Korrekturmarken der Übersicht, die rote Umrandung einer von Hand festgelegten Sprache, „von Hand“ in der Wortleiste. Außerhalb des Editors ist dieselbe Stufe Moliris Fehlerfarbe (`--color-text-error`, negativer Knopf, gescheiterter Download).
- **Rotstift-Hauch** (`red-soft`, `red-10`): Fläche der von Hand gesetzten Bereiche in der Rotstift-Spur; außerhalb des Editors die Fehler-Pille.
- **Rot auf Tinte** (`red-on-ink`, `red-50`): Rotstift in Wortleiste und Laufband.

### Clips
- **Clip-Tinte** (`--clip`, Tinte, mit `--clip-ink` in Papier): Clips sind Schnitte, die eine Redaktion über eine Passage zieht, und tragen deshalb keine eigene Farbe, sondern Tinte: die Klammer über den Wörtern, die Titelpille im Text, die Balken der Clip-Spur und ihr Streifen in der Übersicht, „Clip anlegen“, gewählte Schlagworte, „Zusammenschnitt exportieren“. Ein Clip, der gerade läuft, zeigt Signalgrün (Schere in der Titelpille, Abspielknopf in der Seitenspalte, Umrandung in der Clip-Spur); der zuletzt gezeigte ist in der Spur in Archivcyan umrandet.

### Tertiary
- **Regie-Blau** (`blue`, `blue-200`): Musik, Applaus, Gesang. Regiezeilen, Geräuschwörter im Text, Balken der Regie-Spur, Unterstreichung „neu erkannt“ und deren Balken in der Zweifel-Spur. Kein Bedienakzent.
- **Regie-Hauch** (`blue-soft`, `blue-10`): Hover einer Regiezeile.
- **Blau auf Tinte** (`blue-on-ink`, `blue-50`): Regie und „neu erkannt“ in der Wortleiste.
- **Zweifel-Gelb** (`doubt`, `yellow-200`): ein dunkles Ocker-Gelb, nur für Wörter, bei denen die Maschine unsicher ist: gepunktete Unterstreichung (3,1:1 auf Papier), Balken der Zweifel-Spur mit Deckkraft nach Konfidenz. Nie Schrift auf Papier.
- **Zweifel auf Tinte** (`doubt-on-ink`, `yellow-100`): Sicherheit unter 50 % in der Wortleiste.
- **Sprecherfarben** (`spk0` bis `spk7`): Name in der Randspalte, Balken der Sprecher-Spur, Streifen in der Übersicht, Punkt in Seitenspalte, Player, Wortleiste und Sprechermenü. Kräftig, als Schrift auf Papier und unter weißer Schrift lesbar, fern von Rot und Blau. Bewusst eigene Werte, nicht Moliris Farbkreis. Einmal für alle Seiten in `app.scss` angelegt; die Blätter der Bibliothek nutzen dieselben.
- **Sprachfarben** (`lang-en`, `lang-fr`, `lang-nl`, `lang-it`, `lang-es`, `lang-x`): Balken der Sprache-Spur für Fremdsprachen. Deutsch erscheint neutral (`mist` als Standard, `ink-3` wenn erkannt oder festgelegt).

### Neutral
- **Tinte** (`ink`, `neutral-100`, als `--color-text-primary` und `--color-bg-inverse`): Text, gesprochene Wörter, App-Leiste, Player-Leiste, Wortleiste, Laufband, primärer Knopf, Abspielkopf.
- **Papier** (`paper`, `neutral-0`): Blätter, Kopf des Editors, Zeitleiste, Seitenspalte, Menüs; Schrift auf Tinte.
- **Nebelgrau** (`mist`, `neutral-10`): Seitenfläche (`--page-bg`), Hover-Fläche, Haarlinien (`--rule`), ausgeblendete Stellen im Manuskript, Grund der Übersicht. Auf Tinte gedämpfte Schrift (`--head-ink-2`, 14,6:1).
- **Schiefer** (`slate`, `neutral-50`): Feldrahmen (`--color-border-secondary`), noch nicht gespielte Welle. Nie als Schrift auf Papier (2,9:1).
- **Tinte 2** (`ink-2`, `--color-text-secondary`): Sekundärtext, Spur- und Abschnittsbeschriftungen; im Editor zugleich die noch nicht gesprochenen Wörter (`--future`) und „ergänzt“ (`--fill`).
- **Tinte 3** (`ink-3`, `--color-text-tertiary`): Zeitmarken, Sprechdauern, Sprachkürzel, Maschinenanker, Hinweise.
- **Linie 2** (`rule-2`): Rahmen der Pillen im Sprechermenü, gestrichelter Rand eingeklappter Stellen, Bildlaufleisten.
- **Archivcyan** (`focus`, `cyan-100`, als `--color-border-focus`): Fokusring auf Papier; in der Zeitleiste der Rahmen des überfahrenen Bereichs. Auswahl-Hauch (`sel`, 14 %) für überfahrene Bereiche und markierte Wörter.

### Named Rules
**The Rotstift Rule.** Im Editor bedeutet Rot „ein Mensch hat das getan“. Kein Fehlerzustand, keine Warnung, keine Hervorhebung, keine Dekoration; Fehlermeldungen des Editors stehen in Tinte. Jedes Wort wird einzeln eingefärbt, nicht der ganze Absatz. Die menschliche Änderung erscheint in Text und Zeitleiste zugleich.

**The Signal Rule.** Signalgrün heißt „läuft gerade“ oder „ist aktiv“. Es ist eine Fläche mit Tinte darauf (oder Schrift auf Tinte), nie Schrift auf Papier; dort steht `signal-text`.

**The Gewissheit-gegen-Zweifel Rule.** Rotstift und Zweifel sind Gegenteile: Rot heißt „ein Mensch hat es festgelegt“, Gelb heißt „die Maschine ist unsicher“. Sie liegen deshalb weit auseinander im Farbkreis (Rot 354°, Gelb 48°) und unterscheiden sich zusätzlich in der Linie (durchgezogen gegen gepunktet). Kein Orange, Braun oder anderer Ton dazwischen trägt eine der beiden Bedeutungen.

**The Freihalte Rule.** Sprecher- und Sprachfarben halten Abstand zu Rot, Regie-Blau, Signalgrün und Zweifel-Gelb. Heute liegt `lang-es` (#8a6d00) nah am Zweifel-Gelb; es steht nur als Balken in der Sprache-Spur. „Ergänzt“ ist neutrale Tinte und wird an Kursive und Klammern erkannt, nie an einer Farbe, die als Sprecher gelesen werden könnte.

**The Moliri-Stufen Rule.** Neue Farben im Editor werden als Rolle auf `.editor` angelegt und auf eine Moliri-Stufe gelegt (`hsl(var(--color-…))`). Ein Hex-Wert in einer Komponente ist ein Fehler; Ausnahme sind nur Sprecher- und Sprachfarben.

## Typography

**Display Font:** Atkinson Hyperlegible Next (`--font-ui`, mit Segoe UI, system-ui)
**Body Font:** Atkinson Hyperlegible Next (`--font-ui`)
**Label/Mono Font:** Atkinson Hyperlegible Mono (`--font-mono`), nur für Code; Zeiten in Atkinson Next mit Tabellenziffern

**Character:** Moliris Bedienschrift, gebaut für Lesbarkeit bei schwacher Sehkraft: I, l und 1 sowie O und 0 (mit Schrägstrich) sind klar unterscheidbar. Sie trägt Lesetext und Bedienung gleichermaßen und kommt als variable Schrift mit (`@fontsource-variable/atkinson-hyperlegible-next`, aufrecht und kursiv; die Mono aus `@fontsource-variable/atkinson-hyperlegible-mono`). Die CSS-Familien heißen „… Variable“. Hierarchie entsteht aus Größe und Gewicht, Benennung aus Versalien mit Sperrung.

### Hierarchy
- **Seitentitel** (700, 2rem, 1,2, Sperrung −0,01em): Überschrift einfacher Seiten (Bibliothek, Einstellungen).
- **Aufnahmetitel** (700, 1,375rem, −0,01em; unter 720 px 1,125rem): Kopf des Editors, zugleich Auswahlfeld der Aufnahme.
- **Manuskript** (400, 1,25rem, Zeilenhöhe 1,75; unter 720 px 1,1875rem): der Transkripttext, Spalte höchstens 42rem, `text-wrap: pretty`.
- **Body** (400, 1rem, 1,6): Grundschrift der Oberfläche. Abschnittsüberschriften `h2` 700, 1,25rem.
- **Bedienung** (600, 1rem, Sperrung 0,05em): Moliri-Knöpfe in Größe `large`; Pillen des Editors 600 bis 700 ohne Sperrung.
- **Navigation** (600, 0,9375rem): Tabs der App-Leiste. Wortmarke 700, 1,0625rem; „by Moliri“ 500 in Nebelgrau.
- **Meta** (400, 0,9375rem, `ink-2`, Werte 600 in Tinte mit Tabellenziffern): Stand unter dem Titel, Wortleiste, Hinweise.
- **Regie** (400 kursiv, 1rem, Sperrung 0,01em, Regie-Blau): Regiezeilen; die Gattung davor 700, 0,8125rem, Versalien, Sperrung 0,1em, aufrecht.
- **Sprecher-Label** (700, 0,875rem, 1,3, Sperrung 0,08em, Versalien): Sprechername in der Randspalte in Sprecherfarbe, Sprecher im Player. Das Sprachkürzel daneben 500, Sperrung 0,02em, in `ink-3`, nicht versal.
- **Abschnitts-Label** (700, 0,8125rem, Sperrung 0,08em, Versalien, `ink-2`): Abschnitte der Seitenspalte (Sprecher, Zeichen im Text), aufklappbarer Kopf der Seitenspalte.
- **Spur-Label** (600, 13 px, Versalien, `ink-2`, auf Canvas): Spurnamen der Zeitleiste und „ÜBERSICHT“; Zeitlineal 500, 13 px; Namen auf Balken 600, 12,5 px in Weiß.
- **Uhr** (400, 1,0625rem, Tabellenziffern): Abspielzeit im Player, die laufende Zeit 600 in Papier, die Länge in Nebelgrau.
- **Zeitmarke** (400, 0,875rem, Tabellenziffern, `ink-3`): Zeit unter dem Sprechernamen, Sprechdauern, Dauer im Sprechermenü.

### Named Rules
**The Versalien Rule.** Versalien mit Sperrung benennen Dinge: Sprecher, Spuren, Abschnitte, Regie-Gattungen. Sie stehen nie als Vorspann über einer Überschrift.

**The Ziffern-Heißt-Zeit Rule.** Zeiten, Dauern und Zähler stehen mit `font-variant-numeric: tabular-nums`, damit sie beim Abspielen nicht springen. Atkinson Next hat Tabellenziffern; die Mono steht nicht bei Zeiten, sondern nur bei Code (Lizenztexte, Codebeispiele).

**The Ein-Name Rule.** Schriften kommen nur über `var(--font-ui)` und `var(--font-mono)`, nie über einen Familiennamen in einer Komponente. So wechselt die Schrift an einer Stelle, wie in Moliri.

**The 16-px Rule.** Lesetext und Bedienung sind mindestens 16 px groß. Darunter stehen nur Beschriftungen: Versalien-Labels, Zeitmarken, Spurnamen (13 bis 15 px).

## Layout

Jede Seite liegt unter der App-Leiste (60 px, sticky). Editor und Live füllen die Höhe darunter (siehe Komponenten). Einfache Seiten (Bibliothek, Einstellungen) sind eine zentrierte Spalte von höchstens 76rem mit 2rem/1,5rem Innenabstand, darin Blätter im Abstand von 1,5rem. Die Einstellungen teilen sie in ein Menü (14rem, klebt beim Scrollen) und die Abschnitte (höchstens 56rem); unter 900 px wird das Menü eine seitlich scrollende Leiste oben.

Der Editor füllt die Höhe darunter (`100dvh − 60px`) als Raster in vier Zeilen: Kopf, Zeitleiste, Arbeitsfläche, Player-Leiste. Die Arbeitsfläche teilt sich in Manuskript (flexibel, scrollt) und Seitenspalte (21,25rem). Die Zeitleiste besteht aus Übersicht (34 px, mit Video-Vorschau 58 px) und Spuren mit einer Beschriftungsspalte von 108 px, in deren Zeitlineal-Zeile (26 px) die Zoomtasten sitzen. Spurreihenfolge und Grundhöhen: Welle 70, Text 26, Sprecher 30, Sprache 24, Regie 22, Zweifel 12, Rotstift 18, Clips 22, Abstand 6 px. Regie, Zweifel, Rotstift und Clips entfallen, solange sie leer sind. Bis 920 px Fensterhöhe schrumpfen die Spuren auf 85 %, bis 820 px Höhe oder 720 px Breite auf 70 %, jede aber nicht unter 16 px.

Das Manuskriptblatt ist höchstens 1040 px breit und zentriert (Innenabstand 1rem/2,75rem/3rem, auf dem Tisch 0,75rem/2rem). Jeder Absatz ist ein Zweispalter: Randspalte 10,5rem rechtsbündig (Name, Zeitmarke, gegebenenfalls Video-Bildchen), Abstand 1,75rem, Text bis 42rem. Ein Sprecherwechsel trennt eine Haarlinie in Nebelgrau mit 0,75rem Luft darüber und darunter; Fortsetzungen desselben Sprechers rücken auf 0,5rem heran, Regiezeilen stehen eng. Unter dem Text liegt ein Abstandhalter von 40vh (unter 1180 px 30vh), damit die letzten Zeilen auf Lesehöhe scrollen.

Responsiv: Unter 1180 px wird die Seitenspalte ein aufklappbarer Block unter dem Manuskript (höchstens 42vh). Unter 720 px steht die Randspalte als Zeile über dem Text, das Blatt hat 1,125rem/1rem Innenabstand, Video-Bildchen entfallen, die Player-Leiste bricht um, Bedienung und Mitlaufen zeigen nur ihr Symbol, die App-Leiste nur das Wellensymbol (ab 760 px).

## Elevation & Depth

Weitgehend flach; Tiefe kommt aus dem Papierbild und aus den beiden Tintenrahmen. Blätter liegen mit einem kaum sichtbaren, weiten Schatten auf Nebelgrau. Die Zeitleiste wirft einen weichen Schatten nach unten, weil das Manuskript darunter hindurchscrollt. Schwebende Teile heben sich deutlich ab, auf Papier hell (Menü, Hilfe), als Tinte dunkel (Wortleiste, Laufband).

### Shadow Vocabulary
- **Blatt** (`box-shadow: 0 1px 2px hsl(var(--color-neutral-100) / 0.05), 0 12px 32px -20px hsl(var(--color-neutral-100) / 0.2)`): Manuskriptblatt; auf einfachen Seiten mit 0,25 im zweiten Teil.
- **Zeitleistenkante** (`box-shadow: 0 6px 16px -12px hsl(var(--color-neutral-100) / 0.2)`): Unterkante der Zeitleiste über dem scrollenden Text.
- **Schwebend hell** (`box-shadow: 0 1px 2px hsl(var(--color-neutral-100) / 0.08), 0 14px 30px -10px hsl(var(--color-neutral-100) / 0.3)`): Sprechermenü, Hilfe.
- **Schwebend Tinte** (`box-shadow: 0 10px 28px -8px hsl(var(--color-neutral-100) / 0.45)`): Wortleiste, Laufband der Zeitleiste.
- **Moliri-Dialog** (`box-shadow: 0 25px 50px rgba(27, 28, 29, 0.25)`): Modal der gespiegelten Komponente, über einer Tinten-Ablage.

### Named Rules
**The Blatt-auf-Nebel Rule.** Ruhende Flächen sind flach oder liegen als Papier auf; Schatten sind weich und weit, nie hart versetzt.

## Shapes

Weich gerundet, mit Moliris Pille als Grundform. Alles, was man drückt, ist eine Pille (2rem) oder ein Kreis: Moliri-Knöpfe, Tabs, Schalter, Tempo-Auswahl, Zeitmarken, Filter-Chip, Pillen im Sprechermenü; Abspielknöpfe, 5-Sekunden-Sprünge und die Schalter der Seitenspalte sind Kreise. Die Wortleiste ist eine Tintenpille (1,75rem) mit einem 12-px-Zipfel zum Wort. Blätter haben 0,75rem, Menü und Hilfe 1rem, das Laufband 0,875rem, Felder 0,5rem, Wörter 3 px, Video-Bildchen 4 px. Balken in der Zeitleiste sind gerundet, schmale Marken (Rotstift, Anker) mit 1 bis 2 px.

Linien tragen Bedeutung: durchgezogen (Mensch, neu erkannt), gepunktet (Zweifel), gestrichelt (eingeklappt, also da, aber verborgen).

**The Zeitanker Rule.** Ein Zeitanker ist überall dasselbe Fähnchen (Mast 1,6 px, Wimpel nach rechts, 8×13 px): im Text als CSS-Maske, die keinen Platz nimmt, in der Text-Spur der Zeitleiste als Canvas-Zeichnung, in der Legende der Seitenspalte. Grau (`ink-3`, 40 %) für Maschinenanker, rot für von Hand gesetzte. Andere senkrechte Striche im Text dürfen nicht wie das Fähnchen aussehen.

## Components

### Buttons
Moliris Knopf (`$lib/moliri/components/ui/Button.svelte`): Pille mit 4 px Rahmen, 600 mit Sperrung 0,05em. Seiten nutzen Größe `large`, die `app.scss` auf 48 px Höhe bringt.
- **Shape:** voll gerundet (`{rounded.pill}`).
- **Primary:** Tinte mit Papierschrift; Hover Tinte um ein Fünftel zum Papier gemischt.
- **Secondary:** Tintenrahmen, transparent; Hover gefüllt.
- **Tertiary:** ohne Rahmen; Hover Nebelgrau.
- **Positive:** Signalgrün mit Tinte. **Negative:** `red-200` mit Papierschrift (Löschen, Abbrechen außerhalb des Editors). **Inverse:** für Tintengrund.
- **Disabled:** Deckkraft 0,5. **Loading:** Spinner über dem Knopf, Beschriftung behält ihren Platz.
- **Focus:** 2 px Ring in Archivcyan mit 2 px Abstand; auf Tinte in Signalgrün.
- **Eigene Pillen:** wo der Moliri-Knopf nicht passt (Datei-Eingabe zum Hochladen, Bedienung im Editor-Kopf), ahmen sie ihn nach: `--hit` hoch, 2 bis 4 px Tintenrahmen, Hover invertiert.

### Chips
- **Status-Pille** (Bibliothek): 999px, 700, 0,875rem. Wartet Nebelgrau mit `ink-2`, läuft Signalgrün mit Tinte, fertig Signal-Hauch mit `signal-text`, Fehler `red-10` mit `red-200`, bricht ab Orange-Hauch mit Moliris Warnorange (`--color-text-warning`).
- **Badge:** Signalgrün mit Tinte, 1,5rem hoch, Tabellenziffern; in der App-Leiste die Zahl aktiver Aufträge.
- **Filter-Chip** (Player-Leiste): Signalgrün 14 % auf Tinte, Schrift Signalgrün, mit „alle zeigen“ als Pille darin. Erklärt, warum das Abspielen springt.
- **Pillen-Schalter:** `aria-pressed`, gedrückt Papier auf Tinte (Mitlaufen) oder Tinte auf Papier mit Signal-Symbol (Auge und Zielscheibe der Seitenspalte).

### Cards / Containers
- **Blatt:** Papier, 0,75rem, Blatt-Schatten, Innenabstand 1,5rem/1,75rem; trägt Formulare, Listen und das Manuskript.
- **Sprechermenü:** Papier, 1rem, 19rem breit, Schwebend-hell-Schatten. Kopf mit Punkt, Name 1,0625rem in Sprecherfarbe und Sprechdauer; darunter Pillen mit 1 px `rule-2`-Rahmen („Nur diesen zeigen“, „Ausblenden“), Hover Nebelgrau. Öffnet sich an einem Sprechernamen in Text oder Seitenspalte, schließt mit Esc, Klick daneben oder nach der Wahl.
- **Hilfe:** Papier, 1rem, 26rem breit, rechts oben fest über dem Editor, Begriffe in einem 7,5rem-Raster.
- **Moliri-Modal:** Papier, 12 px, über einer Tinten-Ablage; für Rückfragen der Einrichtung.

### Inputs / Fields
- **Feld** (`.feld` in `app.scss`): echtes `<input>` oder `<select>`, Papier, 1 px Schieferrahmen, 0,5rem, 44 px hoch; Hover Rahmen in `ink-2`; Auswahlfelder mit Tintenpfeil.
- **Beschriftung** (`.feldname`): 600, 0,9375rem, `ink-2`, über dem Feld.
- **Schalter:** Moliris Toggle als ganze Zeile auf Nebelgrau, in `app.scss` auf mindestens 3rem Höhe, Titel 1rem und Beschreibung 0,9375rem gebracht (Moliri hat 14 und 12 px). Aus: Schiefer, beim Überfahren Tinte; an: Signalgrün.
- **Kontrollkästchen, Optionsfelder, Schieber:** Browserdarstellung in Tinte (`accent-color`), 1,25rem.
- **Aufnahmetitel:** ein randloses `<select>` in Titelgröße mit eigenem Pfeil, Hover Nebelgrau.

### Navigation
- **App-Leiste:** 60 px, Tinte, sticky. Marke links (vier Wellenbalken in Papier, der zweite in Signalgrün; „Transcribe“ 700, „by Moliri“ 500 in Nebelgrau). Tabs als Pillen, 600 / 0,9375rem in Papier, Hover Tinte mit 18 % Papier, aktiv Papier-Pille mit Tinte. „Einstellungen“ rechts abgesetzt. Fokus Signalgrün, nach innen versetzt. Jeder Eintrag lädt die Seite vom Server neu; so speichert der Editor beim Verlassen, was noch aussteht.

### Editor-Kopf
Papierband mit Haarlinie darunter. Links der Aufnahmetitel als Auswahl, auf derselben Zeile der Stand (Länge, Sprecher, Erkennung, Korrekturen) und „alle Korrekturen gespeichert“ mit einem Signalpunkt. Rechts die Pille „Bedienung“ (2 px Tintenrahmen, Hover und offen invertiert), die die Hilfe öffnet.

### Player-Leiste (Signatur)
Feste Tintenleiste am unteren Rand, wie bei einem Musikspieler.
- **Position:** Schieberegler über die ganze Breite, 6 px (Hover und Fokus 10 px), gespielt Signalgrün, der Rest Papier 16 %; Griff 14 px Signalgrün mit Tintenrand, erst beim Überfahren sichtbar.
- **Transport:** 5 Sekunden zurück und vor als 44-px-Kreise (Hover Papier 10 %), dazwischen der Abspielknopf: 3,25rem-Kreis in Signalgrün mit Tintensymbol, Hover vergrößert auf 1,05.
- **Jetzt:** Uhr, darunter wer spricht (Punkt in Sprecherfarbe mit Papierring, Name als Sprecher-Label in Papier).
- **Rechts:** Tempo als Pille mit 1 px Papier-22-%-Rahmen und eingebetteter Auswahl (0,75× bis 2×), Mitlaufen als Pillen-Schalter.
- **Tasten:** Leertaste spielt und hält an, Pfeile springen 5 s, jede Funktion hat auch einen Knopf.

### Manuskript (Signatur)
Absätze auf dem Blatt, Randspalte mit Sprechername (Sprecher-Label in Sprecherfarbe, daneben gegebenenfalls die Sprache) und Zeitmarke als Pille, die den Absatz abspielt; rechts der Text.
- **Mitlesen:** Wörter stehen ungesprochen in `ink-2` und werden beim Abspielen zu Tinte. Das aktuelle Wort bekommt den Textmarker: ein Signalgrün-Band (56 bis 94 % der Zeilenhöhe), das in der Dauer des Worts linear von links darüber läuft. Bei `prefers-reduced-motion` ohne Übergang.
- **Wortzustände:** unsicher gepunktet in Zweifel-Gelb (2 px), neu erkannt durchgezogen in Regie-Blau (1,5 px), ergänzt kursiv in `ink-2` in Klammern, von Hand korrigiert rot mit 2 px Unterstreichung, markiert mit Auswahl-Hauch, Hover Nebelgrau.
- **Regiezeile:** eigene, eng angeschlossene Zeile: Gattung in Versalien, danach Zeitspanne kursiv in Regie-Blau; Hover Regie-Hauch, Klick spielt ab. Geräusche im Fließtext als kleinere blaue Kursive.
- **Ausgeblendete Sprecher:** ihre Stellen klappen zu einer Zeile zusammen (Name, gestrichelte Pille auf Nebelgrau mit Zeitspanne und Anzahl); aufgeklappt liegen die Absätze auf Nebelgrau statt auf dem Blatt, ohne Transparenz.
- **Klicks:** Ein Klick auf ein Wort öffnet die Wortleiste, Strg+Klick spielt sofort, eine Textauswahl zum Kopieren bleibt in Ruhe.

### Clips
Eine Stelle mit Titel und Schlagworten, gespeichert mit den Korrekturen, ohne den Text zu ändern.
- **Im Text:** über den Wörtern des Clips eine 2-px-Klammer in Tinte, durch ein Polster von 0,3em über die Buchstaben gehoben (verschiebt keine Zeile); vor dem ersten Wort der Titel als Tintenpille (1,625rem, 600, 0,875rem, Schere davor), die den Clip öffnet. Die Pille gehört nicht zum Text: Tippen und Korrekturen gehen an ihr vorbei.
- **Anlegen:** „Als Clip“ im Dialog eines markierten Bereichs. Der Clip-Dialog (Papier, 1rem, 23rem, Schwebend-hell) zeigt Zeit und Dauer, den Wortlaut mit derselben Klammer, Titel (sonst die ersten sechs Wörter), Schlagworte mit den vorhandenen als Pillen-Schaltern, Anfang und Ende als Zeit (1:02,5). Falsche Zeiten melden sich in Tinte. „Clip anlegen“ und „Übernehmen“ sind Tintenpillen; bei einem bestehenden Clip stehen oben „Abspielen“ und „Exportieren“, unten links „Löschen“.
- **Export:** derselbe Dialog mit den Formaten als 3rem hohen Zeilen (Audio, bei Videos Video, Text mit Titel und Schlagworten); Hover in Tinte.
- **Transkript-Export:** „Exportieren“ im Editor-Kopf, eine Pille wie „Bedienung“, öffnet eine Liste in Papier (Text, Untertitel, HTML, je mit einer Zeile, wofür).

### Wortleiste (Signatur)
Tintenpille über dem angeklickten Wort (darunter, wenn oben kein Platz ist), in den Grenzen des Manuskripts, mit Zipfel zum Wort. Vorn „Ab hier“ als Signalgrün-Pille mit Tinte (spielt ab dem Wort, beim eigenen Abspielen „Anhalten“), dann Abschnitte mit 1 px Trennlinie in Papier 14 %: Zeit, Sprecher mit Punkt, Sprache, Sicherheit, „von Hand korrigiert“, „ergänzt“, „neu erkannt“, Hintergrundmusik, Zeitanker. Werte 600 in Papier, Bedeutungen in den Farben auf Tinte (Rot, Gelb, Blau); ein Clip an der Stelle ist ein Abschnitt mit Schere, der ihn öffnet. Schließt mit Esc oder Klick daneben; bleibt beim Scrollen über dem Wort.

### Zeitleiste (Signatur)
Canvas auf Papier, Spurnamen links in der Beschriftungsspalte. Welle in Schiefer, gespielt in Tinte. Text-Spur: nah herangezoomt der Text zwischen den Ankern, jedes Wort einzeln gefärbt (ungesprochen `ink-2`, gesprochen Tinte, korrigiert rot), mit Ausblendung am Überlauf. Sprecher als Balken in Sprecherfarbe mit weißem Namen (ausgeblendet 20 % mit `ink-3`). Sprache: Deutsch als Standard in Nebelgrau, erkannt oder festgelegt in `ink-3`, Fremdsprachen in Sprachfarbe, von Hand festgelegt rot umrandet. Regie in Blau (Musik und Gesang 85 %). Zweifel als gelbe Balken nach Konfidenz, „neu erkannt“ blau, „ergänzt“ als umrandeter Balken in `ink-2`. Rotstift als Rotstift-Hauch-Bereiche mit roter Umrandung und Beschriftung, Korrekturen als rote Marken.
- **Abspielkopf:** 2-px-Tintenlinie mit Tintenpunkt, darin Signalgrün.
- **Überfahrener Bereich:** Auswahl-Hauch mit 2 px Archivcyan-Rahmen.
- **Übersicht:** die ganze Aufnahme auf Nebelgrau (mit Video als Bildstreifen unter Tinte 28 %), Welle, 4-px-Sprecherstreifen unten, rote Korrekturmarken oben, der sichtbare Ausschnitt als Fenster. Ziehen verschiebt, Doppelklick springt.
- **Zoom:** Tasten 32×26 px als Pillen im Zeitlineal (Hover Nebelgrau), dazu Mausrad, Umschalt+Rad verschiebt, `+`/`−`.
- **Laufband:** Tintenschild (440 px, 0,875rem) unter der Maus, Kopfzeile mit Zeit, Sprecher, Sprache, Regie in Nebelgrau; darunter ein durchlaufender Wortstreifen (1rem) mit weichen Rändern, Mittellinie und aktuellem Wort in Signalgrün, korrigierte Wörter in Rot auf Tinte.

### Seitenspalte
Papier mit Haarlinie links, 1,375rem Innenabstand. Oben das Video (klebt beim Scrollen, bei Audio unsichtbar). Darunter „Sprecher“ als Objektschilder (siehe unten), dann „Clips“: Schlagworte als Filter-Pillen (gewählt in Tinte), je Clip ein Häkchen, der Titel mit der Klammer darüber (springt zum Clip), Zeit und Dauer, Schlagworte als Nebel-Marken und drei 44-px-Kreise (nur ihn abspielen, exportieren, bearbeiten); ab zwei Clips eine Nebel-Fläche mit „alle auswählen“, „Nacheinander abspielen“ und „Zusammenschnitt exportieren“ als Tintenpille. Danach die Korrekturen. „Sprecher“ als Objektschilder: 0,75rem-Punkt in Sprecherfarbe, Name 600 / 1rem (öffnet das Sprechermenü), Sprechdauer als Zeitmarke, zwei 44-px-Kreisschalter (Zielscheibe: nur diesen zeigen, Auge: ausblenden), Haarlinie darunter. Ausgeblendet: Punkt hohl, Name durchgestrichen in `ink-3`. Am Ende „Zeichen im Text“ als Legende der Linienarten, des Ankers und des Textmarkers.

### Live
Füllt die Höhe unter der App-Leiste wie der Editor, als Raster aus Seitenleiste (22rem), Hauptfläche und Aufnahmeleiste. Unter 900 px eine Spalte: Liste oder Aufnahme, mit „Alle Aufnahmen“ zurück.
- **Seitenleiste:** Papier mit Haarlinie rechts. Oben das Suchfeld (unscharf mit fuzzysort, Wort für Wort über alle Sätze, Namen und Arten; Strg+F oder „/“ springt hinein, Esc leert), darunter die Herkunft als Umschalter in einer Nebel-Pille (Alle, Von Hand, Per API, je mit Zahl; gewählt in Tinte), dann Anzahl und Sortierung (randlose Auswahl; beim Suchen „Beste Treffer“). Die Liste gruppiert nach Tagen unter Abschnitts-Labels, die beim Scrollen kleben. Ein Eintrag: Herkunft mit Symbol (Hand für von Hand, Kabel für per API) und Name, Uhrzeit rechts, zwei Zeilen Vorschau (beim Suchen der getroffene Satz, Treffer im Textmarker), darunter Dauer, Quelle und ein Speichern-Symbol. Gewählt im Signal-Hauch mit feinem Signal-Rand. Unten „Programme verbinden“ als ganze Zeile, mit Signal-Badge, wenn Webseiten abgewiesen wurden.
- **Aufnahme:** Kopf wie der Editor-Kopf (Papierband, Haarlinie): Tag und Uhrzeit als Aufnahmetitel, Stand (Herkunft, Länge, Quelle, Sprache, beim Suchen die Zahl der Sätze mit Treffer), rechts Pillen wie „Bedienung“ (Vollständig transkribieren als Tintenpille, Text kopieren, Teilen, Löschen ohne Rand). Darunter das Blatt auf Nebel: Satz für Satz im Zweispalter, Randspalte 7,5rem mit Zeitmarke und beim Quellenwechsel der Quelle als Sprecher-Label (Mikrofon `spk0`, Systemaudio `spk1`), Text in Manuskriptgröße. Der Satz, der gerade gesprochen wird, steht in `ink-2` mit atmendem Signalpunkt; der Text scrollt mit, solange man unten ist.
- **Aufnahmeleiste (Signatur):** Tintenleiste unten wie die Player-Leiste. Aufnehmen ist der Signalkreis (3,25rem, Mikrofon in Tinte) mit Beschriftung und dem eingestellten Stand darunter; läuft die Aufnahme, Papierpille „Beenden“ und die Laufzeit mit Signalpunkt. Pegel als 0,5rem-Balken in Signalgrün auf Papier 16 %, ohne Aufnahme gedämpft. Rechts „Einrichten“ wie die Tempo-Pille; es öffnet über der Leiste ein Papierfenster (1rem, Schwebend hell) mit der Quelle als drei Zeilen zum Wählen, Mikrofon, Sprache und Speichern.
- **Programme verbinden:** eigene Ansicht in der Hauptfläche, Blätter für Adresse (mit den beiden Arten als Begriffsliste), wer sich verbinden darf, und das Protokoll für Entwickler; wer sich verbinden wollte, steht zuoberst auf Signal-Hauch.

### Einfache Seiten
Bibliothek, Einstellungen und Lizenzen: Seitentitel mit Einleitung in `ink-2` (höchstens 60ch), darunter Blätter. Listen als Zeilen mit Haarlinien in Nebelgrau, jede Zeile ein ganzes Klickziel; Fortschritt als 0,5rem-Balken in Signalgrün auf Nebelgrau. Die Leiste der Ersteinrichtung liegt als Tintenband unten (beim Fehler `red-200`).

- **Bibliothek:** rechts vom Seitentitel „Dateien hinzufügen“ als Tintenpille. Darunter, nur wenn es sie gibt, das Blatt „In Arbeit“ (laufende und gescheiterte Aufträge) und das Blatt „Noch nicht transkribiert“ (Aufnahmen zum Ankreuzen, Sprecher und Sprachen, „Transkribieren“). Dann das Regal „Transkribiert“ (Überschrift 1,5rem, Suche und Sortierung rechts): ein Raster aus Blättern (ab 21rem breit, Abstand 1,25rem), jedes ein kleines Manuskript. Titel 700 / 1,1875rem, darunter Länge, Sprecherzahl und Datum als Meta, nach einer Haarlinie der Anfang des Transkripts in `ink-2` (vier Zeilen), davor der erste Sprecher als Sprecher-Label in seiner Farbe; unten die Sprecher mit Punkt und die Arbeit von Hand (Korrekturen, Clips) mit Symbol. Das ganze Blatt öffnet den Editor; beim Überfahren hebt es sich um 2 px mit weiterem Schatten und der Titel bekommt die Signal-Unterstreichung. „Neu transkribieren“ ist ein 44-px-Kreis oben rechts. Ein Blatt, das gerade neu transkribiert wird, trägt einen 2-px-Signalrand.
- **Einstellungen:** das Menü als Pillen (600, `ink-2`, Hover Papier), der Abschnitt im Blick als Tintenpille mit Papierschrift.

## Do's and Don'ts

### Do:
- **Do** jede menschliche Änderung im Editor in Redaktionsrot zeigen, in Text und Zeitleiste zugleich (The Rotstift Rule).
- **Do** neue Editorfarben als Rolle auf `.editor` anlegen und auf eine Moliri-Stufe legen (The Moliri-Stufen Rule); neue Bedeutungsfarben vorher gegen Rot, Regie-Blau, Signalgrün und die Sprecherfarben prüfen (The Freihalte Rule).
- **Do** Signalgrün als Fläche mit Tinte für „läuft gerade“ einsetzen, als Schrift auf Papier `signal-text` (The Signal Rule).
- **Do** gemischte Töne mit `color-mix(in oklab, …)` aus Moliri-Stufen bilden, wie Moliris Knopf-Hover.
- **Do** Klickziele mindestens `--hit` (44 px) hoch halten und jede Funktion auch ohne Tastenkürzel und ohne Hover erreichbar machen.
- **Do** Moliri-Komponenten aus `$lib/moliri` nehmen, wo es sie gibt, und Abweichungen für PRODUCT.md in `app.scss` vornehmen, nicht in der gespiegelten Datei.
- **Do** Zeiten mit Tabellenziffern setzen (The Ziffern-Heißt-Zeit Rule).
- **Do** Zustände durch Linienart unterscheiden: durchgezogen für Feststehendes, gepunktet für Zweifel, gestrichelt für Eingeklapptes.
- **Do** für neue Seiten dieselbe App-Leiste, Papier auf Nebelgrau und Pillen verwenden; Editor-Seiten tragen Tinte nur als Player-Leiste und schwebende Leisten.

### Don't:
- **Don't** Redaktionsrot im Editor für Fehler, Warnungen, Löschen ohne menschlichen Bezug oder Hervorhebung einsetzen.
- **Don't** Signalgrün (`green-100`) oder Schiefer (`neutral-50`) als Schrift auf Papier setzen.
- **Don't** Hex-Werte in Komponenten schreiben oder `frontend/src/lib/moliri/` von Hand ändern.
- **Don't** Blau als Bedienakzent nutzen; im Editor ist Blau Regie.
- **Don't** den Standard-Editor bauen: graue Fläche, blauer Allzweck-Akzent, Wellenform-Kasten als Mittelpunkt.
- **Don't** „ergänzt“ oder andere Maschinenzustände mit einer Farbe kennzeichnen, die als Sprecher lesbar ist.
- **Don't** Versalien mit Sperrung als Vorspann über Überschriften setzen.
- **Don't** Werte der früheren statischen Seiten übernehmen (`#1c2b45` als Clipfarbe, eigene Schriftdateien).
- **Don't** Schriftfamilien beim Namen nennen; nur `var(--font-ui)` und `var(--font-mono)` (The Ein-Name Rule).
- **Don't** Zweifel und Rotstift in benachbarten Tönen zeigen (The Gewissheit-gegen-Zweifel Rule).
- **Don't** harte, versetzte Schatten verwenden.
