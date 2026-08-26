package extract

import (
	"fmt"
	"strings"
	"testing"
)

// A document may put the scale of its text in the text matrix rather than in
// the font size, and TeX puts it there in every PDF it has ever produced:
// "/F1 1 Tf" followed by a text matrix of ten, rather than "/F1 10 Tf". Both
// draw the same page.
//
// Only the page's transform used to be counted when working out how large the
// text was, so such a document reported one-point text with half-point
// letters — while saying quite correctly where on the page each run began.
// Those are the two numbers a word break is decided by, and weighing a gap
// measured in points against a space width ten times too small made every
// ordinary kern between two letters look like a space.
//
// Every test in this package wrote its size into Tf, so nothing here ever saw
// it.

// kernedWord draws one word, one letter at a time, with the pen moved between
// letters the way a typesetter kerns them — a gap far smaller than a space,
// but not nothing. inTm puts the scale in the text matrix; otherwise it goes
// in the font size, which is the same page either way.
func kernedWord(word string, size float64, x, y float64, inTm bool) string {
	var b strings.Builder
	b.WriteString("BT\n")
	if inTm {
		fmt.Fprintf(&b, "/F1 1 Tf\n%g 0 0 %g %g %g Tm\n", size, size, x, y)
	} else {
		fmt.Fprintf(&b, "/F1 %g Tf\n1 0 0 1 %g %g Tm\n", size, x, y)
	}
	for i, r := range word {
		if i > 0 {
			// A tenth of an em of kerning, which is a fifth of this font's
			// space. It is a gap, and it is not a word break.
			b.WriteString("0.1 0 Td\n")
		}
		fmt.Fprintf(&b, "(%c) Tj\n", r)
	}
	b.WriteString("ET\n")
	return b.String()
}

func TestKerningIsNotAWordBreak(t *testing.T) {
	for _, c := range []struct {
		name  string
		inTm  bool
		size  float64
		x, y  float64
		words string
	}{
		{"scale in the text matrix", true, 10, 5, 150, "Original"},
		{"scale in the font size", false, 10, 5, 150, "Original"},
		{"a large scale in the text matrix", true, 24, 5, 100, "Domain"},
		{"a small scale in the text matrix", true, 4, 5, 50, "Parameterization"},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := pageWith(t, kernedWord(c.words, c.size, c.x, c.y, c.inTm), nil)
			got, err := Text(d, 1)
			if err != nil {
				t.Fatal(err)
			}
			got = strings.TrimSpace(got)
			if got != c.words {
				t.Errorf("the page says %q, and it was drawn as %q:\n"+
					"kerning between letters was read as a word break", got, c.words)
			}
		})
	}
}

// TestARealSpaceIsStillAWordBreak is the other half. A fix that never breaks
// a word would score perfectly on the measure that found this and be useless:
// a gap the width of a space has to keep coming back as one.
//
// The two words are placed at absolute positions rather than moved apart with
// Td, because Td shifts the line matrix and so means different distances in
// the two cases. The font is 500/1000 of an em throughout, so at ten points
// each letter is five wide, "one" ends at 20, and a space is five: starting
// "two" at 27 is a gap of seven, comfortably more than a space and nothing
// like a whole word.
func TestARealSpaceIsStillAWordBreak(t *testing.T) {
	for _, inTm := range []bool{true, false} {
		name := "scale in the font size"
		if inTm {
			name = "scale in the text matrix"
		}
		t.Run(name, func(t *testing.T) {
			d := pageWith(t, place("one", 10, 5, 150, inTm)+place("two", 10, 27, 150, inTm), nil)
			got, err := Text(d, 1)
			if err != nil {
				t.Fatal(err)
			}
			if got = strings.TrimSpace(got); got != "one two" {
				t.Errorf("the page says %q, want %q: a real space stopped being a word break", got, "one two")
			}
		})
	}
}

// place draws one word whole at an absolute position, with the scale in
// whichever of the two places is being tested.
func place(word string, size, x, y float64, inTm bool) string {
	var b strings.Builder
	b.WriteString("BT\n")
	if inTm {
		fmt.Fprintf(&b, "/F1 1 Tf\n%g 0 0 %g %g %g Tm\n", size, size, x, y)
	} else {
		fmt.Fprintf(&b, "/F1 %g Tf\n1 0 0 1 %g %g Tm\n", size, x, y)
	}
	fmt.Fprintf(&b, "(%s) Tj\nET\n", word)
	return b.String()
}

// TestTheTextMatrixScalesTheRunItself checks the numbers a caller reads,
// rather than only the text they produce. A run drawn at ten points is ten
// points tall whichever of the two places the document put the ten.
func TestTheTextMatrixScalesTheRunItself(t *testing.T) {
	inTm := pageWith(t, kernedWord("Domain", 10, 5, 150, true), nil)
	inTf := pageWith(t, kernedWord("Domain", 10, 5, 150, false), nil)
	a, err := Runs(inTm, 1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Runs(inTf, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) == 0 || len(b) == 0 {
		t.Fatalf("%d runs against %d", len(a), len(b))
	}
	if a[0].Size != b[0].Size {
		t.Errorf("size is %v with the scale in the text matrix and %v with it in the font size",
			a[0].Size, b[0].Size)
	}
	if a[0].Space != b[0].Space {
		t.Errorf("a space is %v wide with the scale in the text matrix and %v with it in the font size",
			a[0].Space, b[0].Space)
	}
	if a[0].Size != 10 {
		t.Errorf("text drawn at ten points reports a size of %v", a[0].Size)
	}
}
