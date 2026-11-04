// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package sttapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// The OpenAI-compatible transcription API: POST <base>/audio/transcriptions with the audio and
// the model, answered with the text and, where the service can (verbose_json), segments and words
// with their times. Speakers it does not tell apart.

type oaWord struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type oaSegment struct {
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Text       string  `json:"text"`
	AvgLogprob float64 `json:"avg_logprob"`
}

type oaResult struct {
	Text     string      `json:"text"`
	Language string      `json:"language"`
	Duration float64     `json:"duration"`
	Words    []oaWord    `json:"words"`
	Segments []oaSegment `json:"segments"`
}

func openAITranscribe(ctx context.Context, c Config, audio io.Reader, name string, o Request) (*Result, error) {
	// the audio may be needed twice: for services that refuse the detailed answer
	b, err := io.ReadAll(audio)
	if err != nil {
		return nil, err
	}
	hdr := http.Header{}
	if c.Key != "" {
		hdr.Set("Authorization", "Bearer "+c.Key)
	}
	fields := map[string]string{
		"model":                     c.Model,
		"response_format":           "verbose_json",
		"timestamp_granularities[]": "word\x00segment",
	}
	if o.Language != "" {
		fields["language"] = o.Language
	}
	target := c.base() + "/audio/transcriptions"
	body, err := post(ctx, target, hdr, fields, bytes.NewReader(b), name, c.Label())
	var refused *apiError
	if errors.As(err, &refused) && (refused.status == http.StatusBadRequest || refused.status == http.StatusUnprocessableEntity) {
		// models like gpt-4o-transcribe know only the plain answer: the text alone
		delete(fields, "timestamp_granularities[]")
		fields["response_format"] = "json"
		body, err = post(ctx, target, hdr, fields, bytes.NewReader(b), name, c.Label())
	}
	if err != nil {
		return nil, err
	}
	var r oaResult
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("%s: unerwartete Antwort: %w", c.Label(), err)
	}
	res := &Result{Language: shortLang(r.Language), Duration: r.Duration}
	if res.Duration == 0 {
		res.Duration = o.Length
	}
	// how sure the service is: per segment where it says so
	sure := func(t float64) float64 {
		for _, s := range r.Segments {
			if s.AvgLogprob != 0 && t >= s.Start && t <= s.End {
				return prob(s.AvgLogprob)
			}
		}
		return 0.9
	}
	switch {
	case len(r.Words) > 0:
		for _, w := range r.Words {
			if t := strings.TrimSpace(w.Word); t != "" {
				res.Words = append(res.Words, Word{Text: t, Start: w.Start, End: w.End, P: sure((w.Start + w.End) / 2)})
			}
		}
		punctuate(res.Words, r.Text)
	case len(r.Segments) > 0:
		for _, s := range r.Segments {
			res.Words = append(res.Words, spread(s.Text, s.Start, s.End, sure((s.Start+s.End)/2))...)
		}
	case strings.TrimSpace(r.Text) != "":
		res.Words = spread(r.Text, 0, res.Duration, 0.9)
	}
	return res, nil
}

// punctuate gives words the punctuation the full text has: OpenAI's word list leaves it out.
func punctuate(words []Word, text string) {
	fields := strings.Fields(text)
	i := 0
	for k := range words {
		bare := strings.ToLower(strings.Trim(words[k].Text, ".,;:!?…\"'«»„“”"))
		for j := i; j < len(fields) && j < i+4; j++ {
			if strings.ToLower(strings.Trim(fields[j], ".,;:!?…\"'«»„“”")) == bare {
				words[k].Text, i = fields[j], j+1
				break
			}
		}
	}
}
