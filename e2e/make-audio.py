# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

"""Speaks the test recordings in testdata/audio/ with the free Piper voices Thorsten and Kerstin.

Every phrase is synthesised, trimmed, stretched to the time the prepared results in testdata/results/
give it, and laid at that time, so the recordings fit the transcripts the tests read.

    python -m venv .venv && .venv/Scripts/pip install piper-tts
    .venv/Scripts/python make-audio.py <directory with the voice files>

The directory holds de_DE-thorsten-high.onnx and de_DE-kerstin-low.onnx with their .onnx.json, from
https://huggingface.co/rhasspy/piper-voices (de/de_DE/thorsten/high, de/de_DE/kerstin/low).
ffmpeg and ffprobe must be on the PATH.
"""

import os
import subprocess
import sys
import tempfile

# file and pace (Piper's length scale; Thorsten speaks fast, so less is left for atempo to stretch)
VOICES = {"kerstin": ("de_DE-kerstin-low.onnx", "1.0"), "thorsten": ("de_DE-thorsten-high.onnx", "1.25")}

# (voice, text, start, end): the times of the phrase's first and last word in the results.
INTERVIEW = [
    ("kerstin", "Guten Tag,", 0.93, 1.65),
    ("kerstin", "Herr Berger.", 2.17, 2.74),
    ("kerstin", "Sie haben vierzig Jahre in der Papierfabrik gearbeitet.", 3.46, 6.77),
    ("kerstin", "Wie hat das angefangen?", 7.49, 8.82),
    ("thorsten", "Angefangen hat es im Sommer neunzehnhundertsechzig.", 10.33, 13.52),
    ("thorsten", "Ich war fünfzehn und kam als Lehrling an die große Papiermaschine.", 14.27, 18.32),
    ("kerstin", "Was war Ihre erste Aufgabe?", 19.88, 21.59),
    ("thorsten", "Ich musste die Filze waschen und die Rollen beschriften.", 23.04, 26.03),
    ("thorsten", "Das war harte Arbeit.", 26.85, 28.31),
    ("thorsten", "Aber wir hatten viel Spaß.", 28.68, 30.42),
    ("kerstin", "Vielen Dank für das Gespräch.", 31.97, 33.62),
]

# Not transcribed yet: no results to fit, only a length close to the one the tests were written with.
NEW = [
    ("thorsten", "Notiz zum Rundgang am Donnerstag.", 0.6, None),
    ("thorsten", "Die Schöpfbütte im Erdgeschoss braucht ein neues Schild, der Text liegt beim Archiv.", 3.0, None),
]

RECORDINGS = {
    "Papierfabrik Interview.mp3": (INTERVIEW, 35.04),
    "Papierfabrik mit Korrekturen.mp3": (INTERVIEW, 35.04),
    "Neue Aufnahme.mp3": (NEW, 9.58),
}


def run(*args, **kw):
    return subprocess.run(args, check=True, capture_output=True, **kw)


def duration(path):
    out = run("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", path)
    return float(out.stdout)


def speak(voices, voice, text, out):
    """Synthesises text, trims the silence around it and writes 16 kHz mono."""
    raw = out + ".raw.wav"
    model, pace = VOICES[voice]
    run(sys.executable, "-m", "piper", "-m", os.path.join(voices, model), "--length-scale", pace, "-f", raw,
        input=text.encode())
    trim = "silenceremove=start_periods=1:start_threshold=-45dB,areverse,silenceremove=start_periods=1:start_threshold=-45dB,areverse"
    run("ffmpeg", "-y", "-v", "error", "-i", raw, "-af", trim, "-ar", "16000", "-ac", "1", out)


def tempo(factor):
    """atempo takes 0.5 to 2 per stage."""
    stages = []
    while factor > 2:
        stages.append(2.0)
        factor /= 2
    while factor < 0.5:
        stages.append(0.5)
        factor /= 0.5
    stages.append(factor)
    return ",".join(f"atempo={f:.4f}" for f in stages)


def build(voices, phrases, total, out, work):
    # a silent track of the full length first, so every phrase keeps its place in time
    inputs, filters = ["-f", "lavfi", "-t", str(total), "-i", "anullsrc=r=16000:cl=mono"], []
    for i, (voice, text, start, end) in enumerate(phrases, 1):
        wav = os.path.join(work, f"p{i}.wav")
        speak(voices, voice, text, wav)
        chain = []
        if end is not None:
            chain.append(tempo(duration(wav) / (end - start)))
        chain.append(f"adelay={round(start * 1000)}:all=1")
        inputs += ["-i", wav]
        filters.append(f"[{i}:a]{','.join(chain)}[a{i}]")
    mix = "[0:a]" + "".join(f"[a{i}]" for i in range(1, len(phrases) + 1))
    filters.append(f"{mix}amix=inputs={len(phrases) + 1}:duration=first:normalize=0[out]")
    run("ffmpeg", "-y", "-v", "error", *inputs, "-filter_complex", ";".join(filters), "-map", "[out]",
        "-ar", "16000", "-ac", "1", "-b:a", "48k", "-map_metadata", "-1", out)


def main():
    voices = sys.argv[1] if len(sys.argv) > 1 else "."
    target = os.path.join(os.path.dirname(os.path.abspath(__file__)), "testdata", "audio")
    with tempfile.TemporaryDirectory() as work:
        for name, (phrases, total) in RECORDINGS.items():
            out = os.path.join(target, name)
            build(voices, phrases, total, out, work)
            print(f"{name}: {duration(out):.2f} s")


if __name__ == "__main__":
    main()
