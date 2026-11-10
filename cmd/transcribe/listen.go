// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"net"
)

// listenLoopback listens on port at both loopback addresses, 127.0.0.1 and ::1, so that
// http://localhost:<port> reaches this app whichever of them the browser tries first (WebView2
// tries ::1). Listening on 127.0.0.1 alone is not enough on Windows: it succeeds even while
// another program serves the port on all addresses (python -m http.server 8765, say), and the
// window then opened that program's page over ::1. When port is taken at either address, it picks
// a free port instead. Without IPv6 it listens on 127.0.0.1 only.
func listenLoopback(port int) ([]net.Listener, error) {
	if ls, err := listenBoth(port); err == nil {
		return ls, nil
	}
	var err error
	for range 20 {
		var ls []net.Listener
		if ls, err = listenBoth(0); err == nil {
			return ls, nil
		}
	}
	return nil, err
}

// listenBoth listens on port (0: a free one) at 127.0.0.1 and, where the machine has IPv6, at
// ::1 on the same port.
func listenBoth(port int) ([]net.Listener, error) {
	l4, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, err
	}
	if !hasIPv6Loopback() {
		return []net.Listener{l4}, nil
	}
	l6, err := net.Listen("tcp6", fmt.Sprintf("[::1]:%d", l4.Addr().(*net.TCPAddr).Port))
	if err != nil {
		l4.Close()
		return nil, err
	}
	return []net.Listener{l4, l6}, nil
}

func hasIPv6Loopback() bool {
	l, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		return false
	}
	l.Close()
	return true
}
