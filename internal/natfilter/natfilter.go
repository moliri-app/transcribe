// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package natfilter turns a sentence into the conditions of a filter over Moliri's objects
// (docs/filter-api.md): »Gemälde nach 1900 ohne Standort« with the project's field catalog goes
// to the language model, which answers with a field, a comparison and a value per condition.
//
// The prompt and the check of the answer are those of Moliri's server
// (moliri-sveltekit static/api/_func/natural_filter.php), so that a question asked here gets
// the answer it would get from the instance's AI account with the same model. A change to the
// prompt belongs in both places. Moliri's detail search checks the answer once more against its
// own catalog before it shows it.
package natfilter

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// MaxText, MaxCatalog: the longest sentence and catalog text (characters); MaxFields: the
	// most field names.
	MaxText    = 500
	MaxCatalog = 60000
	MaxFields  = 1500
	// MaxConditions taken from an answer.
	MaxConditions = 8
	// MaxTokens of an answer.
	MaxTokens = 600
)

// Comparisons the model may name. Words rather than the filter's operators: a model reads
// »empty« more reliably than »= with an empty text«. Moliri maps them to the rows.
var Comparisons = []string{
	"contains", "not_contains", "equals", "not_equals",
	"greater", "less", "greater_or_equal", "less_or_equal",
	"empty", "not_empty", "has_link", "has_no_link", "violated", "not_violated",
}

// Input is a question: the sentence, the catalog as text (written by Moliri's context source)
// and every field name the answer may use.
type Input struct {
	Text    string   `json:"text"`
	Catalog string   `json:"catalog"`
	Fields  []string `json:"fields"`
}

var spaces = regexp.MustCompile(`\s+`)

// Clean trims and caps a question; Fields come back without empty, too long and doubled names.
func Clean(in Input) Input {
	out := Input{
		Text:    cut(strings.TrimSpace(spaces.ReplaceAllString(in.Text, " ")), MaxText),
		Catalog: cut(strings.TrimSpace(in.Catalog), MaxCatalog),
	}
	seen := map[string]bool{}
	for i, name := range in.Fields {
		if i >= MaxFields {
			break
		}
		name = strings.TrimSpace(name)
		if name == "" || utf8.RuneCountInString(name) > 200 || seen[name] {
			continue
		}
		seen[name] = true
		out.Fields = append(out.Fields, name)
	}
	return out
}

// cut keeps the first max characters of s.
func cut(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max])
}

// Message is one turn of the prompt.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// System is the first message.
const System = "Du bist eine sorgfältige Suchhilfe für eine Museumsdatenbank. Du antwortest nur mit JSON."

// Messages is the prompt: the rules, the catalog, the sentence. The sentence stands last, so
// the model reads only it again on the next question about the same catalog. The keys are named
// in the text too: held to a schema, qwen2.5 7B left the value of »nach 1900« empty.
func Messages(text, catalog string) []Message {
	prompt := strings.Join([]string{
		"Übersetze die Suchanfrage in die Bedingungen eines Filters über die Objekte einer Museumssammlung.",
		"",
		"Regeln:",
		"- field: nur Feldnamen aus dem Feldkatalog unten, genau so geschrieben wie dort in `…` (ohne die Backticks). Unterfelder heißen `Feld~unterfeld`.",
		"- comparison: contains (enthält), not_contains (enthält nicht), equals (ist gleich), not_equals (ist nicht),",
		"  greater, less, greater_or_equal, less_or_equal (für Zahlen, Jahre und Daten),",
		"  empty (Feld ist leer, „ohne …“, „fehlt“), not_empty (Feld ist ausgefüllt, „mit …“).",
		"  Für Verknüpfungsarten (`__links.…`) nur has_link oder has_no_link, für Prüfungen (`__conditions…`) nur violated oder not_violated.",
		"- value: der gesuchte Text, die Zahl oder das Datum (JJJJ-MM-TT); leer bei empty, not_empty, has_link, has_no_link, violated, not_violated.",
		"  Hat ein Feld gespeicherte Werte, nimm den passenden davon, auch wenn er in einer anderen Sprache ist als die Anfrage.",
		"  Jahreszahlen bei Zahlenfeldern als Zahl: „nach 1900“ = greater 1900, „vor 1800“ = less 1800.",
		"- join: AND oder OR, womit die Bedingung an die vorige anschließt; bei der ersten AND.",
		"- not_understood: die Teile der Anfrage, die sich mit diesen Feldern nicht ausdrücken lassen, in den Worten der Anfrage. Sonst leer.",
		"- Nur Bedingungen, die die Anfrage verlangt. Keine erfundenen Felder.",
		"",
		catalog,
		"",
		"Suchanfrage: „" + text + "“",
		"",
		`Antworte nur mit einem JSON-Objekt: {"conditions": [{"join": "AND", "field": "<Feldname>", "comparison": "<Vergleich>", "value": "<Wert>"}, …], "not_understood": ["…"]}`,
	}, "\n")
	return []Message{{Role: "system", Content: System}, {Role: "user", Content: prompt}}
}

