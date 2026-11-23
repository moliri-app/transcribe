// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package msg carries the app's messages to the interface in a form it can translate.
//
// The interface speaks Moliri's languages (frontend/src/locales); the texts the server makes
// (error messages, what a call is doing, the names of components) were German only. A message is
// therefore a stable key with its arguments, which the frontend turns into text in the interface
// language (frontend/src/lib/meldungen.js), together with the German text the API has always sent:
// programs that call the API (Moliri, curl) keep getting that, as `error` and the like, and the
// structured message travels beside it (`msg`, `errorMsg` …).
//
// A *M is an error too, so a package can return one where it returned errors.New, and the server
// finds it again with Of, also when it is wrapped.
package msg

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// M is a message: Key names it, Args fill the placeholders of its text.
type M struct {
	Key   string         `json:"key"`
	Args  map[string]any `json:"args,omitempty"`
	de    string
	cause error // the first error among the arguments, for errors.Is and errors.As
}

// New makes the message key with the German text de, whose placeholders {name} are filled from the
// pairs name, value in kv. A value may be a string, a number, another *M (translated on its own) or
// an error (a *M when it is or wraps one, else its text).
func New(key, de string, kv ...any) *M {
	m := &M{Key: key}
	if len(kv) > 0 {
		m.Args = make(map[string]any, len(kv)/2)
	}
	for i := 0; i+1 < len(kv); i += 2 {
		name, _ := kv[i].(string)
		v := kv[i+1]
		if err, ok := v.(error); ok {
			if m.cause == nil {
				m.cause = err
			}
			if inner := Of(err); inner != nil {
				v = inner
			} else {
				v = err.Error()
			}
		}
		m.Args[name] = v
		de = strings.ReplaceAll(de, "{"+name+"}", german(v))
	}
	m.de = strings.TrimSpace(de)
	return m
}

// german is an argument as the German text shows it (numbers with a decimal comma).
func german(v any) string {
	switch x := v.(type) {
	case *M:
		return x.de
	case float64:
		return strings.Replace(strconv.FormatFloat(x, 'f', -1, 64), ".", ",", 1)
	case float32:
		return german(float64(x))
	}
	return fmt.Sprint(v)
}

// Error is the German text, as the API has always sent it.
func (m *M) Error() string { return m.de }

// String is the German text.
func (m *M) String() string { return m.de }

// Unwrap is the error the message was made from, if any.
func (m *M) Unwrap() error { return m.cause }

// Carrier is an error of its own type that can say itself as a message.
type Carrier interface {
	error
	Msg() *M
}

// Of is the message err is or carries (Carrier), also wrapped as long as the wrapping adds no text
// of its own; nil when it carries none. Wrapped with more text (fmt.Errorf("…: %w", m)), the
// message would lose that text in the interface, so it counts as none.
func Of(err error) *M {
	if err == nil {
		return nil
	}
	var m *M
	if errors.As(err, &m) && m.de == err.Error() {
		return m
	}
	var c Carrier
	if errors.As(err, &c) && c.Error() == err.Error() {
		return c.Msg()
	}
	return nil
}
