// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ocr

import (
	"regexp"
	"strings"
)

// Score compares an OCR text with its ground truth the way the test set is measured (Moliri's
// server/transcribe/testdata/ocr/cer.py): character errors = edit distance / length of the ground
// truth, so text read in another order counts too; words found = share of the ground truth's words
// that occur in the text, in any order. Both texts are normalised first: line-end hyphens joined,
// long s as s, quotes unified, whitespace collapsed. Both results are 0–1.
func Score(groundTruth, text string) (charErrors, wordsFound float64) {
	gt, ocr := []rune(scoreNorm(groundTruth)), []rune(scoreNorm(text))
	if len(gt) == 0 {
		return 0, 1
	}
	charErrors = float64(editDistance(gt, ocr)) / float64(len(gt))

	have := map[string]int{}
	for _, w := range strings.Fields(string(ocr)) {
		have[w]++
	}
	words := strings.Fields(string(gt))
	found := 0
	for _, w := range words {
		if have[w] > 0 {
			have[w]--
			found++
		}
	}
	return charErrors, float64(found) / float64(max(1, len(words)))
}

var (
	lineEndHyphen = regexp.MustCompile(`[-=⸗¬]\s*\n\s*`)
	whitespace    = regexp.MustCompile(`\s+`)
	scoreQuotes   = strings.NewReplacer("ſ", "s", "“", `"`, "„", `"`, "’", "'")
)

func scoreNorm(t string) string {
	t = strings.ReplaceAll(t, "\r\n", "\n")
	t = lineEndHyphen.ReplaceAllString(t, "")
	t = scoreQuotes.Replace(t)
	return strings.TrimSpace(whitespace.ReplaceAllString(t, " "))
}

func editDistance(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
