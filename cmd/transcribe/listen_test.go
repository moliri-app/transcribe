// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

// Another program serving the app's port on all addresses (python -m http.server 8765) must not
// get the app's window: localhost has to reach the app at 127.0.0.1 and at ::1 alike.
func TestLocalhostReachesTheAppWhenAnotherServerHoldsThePort(t *testing.T) {
	other, err := net.Listen("tcp", ":0") // all addresses, IPv4 and IPv6
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	go http.Serve(other, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "other")
	}))
	port := other.Addr().(*net.TCPAddr).Port

	lns, err := listenLoopback(port)
	if err != nil {
		t.Fatal(err)
	}
	app := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "app")
	})}
	defer app.Close()
	for _, ln := range lns {
		go app.Serve(ln)
	}
	got := lns[0].Addr().(*net.TCPAddr).Port

	hosts := []string{"127.0.0.1"}
	if hasIPv6Loopback() {
		hosts = append(hosts, "[::1]")
	}
	c := http.Client{Timeout: 3 * time.Second}
	for _, h := range hosts {
		r, err := c.Get(fmt.Sprintf("http://%s:%d/", h, got))
		if err != nil {
			t.Fatalf("%s: %v", h, err)
		}
		b := make([]byte, 16)
		n, _ := r.Body.Read(b)
		r.Body.Close()
		if string(b[:n]) != "app" {
			t.Errorf("%s:%d answered %q, want the app", h, got, b[:n])
		}
	}
}
