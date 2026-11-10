// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package runner

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/moliri-app/transcribe/audio"
	"github.com/moliri-app/transcribe/fileutil"
	"github.com/moliri-app/transcribe/internal/media"
	"github.com/moliri-app/transcribe/internal/prefs"
	"github.com/moliri-app/transcribe/internal/results"
	"github.com/moliri-app/transcribe/internal/sttapi"
	"github.com/moliri-app/transcribe/jobs"
	"github.com/moliri-app/transcribe/model"
	"github.com/moliri-app/transcribe/pipeline"
)

// Recognition by a service (internal/sttapi): what comes back (words with times, speakers and
// sounds where the service tells them) becomes _machine.json. No model is loaded on this computer.

// apiEvents turns the sounds a service tags into the labels the local event detection shows.
var apiEvents = map[string]string{
	"music": "Musik", "singing": "Gesang", "applause": "Applaus", "clapping": "Applaus",
	"laughter": "Lachen", "laughing": "Lachen", "laughs": "Lachen", "chuckles": "Lachen", "giggles": "Lachen",
	"cheering": "Jubel", "crowd": "Menschenmenge", "chatter": "Stimmengewirr", "shouting": "Ruf",
	"screaming": "Schrei", "phone ringing": "Telefon", "dog barking": "Hund", "bell": "Glocke", "whistling": "Pfeifen",
}

// apiConfig is the service chosen in the settings, with its own address and model where none is given.
func apiConfig(p prefs.Prefs) sttapi.Config {
	return sttapi.Config{Provider: p.API.Provider, URL: p.API.URL, Model: p.API.Model, Key: p.API.Key}.Filled()
}

func (r *Runner) apiConfig() sttapi.Config { return apiConfig(prefs.Load(r.DataDir)) }

// runAPI is Run with the service.
func (r *Runner) runAPI(ctx context.Context, dataDir, audioFile string, o jobs.Options, p jobs.Progress) error {
	n := pipeline.Speakers(o)
	lang := o.Language
	if lang == "" {
		lang = "auto"
	}
	c := r.apiConfig()
	if err := c.Usable(); err != nil {
		return err
	}
	req := sttapi.Request{Speakers: n, Diarize: n != 1}
	// several languages: the service recognises them itself, as it does with auto
	if langs := jobs.Languages(lang); len(langs) == 1 {
		req.Language = langs[0]
	}
	if p != nil {
		p(1, 2, "transcribe ("+c.Label()+")")
	}
	path := media.Path(dataDir, audioFile)
	var res *sttapi.Result
	var err error
	if c.WholeFile() {
		res, err = apiWhole(ctx, c, path, filepath.Base(path), req)
	} else {
		res, err = apiPieces(ctx, c, path, req)
	}
	if err != nil {
		return err
	}

	stem := strings.TrimSuffix(audioFile, filepath.Ext(audioFile))
	outDir := filepath.Join(dataDir, "results", stem)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	segs, events := apiTranscript(res, n == 1, lang)
	duration := res.Duration
	if len(segs) > 0 {
		duration = math.Max(duration, segs[len(segs)-1].End)
	}
	var given *int
	if n > 0 {
		given = &n
	}
	var spkVariant *string
	if req.Diarize && c.Speakers() {
		v := "api-" + c.Provider + "-diarize"
		spkVariant = &v
	}
	mach := model.Machine{Audio: audioFile, Preset: o.Preset, SpeakersGiven: given, Quality: "best", Language: lang,
		EngineUsed: "api-" + c.Provider + "~" + c.Model, SpeakersVariant: spkVariant, EventsVariant: "api-" + c.Provider,
		Duration: round(duration, 2), Segments: segs, Events: events}
	if err := fileutil.WriteJSON(filepath.Join(outDir, "_machine.json"), mach); err != nil {
		return err
	}
	if p != nil {
		p(2, 2, "annotate")
	}
	return results.Apply(ctx, dataDir, stem, apiRetranscriber{r})
}

// apiWhole sends the recording as it is.
func apiWhole(ctx context.Context, c sttapi.Config, path, name string, req sttapi.Request) (*sttapi.Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return sttapi.Transcribe(ctx, c, f, name, req)
}

