// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package pipeline

import (
	"strconv"
	"strings"
)

var deUnits = map[string]int{
	"null": 0, "eins": 1, "ein": 1, "zwei": 2, "drei": 3, "vier": 4, "fünf": 5, "sechs": 6, "sieben": 7,
	"acht": 8, "neun": 9, "zehn": 10, "elf": 11, "zwölf": 12, "dreizehn": 13, "vierzehn": 14,
	"fünfzehn": 15, "sechzehn": 16, "siebzehn": 17, "achtzehn": 18, "neunzehn": 19,
}

var deTens = map[string]int{
	"zwanzig": 20, "dreißig": 30, "vierzig": 40, "fünfzig": 50, "sechzig": 60, "siebzig": 70, "achtzig": 80, "neunzig": 90,
}

// below100 parses "", "sieben", "zwanzig", "einundzwanzig". ok=false when it is not a number word.
func below100(s string) (int, bool) {
	if v, ok := deUnits[s]; ok {
		return v, true
	}
	if v, ok := deTens[s]; ok {
		return v, true
	}
	if i := strings.Index(s, "und"); i > 0 {
		u, ok1 := deUnits[s[:i]]
		t, ok2 := deTens[s[i+3:]]
		if ok1 && ok2 && u >= 1 && u <= 9 {
			return u + t, true
		}
	}
	return 0, false
}

// below1000 also accepts years like "neunzehnhundertsechsunddreißig" (teens times hundred).
func below1000(s string) (int, bool) {
	if i := strings.Index(s, "hundert"); i >= 0 {
		head := 1
		if i > 0 {
			v, ok := deUnits[s[:i]]
			if !ok {
				return 0, false
			}
			head = v
		}
		rest := s[i+len("hundert"):]
		if rest == "" {
			return head * 100, true
		}
		r, ok := below100(rest)
		return head*100 + r, ok
	}
	return below100(s)
}

func parseGermanNumber(s string) (int, bool) {
	s = strings.ToLower(s)
	if i := strings.Index(s, "tausend"); i >= 0 {
		head := 1
		if i > 0 {
			v, ok := below1000(s[:i])
			if !ok {
				return 0, false
			}
			head = v
		}
		rest := s[i+len("tausend"):]
		if rest == "" {
			return head * 1000, true
		}
		r, ok := below1000(rest)
		return head*1000 + r, ok
	}
	return below1000(s)
}

const punct = ".,!?;:»«\"'()"

// toDigits turns 'neunzehnhundertsechsunddreißig,' into '1936,'. Lone "ein"/"eins" stay words.
func toDigits(word string) string {
	core := strings.Trim(word, punct)
	if core == "" || strings.EqualFold(core, "ein") || strings.EqualFold(core, "eins") {
		return word
	}
	v, ok := parseGermanNumber(core)
	if !ok {
		return word
	}
	return strings.Replace(word, core, strconv.Itoa(v), 1)
}
