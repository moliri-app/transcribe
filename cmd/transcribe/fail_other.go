// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build !windows

package main

import "fmt"

func showError(msg string) { fmt.Println(msg) }

// confirm says msg and answers no: without a window there is no one to ask.
func confirm(msg string) bool {
	fmt.Println(msg)
	return false
}
