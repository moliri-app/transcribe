// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/moliri-app/transcribe/internal/prefs"
)

var verbs = regexp.MustCompile(`%(\[\d+\])?[sdv%]`)

// Every text the EXE shows through tx has its translation in every interface language, with the
// same placeholders; and every translation is of a text still in use.
func TestEveryTextTranslated(t *testing.T) {
	used := map[string]bool{}
	files, _ := filepath.Glob("*.go")
	fs := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fs, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); !ok || id.Name != "tx" || len(call.Args) == 0 {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok {
				t.Errorf("%s: tx takes a literal text", fs.Position(call.Pos()))
				return true
			}
			s, _ := strconv.Unquote(lit.Value)
			used[s] = true
			return true
		})
	}
	for en := range used {
		tr, ok := texts[en]
		if !ok {
			t.Errorf("no translations of %q", en)
			continue
		}
		want := verbs.FindAllString(en, -1)
		slices.Sort(want)
		for _, l := range prefs.UILocales[1:] {
			s := tr[l]
			if s == "" {
				t.Errorf("%q has no %s translation", en, l)
				continue
			}
			got := verbs.FindAllString(s, -1)
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Errorf("%q in %s has placeholders %v, not %v", en, l, got, want)
			}
		}
	}
	for en := range texts {
		if !used[en] {
			t.Errorf("%q is translated but not used", en)
		}
	}
}

// The interface language comes from the settings once the data dir is known; English for one the
// catalogue lacks.
func TestTextInInterfaceLanguage(t *testing.T) {
	d := t.TempDir()
	if _, err := prefs.Update(d, func(p *prefs.Prefs) { p.UILocale = "fr" }); err != nil {
		t.Fatal(err)
	}
	useLocaleOf(d)
	defer useLocaleOf("")
	if got := tx("Update to version %s", "1.2"); got != "Mettre à jour vers la version 1.2" {
		t.Errorf("fr: %q", got)
	}
	if got := tx("%s: done   Esc: cancel", keyLabel("ctrl+shift+space")); got != "Ctrl + Maj + Espace : terminer   Échap : annuler" {
		t.Errorf("fr hint: %q", got)
	}
	if got := txIn("en", "Downloading version %s … %d %%", "2.0", 40); got != "Downloading version 2.0 … 40 %" {
		t.Errorf("en: %q", got)
	}
	if got := txIn("pt", "Quit"); got != "Quit" {
		t.Errorf("unknown language: %q", got)
	}
}
