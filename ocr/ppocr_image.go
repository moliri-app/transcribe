// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package ocr

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// raster is an image as the PaddleOCR models read it: blue, green, red per pixel, 0–255.
type raster struct {
	w, h int
	pix  []float32 // (y*w + x)*3 + channel
}

func newRaster(w, h int) *raster { return &raster{w: w, h: h, pix: make([]float32, w*h*3)} }

// decodeImage reads an image file into a raster. Transparent pixels become white, as on paper.
func decodeImage(path string) (*raster, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrImage, err)
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, fmt.Errorf("%w: empty image", ErrImage)
	}
	r := newRaster(b.Dx(), b.Dy())
	set := func(x, y int, red, green, blue float32) {
		i := (y*r.w + x) * 3
		r.pix[i], r.pix[i+1], r.pix[i+2] = blue, green, red
	}
	switch m := img.(type) {
	case *image.YCbCr:
		for y := 0; y < r.h; y++ {
			for x := 0; x < r.w; x++ {
				yi, ci := m.YOffset(b.Min.X+x, b.Min.Y+y), m.COffset(b.Min.X+x, b.Min.Y+y)
				cr, cg, cb := yCbCrToRGB(m.Y[yi], m.Cb[ci], m.Cr[ci])
				set(x, y, cr, cg, cb)
			}
		}
	case *image.Gray:
		for y := 0; y < r.h; y++ {
			row := m.Pix[y*m.Stride:]
			for x := 0; x < r.w; x++ {
				v := float32(row[x])
				set(x, y, v, v, v)
			}
		}
	default:
		for y := 0; y < r.h; y++ {
			for x := 0; x < r.w; x++ {
				// premultiplied, 16 bit: over white is c + (1 - a)
				cr, cg, cb, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
				white := float32(0xffff-a) / 257
				set(x, y, float32(cr)/257+white, float32(cg)/257+white, float32(cb)/257+white)
			}
		}
	}
	return r, nil
}

// yCbCrToRGB is JPEG's conversion (image/color.YCbCrToRGB) without rounding to bytes on the way.
func yCbCrToRGB(y, cb, cr uint8) (float32, float32, float32) {
	fy, fcb, fcr := float32(y), float32(cb)-128, float32(cr)-128
	clamp := func(v float32) float32 { return min(255, max(0, v)) }
	return clamp(fy + 1.402*fcr), clamp(fy - 0.344136*fcb - 0.714136*fcr), clamp(fy + 1.772*fcb)
}

// resize scales r to w×h bilinearly with pixel centres aligned, as OpenCV's INTER_LINEAR does.
func (r *raster) resize(w, h int) *raster {
	if w == r.w && h == r.h {
		return r
	}
	out := newRaster(w, h)
	fx, fy := float64(r.w)/float64(w), float64(r.h)/float64(h)
	xs := make([]struct {
		x0, x1 int
		a      float32
	}, w)
	for x := range xs {
		sx := max(0, (float64(x)+0.5)*fx-0.5)
		x0 := min(int(sx), r.w-1)
		xs[x].x0, xs[x].x1, xs[x].a = x0, min(x0+1, r.w-1), float32(sx-float64(x0))
	}
	for y := 0; y < h; y++ {
		sy := max(0, (float64(y)+0.5)*fy-0.5)
		y0 := min(int(sy), r.h-1)
		y1 := min(y0+1, r.h-1)
		b := float32(sy - float64(y0))
		row0, row1 := r.pix[y0*r.w*3:], r.pix[y1*r.w*3:]
		o := out.pix[y*w*3:]
		for x, c := range xs {
			for k := 0; k < 3; k++ {
				top := row0[c.x0*3+k]*(1-c.a) + row0[c.x1*3+k]*c.a
				bot := row1[c.x0*3+k]*(1-c.a) + row1[c.x1*3+k]*c.a
				o[x*3+k] = top*(1-b) + bot*b
			}
		}
	}
	return out
}

// padVertical adds black rows above and below.
func (r *raster) padVertical(n int) *raster {
	out := newRaster(r.w, r.h+2*n)
	copy(out.pix[n*r.w*3:], r.pix)
	return out
}

// chw lays the raster out as the models' input, channels first, scaled from 0–255 to −1…1, into
// dst at the given row width (a wider row is padded with zeros).
func (r *raster) chw(dst []float32, width int) {
	plane := r.h * width
	for y := 0; y < r.h; y++ {
		for x := 0; x < r.w; x++ {
			i := (y*r.w + x) * 3
			o := y*width + x
			dst[o] = r.pix[i]/127.5 - 1
			dst[plane+o] = r.pix[i+1]/127.5 - 1
			dst[2*plane+o] = r.pix[i+2]/127.5 - 1
		}
	}
}

