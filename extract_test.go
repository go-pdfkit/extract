package extract

import (
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

func TestReadingAPageBack(t *testing.T) {
	d := pageWith(t, "BT /F1 12 Tf 20 100 Td (Hello) Tj ET", nil)
	text, err := Text(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if text != "Hello" {
		t.Errorf("the page says %q", text)
	}
	runs, err := Runs(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("%d runs", len(runs))
	}
	r := runs[0]
	if r.X != 20 || r.Y != 100 {
		t.Errorf("the run starts at (%v,%v), want (20,100)", r.X, r.Y)
	}
	if r.Size != 12 {
		t.Errorf("the run is %v tall", r.Size)
	}
	// Five letters of half an em at twelve points.
	if r.Width != 30 {
		t.Errorf("the run is %v wide, want 30", r.Width)
	}
	if r.Font != "F1" {
		t.Errorf("the run is set in %q", r.Font)
	}
	if r.Invisible || r.Unreadable {
		t.Errorf("the run says %+v", r)
	}
}

func TestWhereTheWordsAre(t *testing.T) {
	// Two pieces on the same line with a gap between them are two words; two
	// pieces touching are one.
	d := pageWith(t, "BT /F1 10 Tf 20 100 Td (one) Tj 40 0 Td (two) Tj ET", nil)
	text, _ := Text(d, 1)
	if text != "one two" {
		t.Errorf("the page says %q", text)
	}
	d = pageWith(t, "BT /F1 10 Tf 20 100 Td (on) Tj 10 0 Td (e) Tj ET", nil)
	text, _ = Text(d, 1)
	if text != "one" {
		t.Errorf("the page says %q", text)
	}
	// A piece already ending in a space is not given another.
	d = pageWith(t, "BT /F1 10 Tf 20 100 Td (one ) Tj 40 0 Td (two) Tj ET", nil)
	if text, _ := Text(d, 1); text != "one two" {
		t.Errorf("the page says %q", text)
	}
}

func TestWhereTheLinesAre(t *testing.T) {
	// Pieces on different baselines are different lines, in the order they
	// come down the page whatever order they were drawn in.
	d := pageWith(t, "BT /F1 10 Tf 20 40 Td (bottom) Tj 0 60 Td (top) Tj ET", nil)
	text, err := Text(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if text != "top\nbottom" {
		t.Errorf("the page says %q", text)
	}
	// A slight wobble is still one line: a superscript sits a little high.
	d = pageWith(t, "BT /F1 10 Tf 20 100 Td (x) Tj 6 2 Td (2) Tj ET", nil)
	if text, _ := Text(d, 1); text != "x2" && text != "x 2" {
		t.Errorf("a superscript made a new line: %q", text)
	}
}

func TestEveryWayToMoveTheTextAlong(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"Td", "BT /F1 10 Tf 20 100 Td (a) Tj ET", "a"},
		{"TD and the leading it sets", "BT /F1 10 Tf 20 100 Td (a) Tj 0 -20 TD (b) Tj T* (c) Tj ET", "a\nb\nc"},
		{"Tm", "BT /F1 10 Tf 1 0 0 1 20 100 Tm (a) Tj ET", "a"},
		{"TL and T*", "BT /F1 10 Tf 20 100 Td 20 TL (a) Tj T* (b) Tj ET", "a\nb"},
		{"the quote operator", "BT /F1 10 Tf 20 100 Td 20 TL (a) Tj (b) ' ET", "a\nb"},
		{"the double quote operator", "BT /F1 10 Tf 20 100 Td 20 TL (a) Tj 0 0 (b) \" ET", "a\nb"},
		{"TJ with a kern between", "BT /F1 10 Tf 20 100 Td [(a) -500 (b)] TJ ET", "a b"},
		{"TJ with no kern", "BT /F1 10 Tf 20 100 Td [(a) (b)] TJ ET", "ab"},
		{"character spacing", "BT /F1 10 Tf 2 Tc 20 100 Td (ab) Tj ET", "ab"},
		{"word spacing", "BT /F1 10 Tf 5 Tw 20 100 Td (a b) Tj ET", "a b"},
		{"horizontal scaling", "BT /F1 10 Tf 50 Tz 20 100 Td (ab) Tj ET", "ab"},
		{"a rise", "BT /F1 10 Tf 4 Ts 20 100 Td (a) Tj ET", "a"},
		{"a transform on the page", "q 2 0 0 2 0 0 cm BT /F1 10 Tf 20 100 Td (a) Tj ET Q", "a"},
		{"a saved and restored state", "q BT /F1 10 Tf 20 100 Td (a) Tj ET Q BT /F1 10 Tf 20 60 Td (b) Tj ET", "a\nb"},
		{"a restore with nothing saved", "Q BT /F1 10 Tf 20 100 Td (a) Tj ET", "a"},
	}
	for _, c := range cases {
		d := pageWith(t, c.content, nil)
		got, err := Text(d, 1)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestTextTheDocumentSaysNothingAbout(t *testing.T) {
	// A symbolic font with no map and no names of its own cannot be read
	// back through a guess, and says so rather than giving a wrong letter.
	d := pageWith(t, "BT /F2 10 Tf 20 100 Td (\x01\x02) Tj ET", func(w *reader.Writer, res reader.Dict) {
		fonts, _ := res["Font"].(reader.Dict)
		fonts["F2"] = w.Add(reader.Dict{
			"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
			"FontDescriptor": w.Add(reader.Dict{"Flags": reader.Integer(4)}),
		})
	})
	runs, err := Runs(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("%d runs", len(runs))
	}
	if !runs[0].Unreadable {
		t.Error("a run nothing could read did not say so")
	}
	if runs[0].Text != "" {
		t.Errorf("it came back as %q", runs[0].Text)
	}
}

func TestATextLayerUnderAPicture(t *testing.T) {
	// Text drawn in the mode that puts no ink on the page is how a scanner
	// says what it read. It is text worth having, and worth knowing about.
	d := pageWith(t, "BT 3 Tr /F1 10 Tf 20 100 Td (scanned) Tj ET", nil)
	runs, err := Runs(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || !runs[0].Invisible {
		t.Fatalf("runs = %+v", runs)
	}
	if text, _ := Text(d, 1); text != "scanned" {
		t.Errorf("the page says %q", text)
	}
}

func TestAPageWithNothingOnIt(t *testing.T) {
	d := pageWith(t, "", nil)
	if text, err := Text(d, 1); err != nil || text != "" {
		t.Errorf("an empty page says %q (%v)", text, err)
	}
	if runs, err := Runs(d, 1); err != nil || len(runs) != 0 {
		t.Errorf("an empty page has %d runs (%v)", len(runs), err)
	}
	if got := Assemble(nil); got != "" {
		t.Errorf("nothing assembled into %q", got)
	}
}

func TestAPageThatIsNotThere(t *testing.T) {
	d := pageWith(t, "BT ET", nil)
	for _, i := range []int{0, 2, 99} {
		if _, err := Text(d, i); err == nil {
			t.Errorf("page %d was read", i)
		}
		if _, err := Runs(d, i); err == nil {
			t.Errorf("page %d gave runs", i)
		}
		if _, err := Images(d, i); err == nil {
			t.Errorf("page %d gave images", i)
		}
	}
}

func TestOperatorsGivenNothingToWorkWith(t *testing.T) {
	// Every showing and moving operator, handed the wrong operands or none.
	contents := []string{
		"BT Tf (a) Tj ET",
		"BT /F1 Tf (a) Tj ET",
		"BT 7 12 Tf (a) Tj ET",
		"BT /Missing 12 Tf (a) Tj ET",
		"BT /F1 12 Tf Td (a) Tj ET",
		"BT /F1 12 Tf TD (a) Tj ET",
		"BT /F1 12 Tf Tm (a) Tj ET",
		"BT /F1 12 Tf Tc Tw Tz TL Ts Tr (a) Tj ET",
		"BT /F1 12 Tf 20 100 Td Tj ET",
		"BT /F1 12 Tf 20 100 Td 7 Tj ET",
		"BT /F1 12 Tf 20 100 Td TJ ET",
		"BT /F1 12 Tf 20 100 Td 7 TJ ET",
		"BT /F1 12 Tf 20 100 Td [7 (a) /x] TJ ET",
		"BT /F1 12 Tf 20 100 Td () Tj ET",
		"BT /F1 12 Tf 20 100 Td (a) ' ET",
		"BT 20 100 Td (a) Tj ET",
		"cm BT ET",
	}
	for _, content := range contents {
		d := pageWith(t, content, nil)
		if _, err := Text(d, 1); err != nil {
			t.Errorf("%q: %v", content, err)
		}
	}
}

func TestAPageWithNoResourcesAtAll(t *testing.T) {
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	page := w.Add(reader.Dict{"Type": reader.Name("Page"), "Parent": pagesRef,
		"Contents": w.Add(&reader.Stream{Dict: reader.Dict{},
			Raw: []byte("BT /F1 12 Tf 20 100 Td (a) Tj ET /X Do")})})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1)})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	d, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	if text, err := Text(d, 1); err != nil || text != "" {
		t.Errorf("a page with no resources says %q (%v)", text, err)
	}
}

func TestTheSameFontNamedOverAndOver(t *testing.T) {
	// A page setting the same font a hundred times reads it once, and gets
	// the same answer every time.
	var content strings.Builder
	content.WriteString("BT 20 100 Td ")
	for i := 0; i < 100; i++ {
		content.WriteString("/F1 10 Tf (a) Tj ")
	}
	content.WriteString("ET")
	d := pageWith(t, content.String(), nil)
	runs, err := Runs(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 100 {
		t.Errorf("%d runs", len(runs))
	}
}

func TestThePagesEdges(t *testing.T) {
	// Everything a page can say that is out of the ordinary, and what it is
	// read as.
	cases := []struct {
		name    string
		content string
		page    func(w *reader.Writer, page reader.Dict)
		want    string
	}{
		{"a box that is not numbers", "BT /F1 10 Tf 20 100 Td (a) Tj ET",
			func(w *reader.Writer, page reader.Dict) {
				page["MediaBox"] = reader.Array{reader.Name("x"), reader.Integer(0),
					reader.Integer(200), reader.Integer(200)}
			}, "a"},
		{"a box with too few numbers", "BT /F1 10 Tf 20 100 Td (a) Tj ET",
			func(w *reader.Writer, page reader.Dict) {
				page["MediaBox"] = reader.Array{reader.Integer(0)}
			}, "a"},
		{"a box written back to front", "BT /F1 10 Tf 20 100 Td (a) Tj ET",
			func(w *reader.Writer, page reader.Dict) {
				page["MediaBox"] = reader.Array{reader.Integer(200), reader.Integer(200),
					reader.Integer(0), reader.Integer(0)}
			}, "a"},
		{"a crop box that is not the paper", "BT /F1 10 Tf 20 100 Td (a) Tj ET",
			func(w *reader.Writer, page reader.Dict) {
				page["CropBox"] = reader.Array{reader.Integer(10), reader.Integer(10),
					reader.Integer(190), reader.Integer(190)}
			}, "a"},
		{"text of no size at all", "BT /F1 0 Tf 20 100 Td (a) Tj 0 -1 Td (b) Tj ET", nil, "a\nb"},
		{"a transform that flattens the page", "0 0 0 0 0 0 cm BT /F1 10 Tf 20 100 Td (a) Tj ET", nil, "a"},
	}
	for _, c := range cases {
		w := reader.NewWriter("1.7")
		pagesRef := w.Reserve()
		page := reader.Dict{
			"Type": reader.Name("Page"), "Parent": pagesRef,
			"MediaBox":  reader.Array{reader.Integer(0), reader.Integer(0), reader.Integer(200), reader.Integer(200)},
			"Contents":  w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(c.content)}),
			"Resources": reader.Dict{"Font": reader.Dict{"F1": w.Add(simpleFont(w))}},
		}
		if c.page != nil {
			c.page(w, page)
		}
		w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
			"Kids": reader.Array{w.Add(page)}, "Count": reader.Integer(1)})
		root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
		out, err := w.Finish(reader.Dict{"Root": root})
		if err != nil {
			t.Fatal(err)
		}
		d, err := reader.Open(out)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Text(d, 1)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestAFontWrittenIntoThePageRatherThanNamed(t *testing.T) {
	// A font may sit in the resources as a dictionary rather than as a
	// reference to one, in which case there is nothing to remember it by.
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef,
		"MediaBox":  reader.Array{reader.Integer(0), reader.Integer(0), reader.Integer(200), reader.Integer(200)},
		"Contents":  w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("BT /F1 10 Tf 20 100 Td (a) Tj ET")}),
		"Resources": reader.Dict{"Font": reader.Dict{"F1": simpleFont(w)}},
	})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1)})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	d, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	if text, _ := Text(d, 1); text != "a" {
		t.Errorf("the page says %q", text)
	}
	// And one named by a reference to something that is not a dictionary.
	d = pageWith(t, "BT /F2 10 Tf 20 100 Td (a) Tj ET", func(w *reader.Writer, res reader.Dict) {
		res["Font"].(reader.Dict)["F2"] = w.Add(reader.Integer(3))
	})
	if text, _ := Text(d, 1); text != "" {
		t.Errorf("a font that is not one gave %q", text)
	}
}

