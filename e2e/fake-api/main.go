// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Command fake-api serves the stand-in for speech recognition services
// (internal/sttapi/sttapitest) on a free local port and prints its address.
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/moliri-app/transcribe/internal/sttapi/sttapitest"
)

func main() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("API-Nachbau: http://%s\n", ln.Addr())
	log.Fatal(http.Serve(ln, (&sttapitest.Fake{}).Handler()))
}
