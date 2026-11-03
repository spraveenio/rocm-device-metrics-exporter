package main

import (
	"bytes"
	"fmt"
)

// encodedPage is a JPEG ready to be embedded as a full-page image.
type encodedPage struct {
	jpeg   []byte
	w, h   int
	gray   bool
	wPt    float64
	hPt    float64
	rotate int
}

// writePDF builds a minimal PDF where each page is a single DCT-encoded image.
func writePDF(pages []encodedPage) []byte {
	var buf bytes.Buffer
	offsets := []int{0}
	newObj := func() int {
		offsets = append(offsets, 0)
		return len(offsets) - 1
	}
	begin := func(n int) {
		offsets[n] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n", n)
	}

	buf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	catalog, pagesObj, info := newObj(), newObj(), newObj()

	kids := make([]int, len(pages))
	for i, p := range pages {
		pageObj, contentObj, imgObj := newObj(), newObj(), newObj()
		kids[i] = pageObj

		begin(pageObj)
		fmt.Fprintf(&buf, "<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %s %s] /Resources << /XObject << /Im0 %d 0 R >> >> /Contents %d 0 R",
			pagesObj, num(p.wPt), num(p.hPt), imgObj, contentObj)
		if p.rotate != 0 {
			fmt.Fprintf(&buf, " /Rotate %d", p.rotate)
		}
		buf.WriteString(" >>\nendobj\n")

		content := fmt.Sprintf("q %s 0 0 %s 0 0 cm /Im0 Do Q", num(p.wPt), num(p.hPt))
		begin(contentObj)
		fmt.Fprintf(&buf, "<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(content), content)

		cs := "/DeviceRGB"
		if p.gray {
			cs = "/DeviceGray"
		}
		begin(imgObj)
		fmt.Fprintf(&buf, "<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace %s /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\n",
			p.w, p.h, cs, len(p.jpeg))
		buf.Write(p.jpeg)
		buf.WriteString("\nendstream\nendobj\n")
	}

	begin(pagesObj)
	buf.WriteString("<< /Type /Pages /Kids [")
	for i, k := range kids {
		if i > 0 {
			buf.WriteByte(' ')
		}
		fmt.Fprintf(&buf, "%d 0 R", k)
	}
	fmt.Fprintf(&buf, "] /Count %d >>\nendobj\n", len(pages))

	begin(catalog)
	fmt.Fprintf(&buf, "<< /Type /Catalog /Pages %d 0 R >>\nendobj\n", pagesObj)

	begin(info)
	buf.WriteString("<< /Producer (pdfshrink) >>\nendobj\n")

	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, off := range offsets[1:] {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root %d 0 R /Info %d 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(offsets), catalog, info, xref)
	return buf.Bytes()
}

func num(f float64) string {
	return fmt.Sprintf("%.2f", f)
}
