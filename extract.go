// Package extract reads a PDF page back: the text on it, with where each piece
// of it sits, and the images it places.
//
// A page does not hold text. It holds instructions for drawing glyphs, and the
// text is what those glyphs were meant to say — which the document says only
// if it was asked to. Where it does not, this says so rather than guessing:
// a run whose characters could not be worked out comes back marked, so that a
// caller can tell "this page says nothing" from "this page could not be read".
package extract

import (
	"sort"
	"strings"

	"github.com/go-pdfkit/pdffont"
	"github.com/go-pdfkit/reader"
)

// A Run is a piece of text drawn in one go: one font, one size, one place.
type Run struct {
	// Text is what the run says.
	Text string
	// X and Y are where it starts, in the page's own coordinates — up from
	// the bottom left corner, in points.
	X, Y float64
	// Width is how far the pen moved drawing it, and Size how tall the text
	// is, both in points.
	Width, Size float64
	// Font is the name the page's resources give the font.
	Font string
	// Space is how wide a space would be here, which is what says whether a
	// gap between two runs is a word break or only kerning.
	Space float64
	// Invisible reports text drawn in the mode that puts no ink on the page.
	// It is how a scanner puts what it read underneath the picture it read it
	// from, so it is text worth having — and worth knowing about.
	Invisible bool
	// Unreadable reports a run some of whose codes said nothing about which
	// characters they were: the font carried no map and named its glyphs
	// after numbers. What Text holds is then only the part that could be read.
	Unreadable bool
}

// Text is everything the page says, in reading order, with the lines put back
// together.
func Text(d *reader.Document, page int) (string, error) {
	runs, err := Runs(d, page)
	if err != nil {
		return "", err
	}
	return Assemble(runs), nil
}

// Runs is every piece of text on the page, in the order it was drawn.
func Runs(d *reader.Document, page int) ([]Run, error) {
	e, err := walk(d, page, false)
	if err != nil {
		return nil, err
	}
	return e.runs, nil
}

// walk reads a page's content stream once, keeping what it finds.
//
// A page whose content decoded only part of the way is read as far as it went:
// 263 streams in 212 of the 1 633 real forms in the corpus cannot be decoded
// cleanly, and half a page of text is worth more to somebody searching than
// none of it. A page that decoded no bytes at all is a different answer and
// stays an error — a conversion that silently produces an empty document from
// an unreadable page is worse than one that says it could not read it, and
// go-pdfkit/latex relies on being told.
//
// wantPictures says whether the caller is going to look at the pictures. A
// page's images have nothing to do with its text, and undoing their filters is
// the most expensive thing on the page: one arXiv figure holds 378 MB of image
// once decompressed, so asking that page what it *said* used to cost 1 094 MB
// and 1.3 seconds spent inflating pictures the answer throws away. Where they
// land is still noted either way, since that costs a matrix multiply; only the
// bytes are left alone.
func walk(d *reader.Document, page int, wantPictures bool) (*extractor, error) {
	dec, err := d.PageContentDecoded(page)
	if err != nil {
		return nil, err
	}
	if len(dec.Data) == 0 && dec.Cause != nil {
		return nil, dec.Cause
	}
	content := dec.Data
	p, _ := d.Page(page)
	resources, _ := d.GetDict(p, "Resources")
	e := &extractor{doc: d, fonts: map[int]*pdffont.Font{}, wantPictures: wantPictures}
	e.run(content, resources, initialState(d, p), 0)
	return e, nil
}

// Assemble puts runs back together as lines of text: everything on one
// baseline in the order it sits across the page, with a space wherever the
// gap between two pieces is wide enough to be one.
func Assemble(runs []Run) string {
	if len(runs) == 0 {
		return ""
	}
	lines := groupIntoLines(runs)
	var out strings.Builder
	for i, line := range lines {
		if i > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(joinLine(line))
	}
	return out.String()
}

// groupIntoLines gathers runs that sit on the same baseline, in the order they
// appear down the page.
func groupIntoLines(runs []Run) [][]Run {
	sorted := append([]Run{}, runs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if !sameLine(sorted[i], sorted[j]) {
			return sorted[i].Y > sorted[j].Y
		}
		return sorted[i].X < sorted[j].X
	})
	var lines [][]Run
	for _, r := range sorted {
		if n := len(lines); n > 0 && sameLine(lines[n-1][0], r) {
			lines[n-1] = append(lines[n-1], r)
			continue
		}
		lines = append(lines, []Run{r})
	}
	return lines
}

// sameLine reports whether two runs sit on the same baseline. A line is
// allowed to wobble by a fraction of its own height, which is what a
// superscript or a slightly different font does.
func sameLine(a, b Run) bool {
	tol := a.Size * 0.3
	if tol <= 0 {
		tol = 1
	}
	d := a.Y - b.Y
	return d < tol && d > -tol
}

// joinLine writes one line out, putting a space wherever the gap between two
// pieces is wide enough to be one.
func joinLine(line []Run) string {
	var out strings.Builder
	for i, r := range line {
		if i > 0 {
			prev := line[i-1]
			gap := r.X - (prev.X + prev.Width)
			if wantsSpace(prev, r, gap) {
				out.WriteByte(' ')
			}
		}
		out.WriteString(r.Text)
	}
	return strings.TrimRight(out.String(), " ")
}

// wantsSpace decides whether a gap between two pieces of a line is a word
// break. A space of its own is never doubled, and a gap of about a third of a
// space is where kerning ends and words begin.
func wantsSpace(prev, next Run, gap float64) bool {
	if strings.HasSuffix(prev.Text, " ") || strings.HasPrefix(next.Text, " ") {
		return false
	}
	space := prev.Space
	if space <= 0 {
		space = prev.Size * 0.25
	}
	return gap > space*0.3
}
