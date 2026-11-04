// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package update

import (
	"strconv"
	"strings"
)

// version is a semantic version: v1.2.3 or 1.2.3, optionally with a pre-release (-rc.1);
// build metadata (+…) is ignored.
type version struct {
	num [3]int
	pre []string // empty for a release
}

func parse(s string) (version, bool) {
	var v version
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	s, _, _ = strings.Cut(s, "+")
	s, pre, hasPre := strings.Cut(s, "-")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p == "" || (len(p) > 1 && p[0] == '0') {
			return v, false
		}
		v.num[i] = n
	}
	if hasPre {
		if pre == "" {
			return v, false
		}
		v.pre = strings.Split(pre, ".")
	}
	return v, true
}

// IsRelease reports whether v is the version of a release build (v1.2.3, no pre-release). Only
// those look for updates: a development build ("dev-…", or none at all) never does.
func IsRelease(v string) bool {
	p, ok := parse(v)
	return ok && len(p.pre) == 0
}

// Compare orders two versions by semantic versioning: -1 if a is older than b, 0 if they are the
// same, 1 if a is newer. A version that does not parse is older than any that does.
func Compare(a, b string) int {
	va, oka := parse(a)
	vb, okb := parse(b)
	switch {
	case !oka && !okb:
		return 0
	case !oka:
		return -1
	case !okb:
		return 1
	}
	for i := range va.num {
		if c := cmpInt(va.num[i], vb.num[i]); c != 0 {
			return c
		}
	}
	// a pre-release comes before its release
	switch {
	case len(va.pre) == 0 && len(vb.pre) == 0:
		return 0
	case len(va.pre) == 0:
		return 1
	case len(vb.pre) == 0:
		return -1
	}
	for i := 0; i < len(va.pre) && i < len(vb.pre); i++ {
		x, y := va.pre[i], vb.pre[i]
		nx, ex := strconv.Atoi(x)
		ny, ey := strconv.Atoi(y)
		var c int
		switch {
		case ex == nil && ey == nil:
			c = cmpInt(nx, ny)
		case ex == nil: // numeric identifiers come before alphanumeric ones
			c = -1
		case ey == nil:
			c = 1
		default:
			c = strings.Compare(x, y)
		}
		if c != 0 {
			return c
		}
	}
	return cmpInt(len(va.pre), len(vb.pre))
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
