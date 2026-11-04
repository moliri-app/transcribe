# Models for live transcription: evaluation (October 2026)

## What Live used before

German has since moved to Kroko for the preview and Parakeet for the final text: last section.

`internal/pipeline/live.go`: Silero VAD cuts the arriving audio into utterances (at most 15 s). The
open utterance is decoded again from its start every 0.4 s of new audio for the preview, and once
more when it ends for the final text. Both use an offline (non-streaming) model on 4 CPU threads:

- language `de`: NVIDIA FastConformer hybrid large, German (`stt_de_fastconformer_hybrid_large_pc`, int8)
- `auto` or any other language: NVIDIA Parakeet TDT 0.6B v3 (25 European languages, int8)

Both decode greedily; Parakeet with modified beam search once the user has own words (last section).

Whisper is already in the app, but not in Live: turbo transcribes files in languages other than
German and re-recognises marked passages; base identifies the language.

## Measurement

`internal/sherpa/livebench_test.go` (opt-in, see its header). Machine: Intel i9-11950H, CPU only,
4 threads, sherpa-onnx 1.13.8, int8 models, greedy search. WER is case- and punctuation-insensitive;
numbers written as words count as errors against digits in the reference.

- **Conversation**: a 2:47 min German podcast excerpt (two speakers, spontaneous speech, fillers,
  English phrases), recorded once clean and once through a headset, with a hand-made reference
  (484 words). Cut by Silero with the Live settings, every utterance decoded as Live does.
- **Read speech**: the first 65 utterances of FLEURS de_de dev (1353 words).

RTF is compute time per second of audio for the final text. Preview load is the compute time per
second of audio that the 0.4 s re-decoding costs while someone speaks (above 1 the preview falls
behind and refreshes less often); max lag is the longest single decode, i.e. how far the preview
trails at the end of a long utterance. Streaming models have no re-decoding: their load is their RTF.

| Model | Kind | WER conversation (clean / headset) | WER read | RTF | Preview load | Max lag |
| --- | --- | --- | --- | --- | --- | --- |
| German FastConformer (today, `de`) | offline | 22.9 % / 25.6 % | 6.7 % | 0.06 | 0.97 | 1.7 s |
| Parakeet TDT 0.6B v3 (today, `auto`) | offline | 8.9 % / 11.4 % | 7.2 % | 0.19 | 3.5 | 3.5 s |
| Whisper base | offline | 25.8 % / 32.9 % | 21.9 % | 0.42 | 10.9 | 9.8 s |
| Whisper large-v3 turbo | offline | 8.3 % / 10.1 % | 5.1 % | 1.26 | 32 | 24 s |
| Canary 180M flash (en/de/es/fr) | offline | 20.5 % / 28.9 % | 6.4 % | 0.39 | 5.7 | 6.6 s |
| Nemotron 3.5 ASR streaming 0.6B, 560 ms | streaming | 17.4 % / 21.1 % | 9.1 % | 0.51 | 0.51 | 0.7 s |
| Nemotron 3.5 ASR streaming 0.6B, 160 ms | streaming | 18.0 % / 21.1 % | 10.8 % | 1.57 | 1.57 | 2.7 s |
| Kroko streaming Zipformer, German (2025-08-06) | streaming | 9.3 % / 10.7 % | 6.1 % | 0.08 | 0.08 | 0.2 s |
| Kroko for the preview, Parakeet v3 for the final text (Live `de` now) | streaming + offline | 8.9 % / 11.4 % | 7.2 % | 0.08 + 0.19 | 0.08 | 0.2 s |

The last line is Parakeet's text with Kroko's preview, measured again with `BENCH_VAD=1` and
`BENCH_REF` (the reference, for the WER) on `clean.wav` and `headset.wav` of `data/bench/live`:
the same numbers as each model alone above (the read speech was not measured again).

For comparison, an existing Whisper transcript of the same conversation (`whisper.txt` next to the
recording, origin not recorded) has 7.4 % WER.

## Findings

- The German FastConformer that Live uses for `de` is good on read speech but makes almost three
  times the errors of Parakeet on spontaneous speech: it spells numbers out, drops "CCC" to "C",
  mangles names and English phrases.
- Whisper turbo is the most accurate, but on this CPU it needs about 13 s for a 10 s utterance:
  it cannot keep up live, not even for the final text. Whisper base is far behind. Whisper also
  hallucinates on silence and music (see the filters in `pipeline/engine.go`).
- Kroko (German only, 54 MB) matches Parakeet and Whisper turbo on the conversation, at a tenth of
  Parakeet's compute, and as a true streaming model it has no re-decoding: its longest single
  decode takes a fifth of a second. (That is compute, not how far the text trails: the model
  decodes in chunks, and a word shows about 0.9 s after it began; last section.) Licence CC BY-SA
  (Banafo names no version): attribution in NOTICE; share-alike concerns changed models, the
  unchanged model is downloaded at runtime like the others.
