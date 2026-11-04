// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package annotate

import "strings"

// whisperLangs are the language codes Whisper can be forced to when a passage is recognised again.
// The editor keeps the same list in frontend/src/lib/editor/sprachen.js to tell which languages are
// only noted and which are recognised again.
var whisperLangs = map[string]bool{}

func init() {
	for _, c := range strings.Fields(`en zh de es ru ko fr ja pt tr pl ca nl ar sv it id hi fi vi he uk el ms cs
		ro da hu ta no th ur hr bg lt la mi ml cy sk te fa lv bn sr az sl kn et mk br eu is hy ne mn bs kk sq
		sw gl mr pa si km sn yo so af oc ka be tg sd gu am yi lo uz fo ht ps tk nn mt sa lb my bo tl mg as tt
		haw ln ha ba jw su yue`) {
		whisperLangs[c] = true
	}
}

// Recognizable reports whether a passage set to lang can be recognised again in that language. Other
// codes (a dialect such as nds for Low German) stay a human note: the words of the machine are kept.
func Recognizable(lang string) bool { return whisperLangs[lang] }