// rot90 turns the raster a quarter to the left (numpy's rot90).
func (r *raster) rot90() *raster {
	out := newRaster(r.h, r.w)
	for y := 0; y < out.h; y++ {
		for x := 0; x < out.w; x++ {
			copy(out.pix[(y*out.w+x)*3:][:3], r.pix[(x*r.w+(r.w-1-y))*3:][:3])
		}
	}
	return out
}

// pt is a point in pixels.
type pt struct{ x, y float64 }

func (a pt) sub(b pt) pt        { return pt{a.x - b.x, a.y - b.y} }
func (a pt) add(b pt) pt        { return pt{a.x + b.x, a.y + b.y} }
func (a pt) mul(f float64) pt   { return pt{a.x * f, a.y * f} }
func (a pt) dot(b pt) float64   { return a.x*b.x + a.y*b.y }
func (a pt) norm() float64      { return math.Hypot(a.x, a.y) }
func (a pt) cross(b pt) float64 { return a.x*b.y - a.y*b.x }

// homography maps points of one plane to another (3×3, row major, h[8] = 1).
type homography [9]float64

func (m homography) apply(p pt) pt {
	w := m[6]*p.x + m[7]*p.y + m[8]
	return pt{(m[0]*p.x + m[1]*p.y + m[2]) / w, (m[3]*p.x + m[4]*p.y + m[5]) / w}
}

// solveHomography finds the homography taking from[i] to to[i] (OpenCV's getPerspectiveTransform).
func solveHomography(from, to [4]pt) (homography, bool) {
	var a [8][9]float64
	for i := 0; i < 4; i++ {
		x, y, u, v := from[i].x, from[i].y, to[i].x, to[i].y
		a[2*i] = [9]float64{x, y, 1, 0, 0, 0, -x * u, -y * u, u}
		a[2*i+1] = [9]float64{0, 0, 0, x, y, 1, -x * v, -y * v, v}
	}
	for c := 0; c < 8; c++ {
		p := c
		for r := c + 1; r < 8; r++ {
			if math.Abs(a[r][c]) > math.Abs(a[p][c]) {
				p = r
			}
		}
		if math.Abs(a[p][c]) < 1e-12 {
			return homography{}, false
		}
		a[c], a[p] = a[p], a[c]
		for r := 0; r < 8; r++ {
			if r == c {
				continue
			}
			f := a[r][c] / a[c][c]
			for k := c; k < 9; k++ {
				a[r][k] -= f * a[c][k]
			}
		}
	}
	var m homography
	for i := 0; i < 8; i++ {
		m[i] = a[i][8] / a[i][i]
	}
	m[8] = 1
	return m, true
}

// cubic is OpenCV's bicubic kernel (A = −0.75).
func cubic(x float64) float64 {
	const a = -0.75
	x = math.Abs(x)
	switch {
	case x <= 1:
		return ((a+2)*x-(a+3))*x*x + 1
	case x < 2:
		return ((a*x-5*a)*x+8*a)*x - 4*a
	}
	return 0
}

// warp samples the w×h image whose pixel (x, y) lies at m(x, y) in r, bicubic, the border
// repeated (OpenCV's warpPerspective with INTER_CUBIC and BORDER_REPLICATE).
func (r *raster) warp(m homography, w, h int) *raster {
	out := newRaster(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			s := m.apply(pt{float64(x), float64(y)})
			x0, y0 := math.Floor(s.x), math.Floor(s.y)
			var wx, wy [4]float64
			for k := 0; k < 4; k++ {
				wx[k] = cubic(s.x - (x0 + float64(k-1)))
				wy[k] = cubic(s.y - (y0 + float64(k-1)))
			}
			var acc [3]float64
			for j := 0; j < 4; j++ {
				sy := min(max(int(y0)+j-1, 0), r.h-1)
				for i := 0; i < 4; i++ {
					sx := min(max(int(x0)+i-1, 0), r.w-1)
					f := wx[i] * wy[j]
					p := r.pix[(sy*r.w+sx)*3:]
					acc[0] += f * float64(p[0])
					acc[1] += f * float64(p[1])
					acc[2] += f * float64(p[2])
				}
			}
			o := out.pix[(y*w+x)*3:]
			for k := 0; k < 3; k++ {
				o[k] = float32(min(255, max(0, acc[k])))
			}
		}
	}
	return out
}
