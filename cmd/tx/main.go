// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Command tx runs the recognition pipeline once on a file in <data>/audio (no server).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/moliri-app/transcribe/internal/runner"
	"github.com/moliri-app/transcribe/jobs"
)

func main() {
	data := flag.String("data", "", "data directory")
	preset := flag.String("preset", "auto", "auto|interview|gespraech|vortrag|musik")
	speakers := flag.Int("speakers", 0, "number of speakers (0 = auto)")
	lang := flag.String("language", "auto", "auto, a language code, or several like de,en")
	threads := flag.Int("threads", 4, "threads")
	flag.Parse()
	if *data == "" || flag.NArg() != 1 {
		log.Fatal("usage: tx -data DIR audiofile (a name inside DIR/audio)")
	}
	r := runner.New(*data)
	t0 := time.Now()
	err := r.Run(context.Background(), *data, flag.Arg(0), jobs.Options{Preset: *preset, Speakers: *speakers, Language: *lang, Threads: *threads},
		func(step, total int, label string) {
			fmt.Printf("[%d/%d] %s (%.0fs)\n", step, total, label, time.Since(t0).Seconds())
		})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("done in", time.Since(t0).Round(time.Second))
}
