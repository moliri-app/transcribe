# Live transcription API

Transcribe can turn speech into text while it is spoken, for other programs: Moliri, a browser
extension, a script. Every request is kept as a note on the app's **Live** page, where its text
scrolls by as well.

The app listens on `http://localhost:31765` (or the port given with `--port`; the Live page shows
the address in use). Live transcription is a WebSocket:

```
ws://localhost:31765/api/live
```

Recognition runs on this computer (Parakeet with Silero VAD); nothing leaves it. While a live
transcription runs (a request here, a recording on the Live page or dictation), the transcription
of whole recordings in the library pauses where it is and goes on once the last one has ended, so
the live text comes first. The background mode
(„Im Hintergrund bereit“), switched on and off on the Live page and in the settings under „Andere
Programme“, keeps the model loaded while it is on (the default), so a request starts at once; only
when the model has not been downloaded yet, the first request downloads and loads it, which takes a
while. The switch only concerns listen mode (`mode=mic`, Transcribe recording the microphone or
what the Live page records): while it is off, such a request is turned away with an `error`
message, and nothing is kept loaded. Audio a program sends (audio mode) is transcribed either way,
loading recognition when it starts, and so are images sent to text recognition
([ocr-api.md](ocr-api.md)). Recording by hand on the Live page and dictating with the global
shortcut work either way too.
If the settings (Recognition) send live transcription to a service's API instead, the audio goes
there: to ElevenLabs as a stream, to an OpenAI-compatible service utterance by utterance, each
after a pause (no `partial` messages then). The protocol here stays the same.

## Two modes

| Mode | URL | Who records | What the program does |
| --- | --- | --- | --- |
| Audio (default) | `/api/live` | the program | sends audio as binary messages, gets the text back |
| Listen | `/api/live?mode=mic` | Transcribe, from the source chosen on the Live page | gets the text from now until it sends `stop` |

The source chosen on the Live page is the microphone, the system audio (what the computer plays,
such as the others in an online meeting), or both mixed. In listen mode it is recorded once,
however many programs listen: each gets what was said from the moment it connected. A recording
started on the Live page and programs listening at the same time share it.

The language chosen there applies too: for German the app uses its own German model, otherwise
Parakeet tells the language apart itself (audio mode always does that), also when several languages
are chosen. A recording saved from the Live page is transcribed in full in that language, or with
several only among them.

System audio is what plays on the default output device: on Windows through WASAPI loopback, on
Linux through the PulseAudio monitor of the default output. On macOS it cannot be recorded yet.

## Parameters

| Parameter | Values | Default |
| --- | --- | --- |
| `mode` | `audio`, `mic` | `audio` |
| `name` | what the note on the Live page is called (e.g. `Moliri`) | the page's host |
| `sample_rate` | 8000 to 96000 (audio mode) | `16000` |
| `encoding` | `s16le` (16-bit signed little endian) or `f32le` (32-bit float), mono (audio mode) | `s16le` |

Other rates are resampled linearly; 16 kHz from the sender gives the best result. In a browser,
`new AudioContext({ sampleRate: 16000 })` does that properly.

## Messages

From the program:

- binary: the next piece of audio (audio mode). Any size; 100 ms pieces work well. A sample may be
  split across messages.
- text `{"type":"stop"}`: the end. Transcribe recognises what was said until now, sends it, then
  `done`, and closes. Closing the socket without `stop` ends the note too, without the last words.

From Transcribe, all JSON text messages:

| `type` | Fields | Meaning |
| --- | --- | --- |
| `ready` | `mode`, `note` | recognition is loaded (audio sent earlier is not lost); `note` is the id of the note |
| `partial` | `text`, `start`, `end`, `source` | the utterance going on, as far as it goes; changes until its `final` |
| `final` | `text`, `start`, `end`, `source` | a finished utterance; `start` and `end` in seconds since the request began |
| `done` | | after `stop`: everything has been sent |
| `error` | `message` | in German, for the user; the socket closes after it |

An utterance ends when the speaker pauses for half a second; if it stops mid-sentence, Transcribe
waits a second longer for the speaker to go on, so that more `final`s are whole sentences.
Speech that goes on for long is ended after about 12 s, after its last sentence or comma. A
`final` is decoded once more with the seconds before it as context, from where the one before
stopped, so no word between two of them is lost; it comes about two seconds after its last word.
In between, a `partial` comes about every half second. Short or doubtful utterances (a word or two
the model is unsure of, or what sounds English) are checked with the German model unless another
language was chosen, and dropped only when it hears nothing either. Quiet input is raised before
recognition, slowly and without clipping.