- Nemotron 3.5 streaming covers 40 languages with language detection, but makes about twice the
  errors of Parakeet on the conversation.

Limits: one conversation (two speakers, Hochdeutsch), no dialect, CPU only. FLEURS may have been
part of some models' training data.

Using a streaming model needs the sherpa-onnx online recognizer in `internal/sherpa` (the test
holds a working binding for 1.13.8) and a streaming variant of `liveStream`.

## Second pass after a recording (October 2026)

Once a recording has ended, its whole audio can be recognised again, without having to keep up.
Measured with `TestLiveBench` (both excerpts together, WER clean / headset), against the live
stream as it is now:

| | `de` | `auto` |
| --- | --- | --- |
| Live (German FastConformer for `de`, Parakeet for `auto`) | 19.5 % / 20.5 % | 4.9 % / 5.5 % |
| The full pipeline without speakers (Parakeet on VAD pieces, German check, Whisper turbo for the gaps) | 7.9 % / 11.1 % | 7.9 % / 11.1 % |
| The live stream again with Parakeet | 4.9 % / 5.5 % | 5.2 % (both together) |

The live stream decodes every utterance with the seconds before it, raises quiet input and ends
lines after whole sentences; the text step of the full pipeline does none of that and is now
worse than Live with Parakeet. Running the live stream again with Parakeet gains nothing for
`auto` (that is what Live does already; more context or a longer hold changed it by a few tenths),
but cuts the errors of `de` to a quarter. So only notes in `de` get a second pass
(`internal/pipeline/refine.go`): the live stream with Parakeet over the whole recording, about half
a second of computing per second of audio on this CPU (on a busy machine). The full pipeline's
text step has since learnt from the live stream (next section).

## The full pipeline's text step decodes as Live does (October 2026)

`Engine.Transcribe` (`internal/pipeline/engine.go`) no longer decodes Silero pieces of at most
15 s without context: it runs the whole recording through the live stream with Parakeet
(`decodeStream`, `liveTune`: context, gain, seamless cuts, held sentences), splits its lines at
the speaker turns afterwards and keeps the German check (`deFallback`) and Whisper turbo for the
gaps (`fillGaps`). Measured with `TestFileBench` (`internal/pipeline/file_bench_test.go`) on the
excerpts of `TestLiveBench`, both together, WER clean / headset, with the speakers the full
pipeline finds (DiarizeAHC2, number found), the computing time of the text step per second of
audio (on a busy machine):

| | Before | Now | Computing now |
| --- | --- | --- | --- |
| Live stream with Parakeet (for comparison) | 4.9 % / 5.5 % | 4.9 % / 5.5 % | 0.4 s |
| File, without speakers (one speaker) | 7.2 % / 11.2 % | 4.9 % / 5.5 % | 0.4 s |
| File with speaker cuts | 6.6 % / 11.0 % | 4.9 % / 5.5 % | 0.4 s |
| File with speaker cuts and Whisper turbo for the gaps (the full pipeline) | 6.6 % / 10.2 % | 4.7 % / 5.5 % | 0.5 s |

- Ending the stream's line at each speaker turn (a flush, as at the end of a recording) made it
  worse (5.3 % / 6.7 %): the cut falls in speech now and then, and the VAD starts afresh. The
  stream runs through, and its lines are split at the turns by the start of their words.
- A line starts with its first word, not where its utterance began: Whisper's words for speech
  the stream skipped (a repetition) were otherwise sorted after the line that follows them.
- The German check changed nothing measurable here; Whisper turbo for the gaps brings back a few
  words on clean audio.
- The text step takes about one and a half times as long as before (0.5 instead of 0.3 s per
  second of audio): every line is decoded with the 4 s before it, a line that stops
  mid-sentence once more.


## The user's own words: hotwords (October 2026)

„Meine Stimme“ on the Live page keeps a list of the user's own words (names, terms) that live
transcription should write as they are written (`internal/pipeline/voice.go`). sherpa-onnx can
favour words while decoding (hotwords, with modified beam search instead of greedy search). Both
NeMo transducers accept them; what they need is a vocabulary to split a hotword into the model's
tokens, which the model folders do not have (no `bpe.vocab`). It is made from `tokens.txt` when the
recognizer is created: `modeling_unit` `bpe` splits a word into the pieces of highest total score,
and a piece scores as a Zipf distribution gives it by its rank in `tokens.txt` (`-ln rank`); with
the id as score instead, the German model split „Moliri“ into single letters and got worse.

`TestHotwords` (`internal/sherpa/hotwords_test.go`) on six sentences synthesised with Windows'
voices (German Hedda, English Zira) with „Moliri“, „Obeya“ and „sherpa-onnx“:

- Greedy, Parakeet wrote „Moliri“ right but „Ubaya“, „Obaier“ for „Obeya“ and „Chopperons“,
  „Sherpa On X“ for „sherpa onnx“; with the hotwords at sherpa-onnx's default score 1.5 it wrote
  „Obeya“ in both German sentences. The German model needed a score of 4 for „Obeya“.
