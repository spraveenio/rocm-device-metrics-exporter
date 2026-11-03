# pdfshrink

Shrink a high-resolution **scanned** (image-based) multi-page PDF to fit a size
limit, such as the 200KB cap on many upload portals.

`pdfshrink` takes the scanned image from each page and searches for the best
combination of:

1. **Resolution (DPI)**: lowered step by step until the file fits.
2. **JPEG quality**: the highest quality that still fits, found by binary search.
3. **Color mode**: color first. It switches to grayscale only when color can't
   fit (unless you pass `-keep-color`). Pages that are already gray are encoded
   as grayscale automatically.

It then writes a new PDF that keeps every page's size, order and rotation. The
tool is pure Go and has no runtime dependencies (no Ghostscript or Poppler).

> **Limitation:** It only works on scanned or image-based PDFs. Pages without a
> raster image are rejected. The output is image-only, so selectable text and
> any OCR layer are dropped. If a page holds several images, the tool keeps
> only the largest one.

## Download or build

### Prebuilt binaries (CI)

Every pull request build (workflow **CI Build**, job
`pdfshrink (linux + cross-build)`) uploads a `pdfshrink-binaries` artifact
containing:

| Platform                            | File                          |
|-------------------------------------|-------------------------------|
| Linux x86_64                        | `pdfshrink-linux-amd64`       |
| Linux ARM64                         | `pdfshrink-linux-arm64`       |
| macOS Apple Silicon (M1/M2/M3 Air)  | `pdfshrink-darwin-arm64`      |
| macOS Intel                         | `pdfshrink-darwin-amd64`      |
| Windows x86_64                      | `pdfshrink-windows-amd64.exe` |

### Build from source

You need Go (the version in [`go.mod`](go.mod) or newer). From the repository root:

```bash
make pdfshrink        # all platforms -> bin/pdfshrink/
make pdfshrink-test   # go vet + unit tests
```

Or build only for the machine you're on:

```bash
cd tools/pdfshrink
go build -o pdfshrink .
```

## Running on Linux

```bash
chmod +x pdfshrink-linux-amd64          # use pdfshrink-linux-arm64 on ARM
sudo mv pdfshrink-linux-amd64 /usr/local/bin/pdfshrink   # optional

pdfshrink -size 200KB scan.pdf
# -> scan-200kb.pdf
```

## Running on a Mac M1 Air (Apple Silicon)

1. Download `pdfshrink-darwin-arm64` from the CI artifact (unzip it if needed),
   or build it with Homebrew Go:

   ```bash
   brew install go
   cd tools/pdfshrink && go build -o pdfshrink .
   ```

2. Open **Terminal** and make the downloaded binary runnable. macOS quarantines
   unsigned binaries downloaded from the internet, so remove the quarantine flag:

   ```bash
   cd ~/Downloads
   chmod +x pdfshrink-darwin-arm64
   xattr -d com.apple.quarantine pdfshrink-darwin-arm64 2>/dev/null || true
   mkdir -p ~/bin && mv pdfshrink-darwin-arm64 ~/bin/pdfshrink
   echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
   ```

   If macOS still says *"cannot be opened because the developer cannot be
   verified"*, open **System Settings → Privacy & Security** and click
   **Allow Anyway**, then run the command again.

3. Run it:

   ```bash
   pdfshrink -size 200KB ~/Downloads/scan.pdf
   ```

## Usage

```text
pdfshrink -size <limit> [options] <input.pdf>
```

| Flag           | Default                 | Description                                                    |
|----------------|-------------------------|----------------------------------------------------------------|
| `-size`        | *(required)*            | Target maximum size: `200KB`, `1.5MB`, `500KiB`, `204800`, ... |
| `-o`           | `<input>-<size>.pdf`    | Output file                                                    |
| `-max-dpi`     | `300`                   | Never output pages above this DPI                              |
| `-min-dpi`     | `20`                    | Lowest DPI to try before giving up                             |
| `-gray`        | `false`                 | Always convert to grayscale (smaller, good for text)           |
| `-keep-color`  | `false`                 | Never convert color pages to grayscale                         |
| `-force`       | `false`                 | Re-encode even if the input already fits                       |
| `-password`    | *(none)*                | Password for encrypted PDFs                                    |
| `-v`           | `false`                 | Print every DPI/quality attempt                                |

Size units: `B`, `KB`/`K` = 1000, `KiB` = 1024, `MB`/`M` = 1000², `MiB` = 1024².
If a portal's "200KB" means 200×1024 bytes, `200KB` still fits, because it's
the stricter of the two.

### Examples

```bash
pdfshrink -size 200KB passport.pdf                      # -> passport-200kb.pdf
pdfshrink -size 1MB -o small.pdf contract.pdf           # custom output name
pdfshrink -size 300KB -gray -v statement.pdf            # grayscale, show progress
pdfshrink -size 500KB -keep-color -max-dpi 200 id.pdf   # keep color, cap at 200 DPI
```

Sample output:

```text
passport-200kb.pdf: 3 page(s), 17.24MB -> 198.5KB (target 200.0KB) at 70 DPI, JPEG quality 41, grayscale
```

### Exit codes

| Code | Meaning                                                                                                                                       |
|------|-----------------------------------------------------------------------------------------------------------------------------------------------|
| `0`  | Success. The output fits the target.                                                                                                          |
| `1`  | Error, such as bad arguments, an unreadable PDF or a page with no image.                                                                      |
| `2`  | The target couldn't be reached even at `-min-dpi` and minimum quality. The smallest result is still written; try a lower `-min-dpi` or a larger size. |

### Tips

- Text scans: `-gray` usually lets you keep a much higher DPI for the same size.
- If text becomes unreadable, fewer pages or a larger limit are the only
  remedies. Very small limits on many-page documents force very low DPI.
