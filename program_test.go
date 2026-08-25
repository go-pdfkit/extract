package extract

import (
	"strings"
	"testing"

	"github.com/go-opentype/fonts"
	"github.com/go-pdfkit/reader"
)

func TestASymbolicFontReadThroughItsOwnProgram(t *testing.T) {
	// A font that says it is symbolic is addressed through its own character
	// map. Nothing in the document says what its codes are, so the program is
	// asked — and a PostScript Type 1 program, which names its glyphs, does
	// say. Here code 1 is the glyph called alpha.
	d := pageWith(t, "BT /F2 10 Tf 20 100 Td (\x01\x02) Tj ET", func(w *reader.Writer, res reader.Dict) {
		prog := type1Program(map[byte]string{1: "alpha", 2: "beta"})
		res["Font"].(reader.Dict)["F2"] = w.Add(reader.Dict{
			"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
			"FontDescriptor": w.Add(reader.Dict{"Flags": reader.Integer(4),
				"FontFile": w.Add(&reader.Stream{
					Dict: reader.Dict{"Length1": reader.Integer(len(prog))}, Raw: prog})}),
		})
	})
	text, err := Text(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if text != "αβ" {
		t.Errorf("the page says %q, want %q", text, "αβ")
	}
}

func TestASymbolicTrueTypeFontThatSaysNothing(t *testing.T) {
	// A TrueType program is addressed through a character map rather than by
	// name, so a symbolic one with no ToUnicode says nothing about its text —
	// and that is reported rather than guessed at.
	d := pageWith(t, "BT /F2 10 Tf 20 100 Td (Hi) Tj ET", func(w *reader.Writer, res reader.Dict) {
		res["Font"].(reader.Dict)["F2"] = w.Add(embeddedFont(w, nil))
	})
	runs, err := Runs(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || !runs[0].Unreadable {
		t.Errorf("runs = %+v", runs)
	}
	// The same font with an encoding of its own is read by name.
	d = pageWith(t, "BT /F2 10 Tf 20 100 Td (Hi) Tj ET", func(w *reader.Writer, res reader.Dict) {
		res["Font"].(reader.Dict)["F2"] = w.Add(embeddedFont(w, reader.Name("WinAnsiEncoding")))
	})
	if text, _ := Text(d, 1); text != "Hi" {
		t.Errorf("the page says %q", text)
	}
}

func TestAFontWhoseProgramCannotBeRead(t *testing.T) {
	// A program that is not one leaves the font as it was: unread rather
	// than read wrong.
	d := pageWith(t, "BT /F2 10 Tf 20 100 Td (Hi) Tj ET", func(w *reader.Writer, res reader.Dict) {
		res["Font"].(reader.Dict)["F2"] = w.Add(reader.Dict{
			"Type": reader.Name("Font"), "Subtype": reader.Name("TrueType"),
			"FontDescriptor": w.Add(reader.Dict{"Flags": reader.Integer(4),
				"FontFile2": w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("not a font")})}),
		})
	})
	runs, err := Runs(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || !runs[0].Unreadable {
		t.Errorf("runs = %+v", runs)
	}
}

func TestWhichParserAProgramIsGivenTo(t *testing.T) {
	ttf := fonts.MostLegible()
	cases := []struct {
		name string
		key  reader.Name
		data []byte
		ok   bool
	}{
		{"a TrueType font under FontFile2", "FontFile2", ttf, true},
		{"an OpenType font under FontFile3", "FontFile3", ttf, true},
		{"a TrueType font under FontFile", "FontFile", ttf, false},
		{"nonsense under FontFile3", "FontFile3", []byte("not a font at all"), false},
		{"nonsense under FontFile2", "FontFile2", []byte("not a font at all"), false},
	}
	for _, c := range cases {
		f, err := readProgram(c.key, c.data)
		if (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok = %v", c.name, err, c.ok)
			continue
		}
		if c.ok && f.NumGlyphs() == 0 {
			t.Errorf("%s: read a font with no glyphs", c.name)
		}
	}
}

func TestAProgramIsNotAskedAboutACompositeFont(t *testing.T) {
	// A composite font is addressed by identifier; its program has nothing
	// to say about a byte.
	d := pageWith(t, "BT /F2 10 Tf 20 100 Td (\x00A) Tj ET", func(w *reader.Writer, res reader.Dict) {
		ttf := fonts.MostLegible()
		file := w.Add(&reader.Stream{Dict: reader.Dict{"Length1": reader.Integer(len(ttf))}, Raw: ttf})
		kid := w.Add(reader.Dict{"Subtype": reader.Name("CIDFontType2"),
			"FontDescriptor": w.Add(reader.Dict{"Flags": reader.Integer(4), "FontFile2": file})})
		res["Font"].(reader.Dict)["F2"] = w.Add(reader.Dict{
			"Subtype": reader.Name("Type0"), "Encoding": reader.Name("Identity-H"),
			"DescendantFonts": reader.Array{kid}})
	})
	runs, err := Runs(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || !runs[0].Unreadable {
		t.Errorf("runs = %+v", runs)
	}
}

func TestAProgramWhoseGlyphsAreNamedAfterNothing(t *testing.T) {
	// The three ways the last resort can come up empty: a code the program's
	// encoding does not name, a glyph the program does not name, and a name
	// that says nothing about which character it is.
	d := pageWith(t, "BT /F2 10 Tf 20 100 Td (\x01\x02) Tj ET", func(w *reader.Writer, res reader.Dict) {
		res["Font"].(reader.Dict)["F2"] = w.Add(embeddedFont(w, nil))
	})
	runs, err := Runs(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || !runs[0].Unreadable {
		t.Errorf("runs = %+v", runs)
	}
	// A code outside a byte cannot come from a simple font's string, but the
	// last resort is asked about one anyway when a composite font's codes
	// reach it, and says nothing.
	if strings.Contains(runs[0].Text, "�") {
		t.Error("a replacement character reached the text")
	}
}

func TestWhatTheProgramCannotSay(t *testing.T) {
	// A program answers only for the codes its own encoding names, and only
	// with names that say which character they are.
	d := pageWith(t, "BT /F2 10 Tf 20 100 Td (\x01\x09\x0a) Tj ET", func(w *reader.Writer, res reader.Dict) {
		// Code 1 is a glyph named after nothing in particular; code 9 is
		// named alpha; code 10 the program does not name at all.
		prog := type1Program(map[byte]string{1: "wingding", 9: "alpha"})
		res["Font"].(reader.Dict)["F2"] = w.Add(reader.Dict{
			"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
			"FontDescriptor": w.Add(reader.Dict{"Flags": reader.Integer(4),
				"FontFile": w.Add(&reader.Stream{
					Dict: reader.Dict{"Length1": reader.Integer(len(prog))}, Raw: prog})}),
		})
	})
	runs, err := Runs(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("%d runs", len(runs))
	}
	if runs[0].Text != "α" {
		t.Errorf("the run says %q, want %q", runs[0].Text, "α")
	}
	if !runs[0].Unreadable {
		t.Error("the codes nothing could name did not say so")
	}
}
