# Embedding API (search by meaning)

Transcribe turns short texts into sentence embeddings for other programs: Moliri's field pickers
find fields by meaning with them („Künstler“ finds *Urheber*). The model is EmbeddingGemma (300M
parameters, the 8-bit ONNX export of onnx-community), the same one Moliri runs in the browser with
transformers.js, computed the same way (mean of the last hidden state over the tokens, normalized
to length 1). Vectors from Transcribe and from the browser can therefore be compared with each
other: on 23 field names and queries their cosine was at least 0.9999995.

The app listens on `http://localhost:31765` (or the port given with `--port`; the Live page shows the
address in use). Texts do not leave the computer.

| Request | Answer |
| --- | --- |
| `GET /api/embed` | the model, its prompts and whether it is downloaded; starts loading it |
| `POST /api/embed` | the vectors of up to 64 texts |
| `POST /api/embed/download` | starts the download of the model (295 MB) |

## Who may call

The same approval as for text recognition ([ocr-api.md](ocr-api.md)) and live transcription:
programs that are no web page (no `Origin` header) and pages on this computer (`localhost`,
`127.0.0.1`) always; pages from other addresses only once the user allowed them. A page that
tries is asked about on whatever page of the app is open (and with a notification at the tray icon
while no window is in sight), is listed under Einstellungen › Programme › Zugriff, and gets `403` with
`{"ok": false, "error": "Diese Seite darf die Suche nach Bedeutung nicht nutzen. … erlauben: <origin>"}`.
The answers carry CORS headers for the calling page, and the preflight `OPTIONS` is answered.

## The model

```
GET /api/embed
```

```json
{
  "ok": true,
  "model": "onnx-community/embeddinggemma-300m-ONNX",
  "dtype": "q8",
  "dim": 768,
  "prefixes": { "query": "task: search result | query: ", "document": "title: none | text: " },
  "installed": true,
  "loaded": false,
  "size": 295,
  "download": { "state": "downloading", "done": 104857600, "total": 308890624, "error": "" }
}
```

`model` and `dtype` name the model as Hugging Face and transformers.js do: a caller that keeps
vectors (Moliri stores them per browser) uses Transcribe only when both match its own. `download` is
there while a download runs or after one failed. When the model is downloaded but not in memory,
this request starts loading it (about ten seconds), so that the first texts need not wait: ask it
when a search is near, as Moliri does when a field picker gets focus.

The model comes on request only, never at start: from the settings page („Suche nach Bedeutung
(EmbeddingGemma)“) or with `POST /api/embed/download`, which answers at once with the same JSON;
`GET /api/embed` then shows how far the download got. Its three files are pinned by revision and
checksum and land in `<data>/embeddinggemma`.

## Vectors

```js
const res = await fetch('http://localhost:31765/api/embed', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ texts: ['Künstler'], kind: 'query' })
});
const { vectors } = await res.json(); // [[-0.0808, 0.0045, …]], 768 numbers each
```

| Field | Meaning |
| --- | --- |
| `texts` | 1 to 64 texts, each at most 4000 bytes of UTF-8 |
| `kind` | `query` (what someone searches for) or `document` (what is searched in, a field's name and description) |

EmbeddingGemma gets a prompt before each text: `task: search result | query: ` for a query,
`title: none | text: ` for a document. Transcribe adds it; send the texts without. The answer is
`{"ok": true, "model": …, "dtype": "q8", "dim": 768, "vectors": [[…], …]}` with one vector per
text, in their order. The vectors have length 1, so their dot product is the cosine.

Texts run in batches of eight; on a laptop's processor a query takes about 0.05 to 0.2 seconds, a
document in a batch about 0.06 seconds. The model stays in memory (about 1.4 GB) until a quarter of
an hour has passed without a request.

| Status | When |
| --- | --- |
| `400` | no texts, more than 64, one too long, `kind` neither `query` nor `document`, no JSON |
| `403` | the page is not allowed (above) |
| `503` | the model is not downloaded yet |
| `500` | the model failed to load or to run; `error` says why |

## Why 1.4 GB

The export keeps its weights in 8 bits and has every run turn them into floats: about a second per
run on a laptop, however short the text. Transcribe turns them once when loading (onnxruntime's
QDQ handling is off for this model, so constant folding takes them), which makes a query about ten
times faster, for 1.4 GB of memory instead of 0.5 GB. The floats are the same ones each run would
compute, so the vectors do not change. The browser keeps the 8-bit weights and takes about a second
per query.
