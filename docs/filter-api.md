# Filter API (a filter in plain words)

Moliri's detail search has an input „Filter in eigenen Worten“: a sentence like „Gemälde nach 1900
ohne Standort“ goes with the project's field catalog to a language model, which proposes the
conditions of a filter. Moliri checks them against its catalog and shows them as a filter that
applies only on a click. Transcribe answers the same question with a language model on this
computer, so that neither the sentence nor the field names leave it.

The model is Qwen 2.5 7B Instruct (Qwen's own 4-bit GGUF, Q4_K_M), on llama.cpp's server program
(`llama-server`, release b12110), which the app starts as a child process on `127.0.0.1` and a port
of its own. The prompt and the check of the answer are those of Moliri's server
(`static/api/_func/natural_filter.php`), so the answer is the one the instance's AI account would
give with the same model.

The app listens on `http://localhost:8765` (or the port given with `--port`; the Live page shows the
address in use).

| Request | Answer |
| --- | --- |
| `GET /api/filter` | the model and whether it is downloaded; with `?warm=1` starts loading it |
| `POST /api/filter` | asks a question; answers at once with its `id` |
| `GET /api/filter?id=…` | how far the question got, and the answer once it is there |
| `POST /api/filter/cancel` | drops a question |
| `POST /api/filter/download` | starts the download of model and server program (4.5 GB) |

## Who may call

The same approval as for text recognition ([ocr-api.md](ocr-api.md)) and the search by meaning
([embed-api.md](embed-api.md)): programs that are no web page (no `Origin` header) and pages on this
computer (`localhost`, `127.0.0.1`) always; pages from other addresses only once the user allowed
them on the Live page under „Wer sich verbinden darf“. A page that tries is listed there, and gets
`403` with `{"ok": false, "error": "Diese Seite darf den Filter in eigenen Worten nicht nutzen. …
erlauben: <origin>"}`. The answers carry CORS headers for the calling page, and the preflight
`OPTIONS` is answered.

## The model

```
GET /api/filter
```

```json
{
  "ok": true,
  "model": "Qwen 2.5 7B Instruct (Q4_K_M)",
  "installed": true,
  "loaded": false,
  "size": 4490,
  "queued": 0,
  "comparisons": ["contains", "not_contains", "equals", "…"],
  "download": { "state": "downloading", "done": 1048576000, "total": 4683073632, "error": "" }
}
```

`download` is there while a download runs or after one failed; a broken-off download goes on where
it stopped. `GET /api/filter?warm=1` also starts loading the model when it is downloaded but not
in memory (some seconds), so that the question need not wait: ask so when a question is near, as
Moliri does when the input gets focus. Without `warm` nothing loads: the model takes some
gigabytes of memory.

The model comes on request only, never at start: from the settings page („Filter in eigenen Worten
(Qwen 2.5 7B)“) or with `POST /api/filter/download`, which answers at once with the same JSON;
`GET /api/filter` then shows how far the download got. Its files are pinned by revision and
checksum and land in `<data>/llm`.

## A question

```js
const res = await fetch('http://localhost:8765/api/filter', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    text: 'Gemälde nach 1900 ohne Standort',
    catalog: 'Feldkatalog:\n- `Objektart` (Liste) …',
    fields: ['Objektart', 'Datierung von', 'Dauerstandort']
  })
});
const { id } = await res.json(); // 202 {"ok": true, "id": "…", "status": "pending", "ahead": 0}
```

| Field | Meaning |
| --- | --- |
| `text` | the sentence, at most 500 characters |
| `catalog` | the fields as the model reads them, written by Moliri's context source (`aiContext.js`), at most 60 000 characters |
| `fields` | every field name the answer may use, at most 1500 |

The question runs in the background: on a laptop's processor the model needs minutes to read a
large catalog, longer than a page should hold a request open. The page asks for the state every
second or two:

```
GET /api/filter?id=…
```

```json
{ "ok": true, "id": "…", "status": "processing", "ahead": 0,
  "stage": "reading", "progress": { "done": 1536, "total": 3021, "cached": 0 },
  "elapsed": 84.2, "waited": 84.2 }
```

| `status` | Meaning |
| --- | --- |
| `pending` | waits; `ahead` questions are asked before it, `waited` seconds since it came |
| `processing` | is being asked; `stage` says how far |
| `done` | the answer is there (below) |
| `failed` | `error` says why, `error_kind` what kind: `cancelled`, `not_installed`, `too_long` (the catalog does not fit into the model's 16 384 tokens), `no_json` (the answer held no JSON object; `error` has its beginning), `failed` |

| `stage` | Meaning |
| --- | --- |
| `loading` | the model loads into memory (some seconds after a start or a quarter of an hour without questions) |
| `reading` | the model reads the prompt: `progress.done` of `progress.total` tokens; `progress.cached` of them it had read before |
| `writing` | the model writes the answer: `progress.done` tokens so far |

When done, the answer has the shape of Moliri's own (`R/natural_filter.php?id=…`):

```json
{ "ok": true, "id": "…", "status": "done", "ahead": 0,
  "conditions": [
    { "join": "AND", "field": "Objektart", "comparison": "equals", "value": "Gemälde" },
    { "join": "AND", "field": "Datierung von", "comparison": "greater", "value": "1900" },
    { "join": "AND", "field": "Dauerstandort", "comparison": "empty", "value": "" }
  ],
  "not_understood": [], "dropped": [], "seconds": 63.5, "model": "Qwen 2.5 7B Instruct (Q4_K_M)" }
```

Only field names of `fields` and comparisons of `comparisons` stay, at most eight conditions;
`dropped` names the fields the model made up. An answer waits ten minutes to be fetched.

Questions run one after the other, beside the transcriptions (which then take longer). `POST
/api/filter/cancel` with `{"id": "…"}` drops a waiting question, or stops the model on the one it
is asking.

| Status | When |
| --- | --- |
| `400` | no text, catalog or fields, no JSON |
| `403` | the page is not allowed (above) |
| `404` | `?id=` of a question that is not (or no longer) there |
| `429` | twenty questions wait already |
| `503` | the model is not downloaded yet |

## Time and memory

Measured on a laptop without a graphics card (Intel, 12th generation) with the field catalog of the
Cleveland collection in Moliri's test data (242 fields, a prompt of 3200 tokens): Qwen 2.5 7B reads
about 45 tokens of prompt a second and writes 3 to 4 a second. The first question took 104 s (70 s
reading, 35 s writing an answer of 120 tokens). What the model read of a catalog is kept, so the
next question about the same project reads only the sentence: 34 to 42 s, nearly all of it writing.
It is kept in memory for the catalogs of the last questions (up to 2 GB), and on disk for the last
six (`<data>/llm/cache`, about 0.06 MB a token, 190 MB for this catalog), also across a restart of
the app: the first question after one took 34 s, loading the model included. The prompt has the
rules and the catalog first and the sentence last for this.

The model takes about 5.5 GB of memory while it is loaded, and is given back after a quarter of an
hour without a question. The server program ends with the app, also when the app crashes.

Qwen 2.5 3B was measured too: faster, but its conditions were wrong.
