// pdfshrink shrinks scanned (image-based) multi-page PDF documents so that the
// resulting file fits within a target size, e.g. "200KB".
//
// Every page's scanned image is extracted, then the tool searches for the best
// combination of resolution (DPI), JPEG quality and color mode that fits the
// requested size, and writes a new image-only PDF preserving page sizes and
// rotation. It is pure Go and has no external runtime dependencies.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const exitTargetNotMet = 2

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "pdfshrink:", err)
		if errors.Is(err, errTargetNotMet) {
			os.Exit(exitTargetNotMet)
		}
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("pdfshrink", flag.ContinueOnError)
	fs.SetOutput(stderr)
	sizeStr := fs.String("size", "", "target maximum output size, e.g. 200KB, 1.5MB, 500KiB, 204800 (required)")
	out := fs.String("o", "", "output PDF path (default: <input>-<size>.pdf)")
	maxDPI := fs.Float64("max-dpi", 300, "never output pages above this DPI")
	minDPI := fs.Float64("min-dpi", 20, "lowest DPI to try before giving up")
	gray := fs.Bool("gray", false, "always convert pages to grayscale")
	keepColor := fs.Bool("keep-color", false, "never convert color pages to grayscale")
	force := fs.Bool("force", false, "re-encode even if the input already fits the target size")
	password := fs.String("password", "", "user password for encrypted PDFs")
	verbose := fs.Bool("v", false, "verbose progress output")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: pdfshrink -size <limit> [options] <input.pdf>\n\nOptions:\n")
		fs.PrintDefaults()
		fmt.Fprintf(stderr, "\nSize units: B, KB/K (1000), KiB (1024), MB/M (1000^2), MiB (1024^2).\n")
		fmt.Fprintf(stderr, `
Examples (Linux / macOS):
  pdfshrink -size 200KB scan.pdf                 # writes scan-200kb.pdf
  pdfshrink -size 1MB -o small.pdf contract.pdf
  pdfshrink -size 300KB -gray -v statement.pdf

macOS (M1/Apple Silicon) first run of a downloaded binary:
  chmod +x pdfshrink-darwin-arm64 && xattr -d com.apple.quarantine pdfshrink-darwin-arm64

Exit codes: 0 fits, 1 error, 2 target not reachable (smallest result still written).
See tools/pdfshrink/README.md for details.
`)
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 || *sizeStr == "" {
		fs.Usage()
		return errors.New("an input PDF and -size are required")
	}
	if *gray && *keepColor {
		return errors.New("-gray and -keep-color are mutually exclusive")
	}
	if *minDPI <= 0 || *maxDPI < *minDPI {
		return errors.New("require 0 < -min-dpi <= -max-dpi")
	}
	target, err := parseSize(*sizeStr)
	if err != nil {
		return err
	}
	in := fs.Arg(0)
	outPath := *out
	if outPath == "" {
		outPath = strings.TrimSuffix(in, ".pdf") + "-" + strings.ToLower(*sizeStr) + ".pdf"
	}

	logf := func(format string, a ...any) {
		if *verbose {
			fmt.Fprintf(stderr, format+"\n", a...)
		}
	}

	st, err := os.Stat(in)
	if err != nil {
		return err
	}
	if st.Size() <= target && !*force {
		data, err := os.ReadFile(in)
		if err != nil {
			return err
		}
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s already fits (%s <= %s); copied to %s\n", in, human(st.Size()), human(target), outPath)
		return nil
	}

	pages, err := loadPages(context.Background(), in, *password, logf)
	if err != nil {
		return err
	}
	logf("loaded %d page(s) from %s (%s)", len(pages), in, human(st.Size()))

	res, err := shrink(pages, target, options{
		maxDPI:    *maxDPI,
		minDPI:    *minDPI,
		forceGray: *gray,
		keepColor: *keepColor,
		logf:      logf,
	})
	if res == nil {
		return err
	}
	if werr := os.WriteFile(outPath, res.pdf, 0o644); werr != nil {
		return werr
	}
	mode := "color"
	if res.gray {
		mode = "grayscale"
	}
	fmt.Fprintf(stdout, "%s: %d page(s), %s -> %s (target %s) at %.0f DPI, JPEG quality %d, %s\n",
		outPath, len(pages), human(st.Size()), human(int64(len(res.pdf))), human(target), res.dpi, res.quality, mode)
	return err
}

// parseSize parses sizes such as "200KB", "200k", "1.5MB", "500KiB" or "204800".
func parseSize(s string) (int64, error) {
	t := strings.ToUpper(strings.TrimSpace(s))
	units := []struct {
		suffix string
		mult   float64
	}{
		{"KIB", 1024}, {"MIB", 1024 * 1024}, {"GIB", 1024 * 1024 * 1024},
		{"KB", 1000}, {"MB", 1000 * 1000}, {"GB", 1000 * 1000 * 1000},
		{"K", 1000}, {"M", 1000 * 1000}, {"G", 1000 * 1000 * 1000},
		{"B", 1},
	}
	mult := 1.0
	for _, u := range units {
		if strings.HasSuffix(t, u.suffix) {
			mult = u.mult
			t = strings.TrimSpace(strings.TrimSuffix(t, u.suffix))
			break
		}
	}
	v, err := strconv.ParseFloat(t, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	return int64(v * mult), nil
}

func human(n int64) string {
	switch {
	case n >= 1000*1000:
		return fmt.Sprintf("%.2fMB", float64(n)/1e6)
	case n >= 1000:
		return fmt.Sprintf("%.1fKB", float64(n)/1e3)
	}
	return fmt.Sprintf("%dB", n)
}