- Modified beam search with the hotwords took about 20 % (Parakeet) and 60 to 80 % (German) more
  time per decoding.

`TestLiveBench` with ten hotwords that are never said (TRANSCRIBE_LIVE_HOTWORDS, WER clean / headset,
`now`):

| | `de` live (German model) | `auto` live (Parakeet) |
| --- | --- | --- |
| Greedy (before) | 19.5 % / 20.5 % | 4.9 % / 5.5 % |
| Modified beam search, no hotword said | 22.5 % / 22.6 % | 4.2 % / 6.2 % |
| Hotwords, score 1 | 21.3 % / 21.3 % | 4.1 % / 6.1 % |
| Hotwords, score 1.5 (Parakeet only) | | 4.1 % / 6.2 % |
| Hotwords, score 2.5 | 24.9 % / 27.3 % | 4.5 % / 7.3 % |
| Hotwords, score 4 | 42.3 % / 45.6 % | 6.0 % / 10.5 % |

With 2.5 one word of the podcast came out as a hotword („Svelte“), with 4 a dozen („Juri“,
„Silero“); with 1 and 1.5 none. So only Parakeet decodes with the hotwords, at 1.5: as good as
greedy on the podcast. The German model stays greedy; beam search alone made it worse, and its live
text is replaced by Parakeet's after a German recording anyway (second pass, which favours the
words too). What a model writes a little off is put right afterwards when it sounds nearly the
same (`fixWords`: „Molly Ri“, „Mollire“ → „Moliri“, „Obeia“ → „Obeya“, „sherpa on X“ →
„sherpa-onnx“), cautiously: one sound in five may differ, words of fewer than five sounds only get
their spelling.


## German live: Kroko for the preview, Parakeet for the final text (October 2026)

With the language set to German, Live no longer uses the German FastConformer
(`internal/pipeline/livekroko.go`): Kroko's streaming Zipformer hears the speech as it comes, and
its running result is the preview; Parakeet's live stream (`liveStream`, as for any other
language, without its preview) runs beside it on the same audio in a goroutine of its own and gives
the final text, which replaces Kroko's words of that utterance. Its doubtful utterances (a cough, a
word or two it is unsure of, what sounds English) are checked with Kroko, as the German model
checked them before. Without Parakeet (not downloaded, or it fails to load), each stretch of speech
Kroko heard is a final of its own. Both models are downloaded on demand; Kroko is 54 MB.

`TestLiveBench` (`internal/pipeline/live_bench_test.go`, both excerpts together, WER clean /
headset, on a machine kept busy by other programs):

| | WER | Lines (ending a sentence) | Computing per second of audio |
| --- | --- | --- | --- |
| Before: the German FastConformer live | 19.5 % / 20.5 % | | |
| Kroko for the preview, Parakeet's live stream for the finals (now) | 4.9 % / 5.5 % | 42 (29) / 40 (25) | 0.6 s (both models) |
| Kroko alone (without Parakeet) | 10.0 % / 10.9 % | 29 (1) / 27 (3) | 0.2 s |
| The second pass after the recording (before) | 4.9 % / 5.5 % | | 0.45 s |

- The finals have the same words as the second pass, which ran Parakeet's live stream over the
  whole recording once it had ended: German notes no longer get it, and since every language now
  gets Parakeet's finals live, no note does. The second pass and its code have since been removed.
- The preview shows a word about 0.9 s after it began (90 % within 1.4 s), however long the
  utterance: the model decodes in chunks, and that is how long one takes to come. The 0.2 s in the
  table above is the longest compute of one decode, not this delay. The German FastConformer took
  0.4 s plus its decode (up to 1.7 s at the end of a long utterance), Parakeet up to 3.5 s.
- A first version cut the utterances itself, where the VAD ended speech and after Kroko's
  sentences, and gave each to Parakeet. Kroko trails the speech, so at the VAD's end it has not given
  its last words yet and cannot tell whether a sentence ended; its lines broke mid-sentence (6 of
  38 ended a sentence) and it reached 4.9 % / 6.1 % only once Parakeet heard a second after each cut
  (before: 6.2 % / 10.1 %, words at a cut came twice). Kroko's own endpoint ended utterances where
  the audio after them was not there yet; it is not used.
- The final text of a sentence comes a few seconds after it ended: the VAD's half second, up to a
  second's wait whether the speaker goes on, and Parakeet's decoding of the sentence with the 4 s
  before it (2 to 4 s on this busy machine). The preview shows Kroko's words meanwhile.
- Handing Parakeet's stream what had piled up at once (up to 2 s) when it fell behind made the
  headset recording worse (7.3 % instead of 5.5 %); it gets the audio in the recording's pieces.
- On this machine, kept at full load by other programs, a call of the stream (Kroko's decoding of
  100 ms) took a few seconds now and then, with Parakeet beside it and with Kroko alone; with the
  machine idle that was not measured.