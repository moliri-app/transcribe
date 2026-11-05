# Transcripts next to the recording

A recording in a connected folder stays where it is. Unless the setting „Transkripte neben die
Aufnahmen legen“ is off, Transcribe keeps its transcript in a file next to it, the way photo
programs keep their edits in an `.xmp` next to the picture:

```
Interviews/
  Berger.mp3
  Berger.mp3.transcribe.json
```

The name is the full file name of the recording plus `.transcribe.json`, so `Berger.mp3` and
`Berger.wav` in one folder do not share one. Whoever connects the same folder (a Nextcloud or
network share used by a team) finds the transcript, the corrections and the clips there and goes on
working on them. Transcribe never deletes anything in a connected folder.

## Why a format of its own

No standard holds what Transcribe keeps apart:

| Format | Holds | Lacks |
|---|---|---|
| WebVTT, SRT | timed text, speakers as voices (VTT); players load a `.vtt` next to a video | word times and certainty, corrections apart from the machine's result, clips |
| Podcasting 2.0 transcript JSON | segments with speaker and times | the same |
| ELAN (`.eaf`) | tiers of timed annotations, common in linguistics and oral history | a place for the machine's result beside the corrections; no program reads it on its own |

So the sidecar holds the app's own JSON. The finished transcript sits under `final` for anyone who
only wants to read it; for subtitles or text, the editor exports VTT, TXT and HTML.

## Content

```json
{
  "format": "transcribe-by-moliri",
  "version": 1,
  "audio": "Berger.mp3",
  "saved": "2026-11-05T09:12:44Z",
  "final": { "...": "the transcript with corrections applied, as results/<stem>/_final.json" },
  "machine": { "...": "what recognition returned, as _machine.json" },
  "annotations": { "...": "the corrections by hand, speakers and clips, as _annotations.json" },
  "retranscribed": { "...": "passages recognised again in another language, as _retranscribed.json" }
}
```

`final` and `machine` are always there; `annotations` and `retranscribed` only when there are any.
The shapes are those of package `model`.

## How the app keeps it in step

The app works on its own copy in `data/results/<stem>/`, as for any other recording, and writes the
sidecar a moment after each change (a finished transcription, a correction in the editor). Whenever
the library or the editor lists the recordings, it reads every sidecar that changed since it last
wrote or read it (`_sidecar.json` in the copy remembers its size and time): someone else worked on
it. Corrections of the copy that the sidecar replaces are kept in `_annotations_history/` first.

Two people correcting the same recording at the same moment overwrite each other: the later save
wins, the earlier one stays in the history of the other's copy.
