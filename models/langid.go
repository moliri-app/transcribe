// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package models

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// LangIDEncoder returns a copy of the Whisper base encoder whose language identification only chooses
// among langs. sherpa-onnx picks the language with the highest score among the languages the encoder's
// metadata lists (all_language_tokens, all_language_codes); the copy lists only langs. Codes Whisper does
// not know are left out; fewer than two that it knows are an error, there is nothing to choose from.
// The copy is made once per set of languages, next to the other models.
func (s Set) LangIDEncoder(langs []string) (string, error) {
	src := s.Whisper("base").Encoder
	key := slices.Clone(langs)
	slices.Sort(key)
	key = slices.Compact(key)
	dst := s.p("whisper-base-langid", strings.Join(key, "-")+".onnx")
	if _, err := os.Stat(dst); err == nil {
		return dst, nil
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	out, err := restrictLanguages(b, key)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return "", err
	}
	return dst, os.Rename(tmp, dst)
}

// restrictLanguages rewrites the metadata of an ONNX model (a protobuf ModelProto) so that
// all_language_tokens and all_language_codes keep only the entries for langs. All other fields are
// copied byte for byte.
func restrictLanguages(model []byte, langs []string) ([]byte, error) {
	var tokens, codes []string
	tokIdx, codeIdx := -1, -1
	var fields [][]byte
	for b := model; len(b) > 0; {
		start := b
		tag, n := binary.Uvarint(b)
		if n <= 0 {
			return nil, errors.New("onnx: broken field tag")
		}
		b = b[n:]
		num, typ := int(tag>>3), int(tag&7)
		var body []byte
		switch typ {
		case 0:
			_, n = binary.Uvarint(b)
		case 1:
			n = 8
		case 5:
			n = 4
		case 2:
			l, k := binary.Uvarint(b)
			if k <= 0 || uint64(len(b)-k) < l {
				return nil, errors.New("onnx: broken field length")
			}
			body, n = b[k:k+int(l)], k+int(l)
		default:
			return nil, fmt.Errorf("onnx: unknown wire type %d", typ)
		}
		if n <= 0 || n > len(b) {
			return nil, errors.New("onnx: truncated field")
		}
		b = b[n:]
		fields = append(fields, start[:len(start)-len(b)])
		if num == 14 && typ == 2 { // metadata_props: StringStringEntryProto{key = 1, value = 2}
			k, v := entry(body)
			switch k {
			case "all_language_tokens":
				tokens, tokIdx = strings.Split(v, ","), len(fields)-1
			case "all_language_codes":
				codes, codeIdx = strings.Split(v, ","), len(fields)-1
			}
		}
	}
	if tokIdx < 0 || codeIdx < 0 || len(tokens) != len(codes) {
		return nil, errors.New("onnx: the model lists no languages to choose from")
	}
	var keepTok, keepCode []string
	for i, c := range codes {
		if slices.Contains(langs, c) {
			keepTok, keepCode = append(keepTok, tokens[i]), append(keepCode, c)
		}
	}
	if len(keepCode) < 2 {
		return nil, fmt.Errorf("Whisper kennt weniger als zwei der Sprachen %s", strings.Join(langs, ", "))
	}
	fields[tokIdx] = metaField("all_language_tokens", strings.Join(keepTok, ","))
	fields[codeIdx] = metaField("all_language_codes", strings.Join(keepCode, ","))
	out := make([]byte, 0, len(model))
	for _, f := range fields {
		out = append(out, f...)
	}
	return out, nil
}

// entry reads key and value of a StringStringEntryProto.
func entry(b []byte) (key, value string) {
	for len(b) > 0 {
		tag, n := binary.Uvarint(b)
		if n <= 0 || tag&7 != 2 {
			return
		}
		l, k := binary.Uvarint(b[n:])
		if k <= 0 || uint64(len(b)-n-k) < l {
			return
		}
		s := string(b[n+k : n+k+int(l)])
		switch tag >> 3 {
		case 1:
			key = s
		case 2:
			value = s
		}
		b = b[n+k+int(l):]
	}
	return
}

// metaField encodes metadata_props (field 14) with one key and value.
func metaField(key, value string) []byte {
	var e []byte
	e = appendBytes(e, 1, []byte(key))
	e = appendBytes(e, 2, []byte(value))
	return appendBytes(nil, 14, e)
}

func appendBytes(b []byte, num int, v []byte) []byte {
	b = binary.AppendUvarint(b, uint64(num)<<3|2)
	b = binary.AppendUvarint(b, uint64(len(v)))
	return append(b, v...)
}