// Prefix is the part of the prompt that questions about the same catalog share: what the model
// read of it can be kept.
func Prefix(catalog string) string {
	m := Messages("", catalog)
	return m[0].Content + "\n" + m[1].Content[:strings.LastIndex(m[1].Content, "Suchanfrage: „")]
}

// Condition is one condition of the answer.
type Condition struct {
	Join       string `json:"join"`
	Field      string `json:"field"`
	Comparison string `json:"comparison"`
	Value      string `json:"value"`
}

// Result is the checked answer. Dropped: the field names the model made up.
type Result struct {
	Conditions    []Condition `json:"conditions"`
	NotUnderstood []string    `json:"not_understood"`
	Dropped       []string    `json:"dropped"`
}

// Parse checks the model's answer: only fields of the question and comparisons of the list stay.
// ok is false when the answer holds no JSON object.
func Parse(content string, fields []string) (Result, bool) {
	decoded, ok := decodeObject(content)
	if !ok {
		return Result{}, false
	}
	known := map[string]bool{}
	for _, f := range fields {
		known[f] = true
	}
	comparisons := map[string]bool{}
	for _, c := range Comparisons {
		comparisons[c] = true
	}
	res := Result{Conditions: []Condition{}, NotUnderstood: []string{}, Dropped: []string{}}
	dropped := map[string]bool{}
	conds, _ := decoded["conditions"].([]any)
	for _, c := range conds {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}
		field := strings.Trim(str(m["field"], 200), " `\"'„“")
		comparison := strings.ToLower(str(m["comparison"], 40))
		if field == "" {
			continue
		}
		if !known[field] {
			if !dropped[field] {
				dropped[field] = true
				res.Dropped = append(res.Dropped, field)
			}
			continue
		}
		if !comparisons[comparison] {
			continue
		}
		join := "AND"
		if strings.ToUpper(str(m["join"], 3)) == "OR" {
			join = "OR"
		}
		res.Conditions = append(res.Conditions, Condition{Join: join, Field: field, Comparison: comparison, Value: str(m["value"], 300)})
		if len(res.Conditions) >= MaxConditions {
			break
		}
	}
	notUnderstood, _ := decoded["not_understood"].([]any)
	if s, ok := decoded["not_understood"].(string); ok {
		notUnderstood = []any{s}
	}
	for _, n := range notUnderstood {
		if s := str(n, 200); s != "" && len(res.NotUnderstood) < 10 {
			res.NotUnderstood = append(res.NotUnderstood, s)
		}
	}
	return res, true
}

// str is a scalar as trimmed text of at most max characters, as PHP's (string) makes it.
func str(v any, max int) string {
	var s string
	switch t := v.(type) {
	case string:
		s = t
	case json.Number:
		s = t.String()
	case bool:
		if t {
			s = "1"
		}
	}
	return cut(strings.TrimSpace(s), max)
}

// decodeObject reads the first JSON object of an answer; some models wrap it in prose or a code
// fence.
func decodeObject(content string) (map[string]any, bool) {
	try := func(s string) (map[string]any, bool) {
		d := json.NewDecoder(strings.NewReader(s))
		d.UseNumber()
		var m map[string]any
		if d.Decode(&m) != nil || m == nil {
			return nil, false
		}
		return m, true
	}
	if m, ok := try(strings.TrimSpace(content)); ok {
		return m, true
	}
	start, end := strings.Index(content, "{"), strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return nil, false
	}
	return try(content[start : end+1])
}
