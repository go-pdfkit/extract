package extract

import (
	"github.com/go-opentype/opentype"
	"github.com/go-pdfkit/pdffont"
	"github.com/go-pdfkit/reader"
)

// attachProgram gives a font a last resort for the codes the document says
// nothing about: what its own embedded program calls them.
//
// A font that says it is symbolic is addressed through its own character map,
// and an assumed encoding is a bad guess at it — a mathematical font puts a
// capital gamma where the standard encoding puts an inverted exclamation mark.
// The program knows, and this is how it is asked.
func attachProgram(f *pdffont.Font) {
	if f.Kind() == pdffont.Composite {
		return
	}
	key, data, ok := f.Program()
	if !ok {
		return
	}
	program, err := readProgram(key, data)
	if err != nil {
		return
	}
	// Only a simple font reaches here, and its codes are bytes.
	f.SetFallback(func(code int) (string, bool) {
		gid, ok := program.GlyphIndexByCode(byte(code))
		if !ok {
			return "", false
		}
		// A glyph the program does not name comes back as no name at all,
		// which names no character either.
		name, _ := program.GlyphName(gid)
		r, ok := pdffont.RuneOfGlyphName(name)
		if !ok {
			return "", false
		}
		return string(r), true
	})
}

// readProgram decodes an embedded font program. Which key it arrived under
// says what it is: FontFile2 is TrueType, FontFile a PostScript Type 1
// program, and FontFile3 either a bare CFF one or a whole OpenType font — the
// key alone does not settle that one, so both are tried.
func readProgram(key reader.Name, data []byte) (*opentype.Font, error) {
	switch key {
	case "FontFile":
		return opentype.ParseType1(data)
	case "FontFile3":
		if f, err := opentype.Parse(data); err == nil {
			return f, nil
		}
		return opentype.ParseCFF(data)
	}
	return opentype.Parse(data)
}
