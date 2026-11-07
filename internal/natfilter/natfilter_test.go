// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package natfilter

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// The cases of Moliri's tests/php/natural_filter.test.php: both places check alike.

func TestClean(t *testing.T) {
	c := Clean(Input{Text: "  Gemälde\n nach 1900 ", Catalog: " - `Objektart` ", Fields: []string{"Objektart", "", "Objektart", "Datierung von"}})
	if c.Text != "Gemälde nach 1900" || c.Catalog != "- `Objektart`" || !reflect.DeepEqual(c.Fields, []string{"Objektart", "Datierung von"}) {
		t.Errorf("%+v", c)
	}
	if n := utf8.RuneCountInString(Clean(Input{Text: strings.Repeat("ä", 900)}).Text); n != MaxText {
		t.Errorf("text capped at %d", n)
	}
}

func TestParse(t *testing.T) {
	fields := []string{"Objektart", "Datierung von", "Dauerstandort"}
	res, ok := Parse(`{"conditions": [
		{"join": "AND", "field": "Objektart", "comparison": "equals", "value": "Painting"},
		{"join": "and", "field": "`+"`Datierung von`"+`", "comparison": "greater", "value": 1900},
		{"join": "OR", "field": "Dauerstandort", "comparison": "EMPTY", "value": ""},
		{"join": "AND", "field": "Künstler", "comparison": "contains", "value": "Monet"},
		{"join": "AND", "field": "Objektart", "comparison": "between", "value": "1"}
	], "not_understood": ["aus Frankreich", ""]}`, fields)
	if !ok {
		t.Fatal("not read")
	}
	want := []Condition{
		{"AND", "Objektart", "equals", "Painting"},
		{"AND", "Datierung von", "greater", "1900"},
		{"OR", "Dauerstandort", "empty", ""},
	}
	if !reflect.DeepEqual(res.Conditions, want) {
		t.Errorf("conditions %+v", res.Conditions)
	}
	if !reflect.DeepEqual(res.Dropped, []string{"Künstler"}) || !reflect.DeepEqual(res.NotUnderstood, []string{"aus Frankreich"}) {
		t.Errorf("dropped %v, not understood %v", res.Dropped, res.NotUnderstood)
	}
	if res, ok := Parse("```json\n{\"conditions\": [{\"field\": \"Objektart\", \"comparison\": \"equals\", \"value\": \"X\"}]}\n```", fields); !ok || len(res.Conditions) != 1 {
		t.Errorf("code fence: %v %+v", ok, res)
	}
	if _, ok := Parse("Ich weiß es nicht.", fields); ok {
		t.Error("no JSON read as an answer")
	}
}

func TestMessages(t *testing.T) {
	m := Messages("Gemälde ohne Standort", "Feldkatalog:\n- `Objektart`")
	if len(m) != 2 || m[0].Role != "system" || strings.LastIndex(m[1].Content, "Gemälde ohne Standort") < strings.Index(m[1].Content, "Feldkatalog") {
		t.Errorf("%+v", m)
	}
	// What questions about one catalog share is the beginning of each of them.
	p := Prefix("Feldkatalog:\n- `Objektart`")
	if full := m[0].Content + "\n" + m[1].Content; !strings.HasPrefix(full, p) || strings.Contains(p, "Gemälde") {
		t.Errorf("prefix %q", p)
	}
}
