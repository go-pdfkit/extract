package extract_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/reader"
)

// seedDir can be pointed at an adversarial corpus — PDF_SEEDS — such as
// mozilla's pdf.js test suite, every file of which is there because it broke a
// reader once. Without it the built-in seeds and anything under testdata still
// run.
var seedDir = os.Getenv("PDF_SEEDS")

func addSeeds(f *testing.F, max, cap int) {
	f.Add([]byte("%PDF-1.7\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n" +
		"2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n" +
		"3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 99 99]>>endobj\n" +
		"trailer<</Root 1 0 R>>"))
	if seedDir == "" {
		return
	}
	ents, err := os.ReadDir(seedDir)
	if err != nil {
		return
	}
	n := 0
	for _, e := range ents {
		info, err := e.Info()
		if err != nil || info.Size() > int64(cap) || filepath.Ext(e.Name()) != ".pdf" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(seedDir, e.Name()))
		if err != nil {
			continue
		}
		f.Add(b)
		if n++; n >= max {
			return
		}
	}
}

// FuzzExtract reads a page back three ways. The budget is asserted as well as
// the absence of a panic: what a page says is worked out by running a little
// program the document wrote, over numbers the document chose, and a small
// file that costs a large amount of time raises nothing on its own.
func FuzzExtract(f *testing.F) {
	addSeeds(f, 300, 40*1024)
	f.Fuzz(func(t *testing.T, b []byte) {
		start := time.Now()
		d, err := reader.Open(b)
		if err != nil {
			return
		}
		n := d.PageCount()
		if n > 3 {
			n = 3
		}
		for i := 1; i <= n; i++ {
			runs, err := extract.Runs(d, i)
			if err == nil {
				extract.Assemble(runs)
			}
			_, _ = extract.Text(d, i)
			_, _ = extract.Images(d, i)
		}
		if el := time.Since(start); el > 5*time.Second {
			t.Fatalf("%d bytes took %s over %d pages", len(b), el, n)
		}
	})
}
