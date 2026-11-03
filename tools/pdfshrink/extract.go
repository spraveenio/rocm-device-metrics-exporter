package main

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	_ "golang.org/x/image/tiff"
)

// page is one scanned page: its raster image and its physical size in points.
type page struct {
	img    image.Image
	wPt    float64
	hPt    float64
	rotate int
}

// loadPages extracts the dominant (largest) raster image of every page.
func loadPages(c context.Context, path, password string, logf func(string, ...any)) ([]page, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	conf := model.NewDefaultConfiguration()
	conf.Cmd = model.EXTRACTIMAGES
	conf.ValidationMode = model.ValidationRelaxed
	if password != "" {
		conf.UserPW = password
		conf.OwnerPW = password
	}
	ctx, err := api.ReadValidateAndOptimize(c, f, conf, nil)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	pages := make([]page, 0, ctx.PageCount)
	for nr := 1; nr <= ctx.PageCount; nr++ {
		_, _, attrs, err := ctx.PageDict(c, nr, false)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", nr, err)
		}
		box := attrs.CropBox
		if box == nil {
			box = attrs.MediaBox
		}
		if box == nil {
			return nil, fmt.Errorf("page %d: missing MediaBox", nr)
		}

		imgs, err := pdfcpu.ExtractPageImages(c, ctx, nr, false)
		if err != nil {
			return nil, fmt.Errorf("page %d: extract images: %w", nr, err)
		}
		var best image.Image
		bestArea := 0
		for _, mi := range imgs {
			if mi.Thumb || mi.Reader == nil {
				continue
			}
			im, _, err := image.Decode(mi)
			if err != nil {
				logf("page %d: skipping image %s (%s): %v", nr, mi.Name, mi.FileType, err)
				continue
			}
			if a := im.Bounds().Dx() * im.Bounds().Dy(); a > bestArea {
				best, bestArea = im, a
			}
		}
		if best == nil {
			return nil, fmt.Errorf("page %d has no decodable raster image; only scanned (image-based) PDFs are supported", nr)
		}
		if len(imgs) > 1 {
			logf("page %d: %d images found, using the largest", nr, len(imgs))
		}
		p := page{img: best, wPt: box.Width(), hPt: box.Height(), rotate: ((attrs.Rotate % 360) + 360) % 360}
		logf("page %d: %dx%d px, %.0fx%.0f pt (~%.0f DPI)", nr, best.Bounds().Dx(), best.Bounds().Dy(), p.wPt, p.hPt, p.srcDPI())
		pages = append(pages, p)
	}
	return pages, nil
}

// srcDPI is the effective resolution of the scanned image on the page.
func (p page) srcDPI() float64 {
	return float64(p.img.Bounds().Dx()) / (p.wPt / 72)
}
