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

// cmapFont is a font whose descriptor says symbolic and whose dictionary says
// nothing else: no Encoding, no ToUnicode. Its own character map is then the
// only thing left that knows what its codes are.
func cmapFont(w *reader.Writer, specs []cmapSpec) reader.Dict {
	ttf := fontWithCmaps(specs)
	file := w.Add(&reader.Stream{Dict: reader.Dict{"Length1": reader.Integer(len(ttf))}, Raw: ttf})
	return reader.Dict{
		"Type": reader.Name("Font"), "Subtype": reader.Name("TrueType"),
		"BaseFont": reader.Name("Test"),
		"FontDescriptor": w.Add(reader.Dict{
			"Type": reader.Name("FontDescriptor"), "FontName": reader.Name("Test"),
			"Flags": reader.Integer(4), "FontFile2": file}),
	}
}

// saysThrough reads one page drawn in a font carrying exactly these subtables.
func saysThrough(t *testing.T, content string, specs []cmapSpec) string {
	t.Helper()
	d := pageWith(t, content, func(w *reader.Writer, res reader.Dict) {
		res["Font"].(reader.Dict)["F2"] = w.Add(cmapFont(w, specs))
	})
	text, err := Text(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func TestASymbolicTrueTypeFontReadThroughItsCharacterMap(t *testing.T) {
	// The way in is the Microsoft Symbol subtable, whose codes conventionally
	// live at 0xF000 + code; the way out is the Unicode subtable, inverted.
	// Code 'A' reaches glyph 5 at 0xF041, and the font says glyph 5 is 'Z'.
	text := saysThrough(t, "BT /F2 10 Tf 20 100 Td (A) Tj ET", []cmapSpec{
		{3, 0, map[rune]uint16{0xF041: 5}},
		{3, 1, map[rune]uint16{'Z': 5}},
	})
	if text != "Z" {
		t.Errorf("the page says %q, want %q", text, "Z")
	}
}

func TestACodeAddressedWithoutTheSymbolOffset(t *testing.T) {
	// A symbol subtable written at the raw code rather than at 0xF000 + code
	// is tried first, which is the order poppler uses.
	text := saysThrough(t, "BT /F2 10 Tf 20 100 Td (B) Tj ET", []cmapSpec{
		{3, 0, map[rune]uint16{'B': 7}},
		{3, 1, map[rune]uint16{'W': 7}},
	})
	if text != "W" {
		t.Errorf("the page says %q, want %q", text, "W")
	}
}

func TestAFontAddressedThroughItsMacintoshRomanMap(t *testing.T) {
	// With no Microsoft Symbol subtable the Macintosh Roman one is the way in.
	// Platform 0 serves as the way out just as Microsoft Unicode does.
	text := saysThrough(t, "BT /F2 10 Tf 20 100 Td (A) Tj ET", []cmapSpec{
		{1, 0, map[rune]uint16{'A': 6}},
		{0, 3, map[rune]uint16{'Q': 6}},
	})
	if text != "Q" {
		t.Errorf("the page says %q, want %q", text, "Q")
	}
}

func TestTheFirstSubtableOfAKindIsTheOneTaken(t *testing.T) {
	// A font that repeats a kind of subtable means the first of them.
	text := saysThrough(t, "BT /F2 10 Tf 20 100 Td (A) Tj ET", []cmapSpec{
		{3, 0, map[rune]uint16{0xF041: 5}},
		{3, 0, map[rune]uint16{0xF041: 8}},
		{1, 0, map[rune]uint16{'A': 8}},
		{1, 0, map[rune]uint16{'A': 9}},
		{3, 1, map[rune]uint16{'Z': 5}},
		{3, 10, map[rune]uint16{'Y': 5}},
	})
	if text != "Z" {
		t.Errorf("the page says %q, want %q", text, "Z")
	}
}

func TestWhatTheCharacterMapRouteRefusesToSay(t *testing.T) {
	// Every way this can honestly come up empty. A guess would be exactly the
	// wrong letter the symbolic guard exists to refuse, so it says nothing.
	cases := []struct {
		why   string
		specs []cmapSpec
	}{
		{"no way out: nothing says which character a glyph is", []cmapSpec{
			{3, 0, map[rune]uint16{0xF041: 5}},
		}},
		{"no way in: the font is not addressed by its own codes", []cmapSpec{
			{3, 1, map[rune]uint16{'A': 5}},
		}},
		{"no subtable of any kind this route knows", []cmapSpec{
			{7, 7, map[rune]uint16{'A': 5}},
		}},
		{"the code reaches no glyph, at either address", []cmapSpec{
			{3, 0, map[rune]uint16{0xF042: 5}},
			{3, 1, map[rune]uint16{'Z': 5}},
		}},
		{"no code in the Unicode map reaches that glyph", []cmapSpec{
			{1, 0, map[rune]uint16{'A': 9}},
			{3, 1, map[rune]uint16{'Z': 5}},
		}},
		{"the glyph is a private-use character, which nothing can read", []cmapSpec{
			{1, 0, map[rune]uint16{'A': 5}},
			{3, 1, map[rune]uint16{0xE000: 5}},
		}},
		{"the glyph is a control character, which a page does not say", []cmapSpec{
			{1, 0, map[rune]uint16{'A': 5}},
			{3, 1, map[rune]uint16{0x0B: 5}},
		}},
		{"the glyph is a C1 control character", []cmapSpec{
			{1, 0, map[rune]uint16{'A': 5}},
			{3, 1, map[rune]uint16{0x85: 5}},
		}},
	}
	for _, c := range cases {
		if text := saysThrough(t, "BT /F2 10 Tf 20 100 Td (A) Tj ET", c.specs); text != "" {
			t.Errorf("%s: the page says %q, want nothing", c.why, text)
		}
	}
}

func TestWhichCharactersAreWorthReporting(t *testing.T) {
	// A format-4 subtable cannot reach beyond the basic plane, so the high
	// private-use planes are checked here rather than through a whole page.
	cases := map[rune]bool{
		'A':      true,
		' ':      true,
		0x00:     false, // NUL
		0x1F:     false, // a C0 control
		0x7F:     false, // delete
		0x85:     false, // a C1 control
		0xA0:     true,  // no-break space, the first character past them
		0xE000:   false, // private use
		0xF8FF:   false, // private use, last
		0xF900:   true,  // a compatibility ideograph, just past it
		0x1F600:  true,  // an astral character a font may really mean
		0xF0000:  false, // supplementary private use area A
		0x10FFFF: false, // supplementary private use area B
	}
	for r, want := range cases {
		if got := readableRune(r); got != want {
			t.Errorf("readableRune(%#x) = %v want %v", r, got, want)
		}
	}
}