What the user set under „Meine Stimme“ on the Live page applies to every request: their own words
(names and terms such as „Moliri“) are written as they are written there, and punctuation they say
aloud („Komma“, „Punkt“, „Fragezeichen“, „neuer Absatz“; „comma“, „full stop“, „new line“ in
English) becomes the mark or the line break, unless they switched that off. A mark said at the
start of an utterance belongs to the end of the one before: its `final` then begins with it
(`". Dann gehen wir"`, or just `"."` or `"\n\n"`). Put it onto the text before, without a space,
in place of a mark there. A `final` may end in a line break (`"\n"`, or `"\n\n"` for a
paragraph); the next one follows it without a space.

`source` is only there when the source chosen on the Live page mixes microphone and system audio:
`mic` or `system`, whichever was louder while the utterance was spoken. It tells the own voice
from the others in a meeting; several people on one side are not told apart.

## Who may connect

Programs that are no web page (no `Origin` header) and pages served from this computer
(`localhost`, `127.0.0.1`) may always connect. A page from any other address is turned away with an
`error` message the first time; the Live page then lists it with a button to allow it. Allowed
addresses are kept in `settings.json` (`liveOrigins`).

## Example

```js
const ws = new WebSocket('ws://localhost:31765/api/live?mode=mic&name=Moliri');
ws.onmessage = (e) => {
  const m = JSON.parse(e.data);
  if (m.type === 'partial') field.placeholder = m.text;
  if (m.type === 'final') field.value += m.text + ' ';
  if (m.type === 'error') alert(m.message);
};
// when the user is done:
ws.send(JSON.stringify({ type: 'stop' }));
```

Sending audio from the browser's microphone instead (audio mode):

```js
const ctx = new AudioContext({ sampleRate: 16000 });
const mic = ctx.createMediaStreamSource(await navigator.mediaDevices.getUserMedia({ audio: true }));
const ws = new WebSocket('ws://localhost:31765/api/live?name=Moliri&encoding=f32le');
ws.binaryType = 'arraybuffer';
const proc = ctx.createScriptProcessor(2048, 1, 1); // or an AudioWorklet
proc.onaudioprocess = (e) => ws.readyState === 1 && ws.send(e.inputBuffer.getChannelData(0).slice().buffer);
mic.connect(proc);
proc.connect(ctx.destination);
```

The Live page follows the same recognition over its own stream (`/api/live/events`, not part of
this API): it also learns there how much speech has been heard but not yet recognised, and shows
placeholders for it, and whether the microphone sends only telephone quality or is very quiet. The
messages of this API stay as they are.

## Notes

Each request is saved as `<data>/live/<id>.json`: who asked, when, and every utterance with its
times. The Live page shows the newest 200, copies their text and deletes them.

Under "Programme verbinden" on the Live page, saving requests can be switched off
(`liveApiNoSave` in `settings.json`): a request's note is then only held in memory, shown on the
Live page (marked "nicht gespeichert") while it runs and for five minutes after it ended, readable from `GET /api/live/note` for as long, and never written to disk.
Saved requests can instead be deleted automatically some time after they ended
(`liveApiKeepHours`; 0 keeps them), also those saved before; the app looks for them at start and
every ten minutes. Both apply only to requests over this API, not to recordings started on the
Live page or dictations. A recording started
on the Live page can also save its audio to `audio/Live <date> <time>.wav`; the Live page plays it
back beside its text, and from there it can be transcribed in full like any other recording, with
speakers.

When German was chosen for the recording, live transcription uses the German model, which keeps
up easily but makes more errors. Once such a note has ended, its whole audio runs through the
same recognition again with Parakeet, as fast as the computer allows, and the better words replace
those of its utterances (about a quarter of the errors on spontaneous speech, see
`docs/live-models.md`). The audio is kept for that while the note runs, and deleted afterwards
unless the recording saves it. The socket knows nothing of this: it has ended by then. A program
reads the note afterwards as plain text, one utterance per line, from
`GET /api/live/note?id=<note>`. Other languages are not recognised again (live transcription uses
Parakeet for them already), nor is a dictation or what a service recognised live.

For tests and demos without a microphone, `TRANSCRIBE_LIVE_INPUT=<file>` makes the app play that
file in real time instead of recording the microphone, `TRANSCRIBE_LIVE_SYSTEM_INPUT=<file>` one
instead of the system audio; several files (separated like `PATH`) play in turn, one per
recording.
`TRANSCRIBE_LIVE_DEVICES=<file>` stands in for the microphones connected, once that file exists:
a name per line, the system's default marked with a leading `* `; rewriting it plugs them in and
out, and the Live page follows.
