# CLAUDE.md

## After landing

As soon as a piece of work has landed on main, update the main checkout at `C:\GIT\anderes\transcribe-go` to the current main and rebuild `transcribe.exe` there with `.\build.ps1`, so the owner can try every finished step right away. If the app is running, the EXE cannot be overwritten: then tell the owner instead.

## The engine is a module of its own

The top-level packages (`pipeline`, `annotate`, `export`, `model`, `models`, `sherpa`, `audio`,
`ocr`, `ort`, `embed`, `jobs`, `stream`, `fileutil`, `proc`) are the engine that Moliri's
`server/transcribe` imports at a tagged version. They must not import anything from `internal/` and
must build with `CGO_ENABLED=0` for Linux (`TestEngineStandsOnItsOwn` in `pipeline` checks both).
What only the desktop app has (settings, GPU download, window, microphone) reaches the engine as an
option or interface (`pipeline.Runner.Runtime`, `Voice`), wired up in `internal/runner`. A change
to the engine that Moliri should get needs a new tag; Moliri then raises its version in
`server/transcribe/go.mod` and runs `go mod vendor`.

## Interface texts: English in the code, translations in the catalogues

The frontend speaks Moliri's languages (en, de, es, fr, ja, uk, el) with wuchale, set up as in
Moliri (`frontend/wuchale.config.js`). Interface texts are written in **English** in the code;
text nodes and text attributes (`title`, `aria-label`, `placeholder`, `label` …) are picked up on
their own, every string in script or inside `{…}` only with `/* @wc-include */` in front of it, and
counts with `plural(n, ['# file', '# files'])` (`src/locales/plural.js`). In plain `.js` modules
only texts inside functions are translated. Dates and numbers are formatted with `getLocale()` from
`$lib/moliri/i18n/locale.js`, never a hard-wired `'de-DE'`. A new or changed text needs its
translation in every `frontend/src/locales/<locale>.po` (run `pnpm i18n:extract` in `frontend/`,
then fill the empty `msgstr`); a missing one shows English. The mirrored Moliri components get
their translations from Moliri through `pnpm sync:moliri`. The language is chosen in the settings
(System › Language) and otherwise follows the browser's, i.e. Windows', language.