// apiPieces sends the recording as WAV pieces cut at pauses, one after the other, and puts the
// words together.
func apiPieces(ctx context.Context, c sttapi.Config, path string, req sttapi.Request) (*sttapi.Result, error) {
	pcm, err := audio.Decode(ctx, path)
	if err != nil {
		return nil, err
	}
	if err := pipeline.TooShort(pcm); err != nil {
		return nil, err
	}
	dur := float64(len(pcm)) / audio.SR
	cuts := []float64{0}
	for dur-cuts[len(cuts)-1] > sttapi.MaxPiece {
		cuts = append(cuts, pipeline.SnapToPause(pcm, cuts[len(cuts)-1]+sttapi.MaxPiece-10, 10))
	}
	cuts = append(cuts, dur)
	all := &sttapi.Result{Duration: dur}
	for i := 0; i+1 < len(cuts); i++ {
		if err := jobs.Hold(ctx); err != nil {
			return nil, err
		}
		a, b := cuts[i], cuts[i+1]
		req.Length = b - a
		res, err := sttapi.Transcribe(ctx, c, bytes.NewReader(sttapi.WAV(audio.Slice(pcm, a, b))), fmt.Sprintf("teil-%d.wav", i+1), req)
		if err != nil {
			return nil, err
		}
		if all.Language == "" {
			all.Language = res.Language
		}
		for _, w := range res.Words {
			w.Start, w.End = a+w.Start, a+w.End
			all.Words = append(all.Words, w)
		}
	}
	return all, nil
}

// apiTranscript cuts the service's words into segments: at a change of speaker, a long pause, or
// where a sentence ends with a short pause after it or the segment is long enough.
func apiTranscript(res *sttapi.Result, oneSpeaker bool, lang string) ([]model.Segment, []model.Event) {
	lid := ""
	if l := res.Language; l != "" && lang == "auto" && l != "de" {
		lid = l // like the local pipeline, which takes German as given and marks what is not
	}
	speakers := map[string]int{}
	speakerOf := func(id string) int {
		if oneSpeaker || id == "" {
			return 0
		}
		if n, ok := speakers[id]; ok {
			return n
		}
		n := len(speakers)
		if i, err := strconv.Atoi(strings.TrimPrefix(id, "speaker_")); err == nil && i >= 0 && i < 100 {
			n = i
		}
		speakers[id] = n
		return n
	}
	var segs []model.Segment
	var events []model.Event
	var cur *model.Segment
	end := func() {
		if cur != nil && len(cur.Words) > 0 {
			cur.Text = pipeline.JoinWords(cur.Words)
			segs = append(segs, *cur)
		}
		cur = nil
	}
	for _, w := range res.Words {
		if w.Event {
			if label, ok := apiEvents[strings.ToLower(w.Text)]; ok && w.End > w.Start {
				events = append(events, model.Event{Label: label, Start: round(w.Start, 2), End: round(w.End, 2)})
			}
			continue
		}
		spk := speakerOf(w.Speaker)
		if cur != nil {
			last := cur.Words[len(cur.Words)-1]
			sentence := strings.ContainsAny(last.W[len(last.W)-1:], ".!?")
			pause := w.Start - last.End
			if *last.Speaker != spk || pause >= 0.8 || sentence && (pause >= 0.3 || w.Start-cur.Start > 15) || w.Start-cur.Start > 25 {
				end()
			}
		}
		if cur == nil {
			cur = &model.Segment{Start: round(w.Start, 3)}
		}
		s := spk
		cur.Words = append(cur.Words, model.Word{W: w.Text, Start: round(w.Start, 3), End: round(w.End, 3),
			P: round(w.P, 3), Speaker: &s, Lid: lid})
		cur.End = round(w.End, 3)
	}
	end()
	return segs, events
}

// apiRetranscriber recognises passages again with the service.
type apiRetranscriber struct{ r *Runner }

func (a apiRetranscriber) Retranscribe(ctx context.Context, audioPath string, from, to float64, lang string) ([]model.Word, error) {
	c := a.r.apiConfig()
	if err := c.Usable(); err != nil {
		return nil, err
	}
	pcm, err := audio.Decode(ctx, audioPath)
	if err != nil {
		return nil, err
	}
	x := audio.Slice(pcm, from, to)
	if len(x) < audio.SR/10 {
		return nil, nil
	}
	res, err := sttapi.Transcribe(ctx, c, bytes.NewReader(sttapi.WAV(x)), "stelle.wav", sttapi.Request{Language: lang, Length: to - from})
	if err != nil {
		return nil, err
	}
	var out []model.Word
	for _, w := range res.Words {
		if !w.Event {
			out = append(out, model.Word{W: w.Text, Start: round(from+w.Start, 3), End: round(from+w.End, 3),
				P: round(w.P, 3), Lang: lang, Redone: true})
		}
	}
	return out, nil
}

func round(v float64, d int) float64 {
	p := math.Pow10(d)
	return math.Round(v*p) / p
}
