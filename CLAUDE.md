# CLAUDE.md

## After landing

As soon as a piece of work has landed on main, update the main checkout at `C:\GIT\anderes\transcribe-go` to the current main and rebuild `transcribe.exe` there with `.\build.ps1`, so the owner can try every finished step right away. If the app is running, the EXE cannot be overwritten: then tell the owner instead.

## The engine is a module of its own

The top-level packages (`pipeline`, `annotate`, `export`, `model`, `models`, `sherpa`, `audio`,
`ocr`, `ort`, `jobs`, `stream`, `fileutil`, `proc`) are the engine that Moliri's
`server/transcribe` imports at a tagged version. They must not import anything from `internal/` and
must build with `CGO_ENABLED=0` for Linux (`TestEngineStandsOnItsOwn` in `pipeline` checks both).
What only the desktop app has (settings, GPU download, window, microphone) reaches the engine as an
option or interface (`pipeline.Runner.Runtime`, `Voice`), wired up in `internal/runner`. A change
to the engine that Moliri should get needs a new tag; Moliri then raises its version in
`server/transcribe/go.mod` and runs `go mod vendor`.
