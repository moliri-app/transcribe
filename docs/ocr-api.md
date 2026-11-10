# Text recognition (OCR) API

Transcribe reads the text of images for other programs: Moliri, a browser extension, a script.
Recognition runs on this computer and the image does not leave it, except with the engine `api`
(an AI service with the user's own key, below), which reads only when a request names it or the
user made it the default in the settings.

The app listens on `http://localhost:31765` (or the port given with `--port`; the Live page shows the
address in use).

| Request | Answer |
| --- | --- |
| `POST /api/ocr` | the text of one image: a result in format 1 (below) |
| `GET /api/ocr/engines` | the engines, their models and whether they are ready |
| `GET /api/ocr/schema` | the JSON schema of the result (`application/schema+json`) |

## Who may call

The same approval as for live transcription ([live-api.md](live-api.md)): programs that are no web
page (no `Origin` header) and pages on this computer (`localhost`, `127.0.0.1`) always; pages from
other addresses only once the user allowed them on the Live page under „Wer sich verbinden darf“. A
page that tries is listed there, and gets `403` with `{"ok": false, "error": "… erlauben: <origin>"}`
it can read and show.

The switch „Im Hintergrund bereit“ does not matter here: it only decides whether programs may have
Transcribe record the microphone. Images are read while it is off too.

Pages call from their own address, so the answers carry CORS headers
(`Access-Control-Allow-Origin` for the calling page; a preflight `OPTIONS` is answered, with
`Access-Control-Allow-Private-Network: true` when a public page asks for `localhost`). Requests
addressed to another host name than `localhost`, `127.0.0.1` or `::1` are refused (DNS rebinding).

## Reading an image

Either as a form (`multipart/form-data`, no preflight):

```js
const form = new FormData();
form.append('image', blob, 'seite.jpg');
form.append('languages', 'deu+frak2021');
form.append('layout', 'lines');
const res = await fetch('http://localhost:31765/api/ocr', { method: 'POST', body: form });
```

or with the image as the body and the options in the query:

```
POST /api/ocr?engine=tesseract&languages=deu+eng&layout=block
Content-Type: image/jpeg

<image>
```

| Field | Default | Meaning |
| --- | --- | --- |
| `image` | – | the image (JPEG, PNG, TIFF, WebP, BMP, GIF), at most 64 MB |
| `engine` | the default | an `id` of `/api/ocr/engines`: `tesseract`, `onnx` (PP-OCRv5), on Windows also `os`, and `api` (an AI service). Without one: the engine chosen in the settings („Für Moliri und andere Programme“) when it is usable, else the first usable one other than `api` |
| `languages` | `deu` | models joined by `+`, as `/api/ocr/engines` names them; case does not matter. `onnx` reads every Latin-script language with one model (`latin`) and takes Tesseract's names of them too (`deu`, `eng+fra`, …) |
| `layout` | `auto` | `auto` (pages, columns), `lines` (forms, index cards), `block` (a label), `sparse` (writing on objects) |

Every engine takes the same `languages` and `layout`; one that has fewer reads with what it has and
says so in the result (`language`, `layout`): the Windows engine reads `deu+frak2021` as `deu`.

The answer waits for the reading (most images one to twenty seconds; the first reading on Windows
downloads Tesseract and the models it needs first, about 6 MB plus 2–11 MB per model; the first
reading with `onnx` downloads its two models, 13 MB, and the speech recognition runtime if it is not
there yet). Errors are
`{"ok": false, "error": "…"}`:

| Status | When |
| --- | --- |
| `400` | unknown language or layout, no image |
| `403` | the page is not allowed (above) |
| `404` | unknown engine |
| `413` | image larger than 64 MB |
| `422` | not an image the engine can read |
| `502` | the AI service (`api`) refused or failed: key, model, credit, rate limit, timeout; `error` says which, in German |
| `503` | the engine is not installed and cannot be downloaded (Linux, macOS: install tesseract); `api` without a key |

## Result, format 1

Every engine and every place (this app, Moliri's server service, the browser, an AI service through
Moliri) gives the same format; the schema is [`ocr/format.schema.json`](../ocr/format.schema.json),
served at `/api/ocr/schema`. Readers refuse a `format` they do not know.

```json
{ "format": 1, "engine": "desktop:tesseract-5.0.0", "language": "deu+eng", "layout": "block",
  "pages": [ { "width": 1300, "height": 780, "lines": [
    { "bbox": [229, 290, 194, 39], "para": 1, "text": "Pewter plate",
      "words": [ { "w": "Pewter", "bbox": [229, 302, 108, 27], "p": 0.926 },
                 { "w": "plate", "bbox": [346, 290, 77, 35], "p": 0.97 } ] } ] } ] }
```

- `engine`: place, engine and version (`desktop:…` from this app, `server:…` from Moliri's service).
- Boxes are `[x, y, width, height]` in pixels of the image that was read; `width` and `height` of the
  page scale them to any size.
- `para` numbers the paragraphs of a page; lines of one paragraph belong together.
- `p` is the engine's confidence 0–1. An engine without one (Windows) leaves it out of the words;
  readers take such a word as certain.
- An engine without word boxes leaves `bbox` out of the words, or gives no `words`; a line's
  confidence may then stand on the line (`p`). An engine that locates nothing leaves out the line's
  `bbox` too.

## Engines

```json
GET /api/ocr/engines
{ "ok": true, "format": 1, "place": "desktop", "accel": "cpu", "default": "tesseract",
  "engines": [ { "id": "tesseract", "name": "Tesseract", "version": "5.0.0",
                 "ready": true, "usable": true,
                 "languages": ["deu", "frak2021"], "download": ["Fraktur", "eng", "frk"],
                 "layouts": ["auto", "lines", "block", "sparse"], "boxes": "words", "gpu": false },
               { "id": "onnx", "name": "PP-OCRv5",
                 "ready": true, "usable": true, "languages": ["latin"],
                 "layouts": ["auto"], "boxes": "words", "gpu": true } ] }
```

- `default`: the engine a request without `engine` reads with; the list names it first, then the
  others in the app's order. "" when none can read.
- `ready`: reads now; `usable`: reads, perhaps after downloading what it lacks. A connection test
  shows an engine that is usable.
- `languages` are installed, `download` are fetched on first use.
- `boxes`: what the engine locates, `words`, `lines` or `none`.
- `accel` is the app's GPU choice (settings); `gpu` says whether an engine runs on the graphics card
  with it. Tesseract never does; `onnx` does when the app uses the GPU for speech recognition.

### Windows text recognition (`os`)

On Windows the app also reads with `Windows.Media.Ocr`, the text recognition of Windows itself
(Windows 10 and later), through the Windows Runtime without any download:

```json
{ "id": "os", "name": "Windows-Texterkennung", "version": "10.0.26300", "ready": true, "usable": true,
  "languages": ["deu", "eng"], "layouts": ["auto", "lines"], "boxes": "words", "gpu": false }
```

- `languages` are the recognition languages installed in Windows (Settings › Time & language ›
  Language; German comes with German Windows), named like Tesseract's models (`de-DE` is `deu`), so a
  caller can ask both engines alike; a language without such a name keeps its tag. A request names
  them that way or by tag (`de-DE`, `de`). Windows reads one language at a time: the first of the
  request that is installed; `frak2021` and other models Windows lacks are passed over (Windows has
  no Fraktur). None installed: `usable` is false and `error` says how to add one.
- Windows finds the lines itself and reads a page column by column. `lines` (and `block`) join
  what stands side by side into one line, label and value of an index card; `auto` (and `sparse`)
  keep Windows' order.
- Words have boxes but no confidence (`p` left out); a line's box is the box around its words.
  Paragraphs are guessed from the gaps between lines. Skewed text: Windows reads it straightened,
  the boxes are turned back into the image. Images larger than Windows takes (10,000 pixels a side)
  are read shrunk, the boxes scaled back.
- `engine` in the result is `desktop:windows-ocr-<Windows version>`.
- The image is decoded by Windows (JPEG, PNG, TIFF, BMP, GIF; WebP and HEIF where their codecs are
  installed); of a TIFF with several pages only the first is read.
- The engine is listed only on Windows (macOS Vision may follow).

### AI service (`api`)

With a key of the user's own, an AI model reads the image: through the OpenAI-compatible interface
(OpenAI, Mistral, or a server of one's own such as Ollama, which needs no key), Anthropic's Messages
API or Google's Gemini API. The service, its address, the model and the key are set in the app
(Settings › Texterkennung › „KI-Dienst“), apart from the speech recognition service, and kept in
`settings.json` in the data folder; they never go to Moliri or any caller. The settings are read at
every request, so a new key counts at once.

```json
{ "id": "api", "name": "KI-Dienst (OpenAI)", "version": "gpt-5-mini", "ready": true, "usable": true,
  "languages": ["deu", "eng", "fra", "ita", "nld", "spa", "frak2021"],
  "layouts": ["auto", "lines", "block", "sparse"], "boxes": "lines", "gpu": false }
```

- It costs money and the image leaves the computer, so it never reads for a request that names no
  engine, unless the user chose it in the settings as the engine for programs (`default` then says
  `api`, and Moliri, which names no engine, reads with it). Without a key (a hosted service) it is
  not `usable`, and `error` says what is missing.
- `version` is the model. The model reads every language and script; `languages` are the ones
  callers usually offer, and any other name of Tesseract's kind is taken too (`lat`, `eng+fra`). The
  prompt names the languages it knows ("probably German or German (Fraktur)"); `lines` adds that a
  label and its value stay on one line (index cards), the other layouts make no difference.
- The image goes as JPEG of quality 90, its longest side at most 1600 pixels. The model answers with
  one entry per line, its text and a box in thousandths of the image; the answer may also be a bare
  list, in a code fence, or plain text, one line per line, and boxes in pixels of the image sent.
  Lines without a box get one between their neighbours, a page without any box equal bands. Words
  have no boxes and no confidence: the editor marks the line. Paragraphs are the model's, or one per
  line.
- Prompt and reading of the answer are the same as Moliri's own AI path on the server
  (`static/api/_func/ai_ocr.php`, tested alike in `tests/php/ai_ocr.test.php` there and
  `internal/ocrapi/parse_test.go` here): a page read by the app or by Moliri's server gives the same
  result. Change both together.
- `engine` in the result is `desktop:api:<model>`, e.g. `desktop:api:gemini-2.5-pro`.
- Errors of the service come back as `502` with what went wrong in German: the key refused, the
  model unknown or reading no images, credit or rate limit used up, the address no such interface,
  no answer within 150 s. GPT-5 and other reasoning models of OpenAI refuse `temperature` and
  `max_tokens`; the app asks them again without (`max_completion_tokens`).

| Service | Default address | Default model |
| --- | --- | --- |
| OpenAI | `https://api.openai.com/v1` | `gpt-5-mini` |
| Anthropic | `https://api.anthropic.com/v1` | `claude-opus-5-5` |
| Google Gemini | `https://generativelanguage.googleapis.com/v1beta` | `gemini-2.5-pro` |
| Mistral | `https://api.mistral.ai/v1` | `mistral-medium-latest` |
| Own server (OpenAI-compatible, no key) | `http://localhost:11434/v1` (Ollama) | `qwen2.5vl:7b` |

### Tesseract

Tesseract on Windows comes from the NuGet package „Tesseract“ (Tesseract 5 and Leptonica as DLLs)
into `<data>/tesseract/lib`, the models (`deu`, `eng`, `frk`, `Fraktur` from tessdata_fast 4.1.0,
`frak2021` from UB Mannheim) into `<data>/tesseract/tessdata`, each pinned by its checksum. On Linux
and macOS the tesseract program of the system reads with its own models
(`apt install tesseract-ocr tesseract-ocr-deu`, `brew install tesseract tesseract-lang`).

PP-OCRv5 (`onnx`) is PaddleOCR's text detection and its recognition of Latin script, the mobile
models as RapidOCR 3.10 publishes them in ONNX, downloaded on first use into `<data>/ppocr` and pinned
by checksum. They run on the ONNX Runtime that comes with speech recognition (`<data>/lib`, or
`<data>/lib-cuda` on the graphics card), called through its C API with purego (`ort`);
pre- and post-processing are RapidOCR's, in Go (`ocr/ppocr*.go`). Detection finds the
lines, recognition reads them, the columns where its characters fall give the word boxes. Pieces of
one row (a label and its value on an index card) become one line; a larger gap between lines starts
a paragraph. The layout option makes no difference to it.

## Tests

`go test ./internal/ocrapi` checks the AI service against fakes of the three interfaces (request
shape, headers, image part, answers, errors) and the reading of answers against Moliri's
expectations. `go test ./ocr/...` checks the format, the schema and the engine list; with the tesseract
program installed it also reads the test set (`testdata/ocr`, the same as Moliri's
`server/transcribe/testdata/ocr`). On Windows the library reads it with
`TRANSCRIBE_OCR_DATA=<folder> go test ./internal/tesslib`, which downloads Tesseract and the models into
that folder first, as the app does. Measured on 2026-11-02 (Tesseract 5.0.0 from the package):

| Image | Languages | Layout | Character errors | Words found |
| --- | --- | --- | --: | --: |
| Karteikarte | deu | lines | 2.4 % | 88 % |
| Karteikarte | deu+frak2021 | lines | 2.4 % | 88 % |
| Etikett | deu+frak2021 | block | 0.0 % | 100 % |
| Frakturdruck | deu+frak2021 | auto | 1.6 % | 92 % |

The same as the server's Tesseract 5.3 (2.4 %, 2.4 %, 0.0 %, 1.5 %).

PP-OCRv5 reads the test set with `TRANSCRIBE_ORT_LIB=<data>/lib TRANSCRIBE_OCR_MODELS=<data>/ppocr go
test ./ocr` (with `TRANSCRIBE_ORT_CUDA=1` and `<data>/lib-cuda` on the graphics card).
Measured on 2026-11-03, the same on CPU and GPU:

| Image | Character errors | Words found |
| --- | --: | --: |
| Karteikarte | 0.0 % | 100 % |
| Etikett | 0.0 % | 100 % |
| Frakturdruck | 3.5 % | 86 % |

On Moliri's larger measurement (`docs/ki-einsatz-skizze.md` §2.5 there) it reads typed index cards
with 0.7 % character errors, real Fraktur prints with 1.8 % (Tesseract `deu+frak2021`: 2.8 % and
1.6 %), handwriting no better than the others (50 %). On a laptop under full load a page took a
median 1.6 s on the CPU (4 threads) and 1.4 s on the GPU.

`go test ./internal/osocr` reads the same set with the Windows engine when German is installed in
Windows (otherwise it is skipped). Measured on 2026-11-03 (Windows 10.0.26300, `deu`; Fraktur is read
as German):

| Image | Languages | Layout | Character errors | Words found | Seconds |
| --- | --- | --- | --: | --: | --: |
| Karteikarte | deu | lines | 2.1 % | 90 % | 0.1 |
| Etikett | deu+frak2021 | block | 0.0 % | 100 % | 0.1 |
| Frakturdruck | deu+frak2021 | auto | 38.9 % | 14 % | 0.5 |
