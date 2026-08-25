package extract

import (
	"testing"

	"github.com/go-opentype/fonts"
	"github.com/go-pdfkit/reader"
)

// pageWith builds a one-page document whose content is what the test wants and
// whose resources hold a real embedded font under /F1.
func pageWith(t *testing.T, content string, extra func(w *reader.Writer, res reader.Dict)) *reader.Document {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	res := reader.Dict{"Font": reader.Dict{"F1": w.Add(simpleFont(w))}}
	if extra != nil {
		extra(w, res)
	}
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef,
		"MediaBox":  reader.Array{reader.Integer(0), reader.Integer(0), reader.Integer(200), reader.Integer(200)},
		"Contents":  w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(content)}),
		"Resources": res,
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
	return d
}

// simpleFont is a WinAnsi font of even width, so that a test can work out
// where every letter should land.
func simpleFont(w *reader.Writer) reader.Dict {
	widths := make(reader.Array, 0, 224)
	for i := 32; i < 256; i++ {
		widths = append(widths, reader.Integer(500))
	}
	return reader.Dict{
		"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
		"BaseFont": reader.Name("Helvetica"), "FirstChar": reader.Integer(32),
		"LastChar": reader.Integer(255), "Widths": widths,
		"Encoding": reader.Name("WinAnsiEncoding"),
	}
}

// embeddedFont is a font carrying a real TrueType program, for the tests that
// need one to be read.
func embeddedFont(w *reader.Writer, encoding reader.Object) reader.Dict {
	ttf := fonts.MostLegible()
	file := w.Add(&reader.Stream{Dict: reader.Dict{"Length1": reader.Integer(len(ttf))}, Raw: ttf})
	widths := make(reader.Array, 0, 224)
	for i := 32; i < 256; i++ {
		widths = append(widths, reader.Integer(500))
	}
	dict := reader.Dict{
		"Type": reader.Name("Font"), "Subtype": reader.Name("TrueType"),
		"BaseFont": reader.Name("Test"), "FirstChar": reader.Integer(32),
		"LastChar": reader.Integer(255), "Widths": widths,
		"FontDescriptor": w.Add(reader.Dict{
			"Type": reader.Name("FontDescriptor"), "FontName": reader.Name("Test"),
			"Flags": reader.Integer(4), "FontFile2": file}),
	}
	if encoding != nil {
		dict["Encoding"] = encoding
	}
	return dict
}
