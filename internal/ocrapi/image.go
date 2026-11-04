// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ocrapi

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"math"
	"os"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"github.com/moliri-app/transcribe/ocr"
)

// MaxSide is the longest side of the image sent, as Moliri sends it (measured in Moliri's
// docs/ki-einsatz-skizze.md §2.5: larger costs more and reads no better).
const MaxSide = 1600

// Size is the image read (Width, Height: the boxes of the result are in its pixels) and the image
// sent (a model that answers in pixels answers in those).
type Size struct {
	Width, Height         int
	SentWidth, SentHeight int
}

// Prepared is the image as it goes to the service.
type Prepared struct {
	Mime string
	Data []byte // the image file, JPEG
	Size Size
}

// PrepareImage reads the image at path and scales it so its longest side is at most maxSide, on
// white (transparent parts are paper), as JPEG of quality 90. Unreadable images wrap ocr.ErrImage.
func PrepareImage(path string, maxSide int) (*Prepared, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ocr.ErrImage, err)
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("%w: empty image", ocr.ErrImage)
	}
	scale := min(1, float64(maxSide)/float64(max(w, h)))
	sw := max(1, int(math.Round(float64(w)*scale)))
	sh := max(1, int(math.Round(float64(h)*scale)))
	dst := image.NewRGBA(image.Rect(0, 0, sw, sh))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	if sw == w && sh == h {
		draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Over)
	} else {
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	return &Prepared{Mime: "image/jpeg", Data: out.Bytes(), Size: Size{Width: w, Height: h, SentWidth: sw, SentHeight: sh}}, nil
}