func TestACodeThatStandsForNothingAtAll(t *testing.T) {
	// A map may say a code means no characters at all, which is neither text
	// nor a failure to read one.
	d := pageWith(t, "BT /F2 10 Tf 20 100 Td (A) Tj ET", func(w *reader.Writer, res reader.Dict) {
		res["Font"].(reader.Dict)["F2"] = w.Add(reader.Dict{
			"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
			"FontDescriptor": w.Add(reader.Dict{"Flags": reader.Integer(4)}),
			"ToUnicode": w.Add(&reader.Stream{Dict: reader.Dict{},
				Raw: []byte("beginbfchar <41> <0000> endbfchar")}),
		})
	})
	runs, err := Runs(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Errorf("a code standing for nothing gave %+v", runs)
	}
}

func TestAWordGapInAFontWithNoSpaceOfItsOwn(t *testing.T) {
	// A Type 3 font gives no width to a code it does not draw, so the gap
	// that makes a word break has to come from the size instead.
	d := pageWith(t, "BT /F3 10 Tf 20 100 Td (a) Tj 20 0 Td (b) Tj ET", func(w *reader.Writer, res reader.Dict) {
		res["Font"].(reader.Dict)["F3"] = w.Add(reader.Dict{
			"Type": reader.Name("Font"), "Subtype": reader.Name("Type3"),
			"FontMatrix": reader.Array{reader.Real(0.001), reader.Integer(0), reader.Integer(0),
				reader.Real(0.001), reader.Integer(0), reader.Integer(0)},
			"Encoding": w.Add(reader.Dict{"Differences": reader.Array{
				reader.Integer(97), reader.Name("a"), reader.Name("b")}}),
			"CharProcs": reader.Dict{},
		})
	})
	if text, _ := Text(d, 1); text != "a b" {
		t.Errorf("the page says %q", text)
	}
}
