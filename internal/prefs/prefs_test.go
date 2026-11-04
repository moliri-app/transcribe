// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package prefs

import "testing"

func TestRecognition(t *testing.T) {
	dir := t.TempDir()
	if p := Load(dir); p.Recognition != Local || p.Engine("", p.FileEngine) != Local || !p.UsesModels() {
		t.Errorf("default: %+v", p)
	}
	// only the external service: everything goes there, whatever is asked
	p, _ := Update(dir, func(p *Prefs) { p.Recognition, p.LiveEngine = API, Local })
	if p.LiveEngine != API || p.FileEngine != API || p.Engine(Local, p.FileEngine) != API || p.UsesModels() {
		t.Errorf("api: %+v", p)
	}
	// both: the defaults stand as chosen, a job may ask for either
	p, _ = Update(dir, func(p *Prefs) { p.Recognition, p.LiveEngine, p.FileEngine = Both, Local, "nonsense" })
	if p.LiveEngine != Local || p.FileEngine != Local || p.Engine(API, p.FileEngine) != API || p.Engine("", p.FileEngine) != Local {
		t.Errorf("both: %+v", p)
	}
	p, _ = Update(dir, func(p *Prefs) { p.Recognition = "nonsense" })
	if p.Recognition != Local || p.Engine(API, p.FileEngine) != Local {
		t.Errorf("unknown: %+v", p)
	}
}
