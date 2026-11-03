package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"runtime"
	"sync"

	"golang.org/x/image/draw"
)

var errTargetNotMet = errors.New("could not reach target size; wrote the smallest achievable result")

const (
	maxQuality = 90
	dpiStep    = 0.85
)

type options struct {
	maxDPI    float64
	minDPI    float64
	forceGray bool
	keepColor bool
	logf      func(string, ...any)
}

type result struct {
	pdf     []byte
	dpi     float64
	quality int
	gray    bool
}

// shrink searches for the highest-fidelity encoding that fits within target bytes.
// Search order: color with decent quality, grayscale with decent quality, then
// the same with very low quality; within each pass resolution is lowered until
// it fits and then JPEG quality is maximised by binary search.
func shrink(pages []page, target int64, o options) (*result, error) {
	if len(pages) == 0 {
		return nil, errors.New("document has no pages")
	}
	allGray := true
	srcDPI := 0.0
	for _, p := range pages {
		if !isGrayish(p.img) {
			allGray = false
		}
		srcDPI = math.Max(srcDPI, p.srcDPI())
	}
	startDPI := math.Min(o.maxDPI, srcDPI)

	var modes []bool // gray?
	switch {
	case o.forceGray || allGray:
		modes = []bool{true}
	case o.keepColor:
		modes = []bool{false}
	default:
		modes = []bool{false, true}
	}

	var smallest *result
	consider := func(r *result) {
		if smallest == nil || len(r.pdf) < len(smallest.pdf) {
			smallest = r
		}
	}

	for _, qMin := range []int{40, 5} {
		for _, gray := range modes {
			for dpi := startDPI; dpi >= o.minDPI; {
				prepared := prepare(pages, dpi, gray)
				r := build(prepared, dpi, qMin, gray)
				consider(r)
				size := int64(len(r.pdf))
				o.logf("try dpi=%.0f gray=%v q=%d -> %s", dpi, gray, qMin, human(size))
				if size > target {
					// Size scales roughly with pixel count (dpi^2); jump close to the boundary.
					prev := dpi
					next := dpi * math.Sqrt(float64(target)/float64(size)) * 1.05
					dpi = math.Max(math.Min(next, dpi*dpiStep), dpi*0.5)
					if dpi < o.minDPI && prev > o.minDPI {
						dpi = o.minDPI // always give the floor resolution one try
					}
					continue
				}
				best := r
				lo, hi := qMin+1, maxQuality
				for lo <= hi {
					q := (lo + hi) / 2
					r := build(prepared, dpi, q, gray)
					o.logf("try dpi=%.0f gray=%v q=%d -> %s", dpi, gray, q, human(int64(len(r.pdf))))
					if int64(len(r.pdf)) <= target {
						best, lo = r, q+1
					} else {
						hi = q - 1
					}
				}
				return best, nil
			}
		}
	}
	return smallest, fmt.Errorf("%w (%s > %s); try a lower -min-dpi", errTargetNotMet, human(int64(len(smallest.pdf))), human(target))
}

type preparedPage struct {
	img image.Image
	src page
}

// prepare resamples every page to dpi (never upscaling) in the given color mode.
func prepare(pages []page, dpi float64, gray bool) []preparedPage {
	out := make([]preparedPage, len(pages))
	parallel(len(pages), func(i int) {
		p := pages[i]
		b := p.img.Bounds()
		scale := math.Min(1, dpi/p.srcDPI())
		w := max(1, int(math.Round(float64(b.Dx())*scale)))
		h := max(1, int(math.Round(float64(b.Dy())*scale)))
		rect := image.Rect(0, 0, w, h)
		var dst draw.Image
		if gray {
			dst = image.NewGray(rect)
		} else {
			dst = image.NewRGBA(rect)
		}
		if w == b.Dx() && h == b.Dy() {
			draw.Draw(dst, rect, p.img, b.Min, draw.Src)
		} else {
			draw.BiLinear.Scale(dst, rect, p.img, b, draw.Src, nil)
		}
		out[i] = preparedPage{img: dst, src: p}
	})
	return out
}

func build(pages []preparedPage, dpi float64, quality int, gray bool) *result {
	enc := make([]encodedPage, len(pages))
	parallel(len(pages), func(i int) {
		var buf bytes.Buffer
		// Encoding into an in-memory buffer cannot fail for these image types.
		_ = jpeg.Encode(&buf, pages[i].img, &jpeg.Options{Quality: quality})
		b := pages[i].img.Bounds()
		s := pages[i].src
		enc[i] = encodedPage{jpeg: buf.Bytes(), w: b.Dx(), h: b.Dy(), gray: gray, wPt: s.wPt, hPt: s.hPt, rotate: s.rotate}
	})
	return &result{pdf: writePDF(enc), dpi: dpi, quality: quality, gray: gray}
}

func parallel(n int, fn func(int)) {
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer func() { <-sem; wg.Done() }()
			fn(i)
		}(i)
	}
	wg.Wait()
}

// isGrayish reports whether an image carries (almost) no color information.
func isGrayish(img image.Image) bool {
	switch img.ColorModel() {
	case color.GrayModel, color.Gray16Model:
		return true
	}
	if y, ok := img.(*image.YCbCr); ok {
		for _, s := range [][]uint8{y.Cb, y.Cr} {
			if !samplesNear(s, 128) {
				return false
			}
		}
		return true
	}
	b := img.Bounds()
	step := max(1, int(math.Sqrt(float64(b.Dx()*b.Dy())/20000)))
	colored, total := 0, 0
	for y := b.Min.Y; y < b.Max.Y; y += step {
		for x := b.Min.X; x < b.Max.X; x += step {
			r, g, bl, _ := img.At(x, y).RGBA()
			total++
			if spread(r>>8, g>>8, bl>>8) > 24 {
				colored++
			}
		}
	}
	return colored*200 <= total
}

func samplesNear(s []uint8, center int) bool {
	step := max(1, len(s)/20000)
	off, total := 0, 0
	for i := 0; i < len(s); i += step {
		total++
		if d := int(s[i]) - center; d > 12 || d < -12 {
			off++
		}
	}
	return off*200 <= total
}

func spread(r, g, b uint32) uint32 {
	return max(r, g, b) - min(r, g, b)
}
