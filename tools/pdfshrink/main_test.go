package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// fakeScan writes an A4 "scanned" PDF at 300 DPI with noisy text-like content.
func fakeScan(t *testing.T, path string, pages int, colored bool) int64 {
	t.Helper()
	rng := rand.New(rand.NewSource(1))
	var enc []encodedPage
	for p := 0; p < pages; p++ {
		img := image.NewRGBA(image.Rect(0, 0, 2480, 3508))
		for y := 0; y < 3508; y++ {
			for x := 0; x < 2480; x++ {
				v := uint8(235 + rng.Intn(20))
				if (y/40)%2 == 0 && (x/14+y/40+p)%5 != 0 && x > 200 && x < 2280 && rng.Intn(3) == 0 {
					v = uint8(rng.Intn(60))
				}
				c := color.RGBA{v, v, v, 255}
				if colored && x < 600 && y < 600 {
					c = color.RGBA{200, 30, uint8(rng.Intn(80)), 255}
				}
				img.SetRGBA(x, y, c)
			}
		}
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
			t.Fatal(err)
		}
		enc = append(enc, encodedPage{jpeg: buf.Bytes(), w: 2480, h: 3508, wPt: 595.28, hPt: 841.89, rotate: 90 * (p % 2)})
	}
	data := writePDF(enc)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return int64(len(data))
}

func TestShrinkToTarget(t *testing.T) {
	for _, tc := range []struct {
		name    string
		colored bool
		size    string
		args    []string
	}{
		{"gray-200KB", false, "200KB", nil},
		{"color-300KB", true, "300KB", nil},
		{"color-keep-150KB", true, "150KB", []string{"-keep-color"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			in := filepath.Join(dir, "scan.pdf")
			out := filepath.Join(dir, "out.pdf")
			inSize := fakeScan(t, in, 3, tc.colored)
			target, _ := parseSize(tc.size)
			if inSize <= target {
				t.Fatalf("fixture too small: %d", inSize)
			}

			args := append(append([]string{"-size", tc.size, "-o", out}, tc.args...), in)
			if err := run(args, io.Discard, io.Discard); err != nil {
				t.Fatal(err)
			}
			st, err := os.Stat(out)
			if err != nil {
				t.Fatal(err)
			}
			if st.Size() > target {
				t.Fatalf("output %d > target %d", st.Size(), target)
			}
			if err := api.ValidateFile(context.Background(), out, nil, nil); err != nil {
				t.Fatalf("output is not a valid PDF: %v", err)
			}
			pages, err := loadPages(context.Background(), out, "", t.Logf)
			if err != nil {
				t.Fatal(err)
			}
			if len(pages) != 3 || pages[1].rotate != 90 {
				t.Fatalf("unexpected pages: n=%d rotate=%d", len(pages), pages[1].rotate)
			}
			t.Logf("%d -> %d bytes", inSize, st.Size())
		})
	}
}

func TestTargetNotMet(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "scan.pdf")
	fakeScan(t, in, 1, false)
	err := run([]string{"-size", "1KB", "-o", filepath.Join(dir, "o.pdf"), in}, io.Discard, io.Discard)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("could not reach")) {
		t.Fatalf("expected target-not-met error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "o.pdf")); err != nil {
		t.Fatal("smallest result should still be written")
	}
}

func TestParseSize(t *testing.T) {
	for in, want := range map[string]int64{
		"200KB": 200000, "200k": 200000, "500KiB": 512000, "1.5MB": 1500000, "204800": 204800, "2MiB": 2 << 20,
	} {
		if got, err := parseSize(in); err != nil || got != want {
			t.Errorf("parseSize(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "-5KB", "0"} {
		if _, err := parseSize(in); err == nil {
			t.Errorf("parseSize(%q) should fail", in)
		}
	}
}
