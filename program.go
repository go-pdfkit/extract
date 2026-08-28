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
//
// There are two ways to ask, and a program answers at most one of them. A
// PostScript program names its glyphs and carries an encoding from code to
// name, so the name is the answer. A TrueType program usually does neither —
// a subset of one names nothing worth reading and has no code-to-name
// encoding at all — but it does carry character maps, and walking through one
// to the glyph and back out of another says which character the glyph is for.
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
	maps := chooseCharacterMaps(program)
	// Only a simple font reaches here, and its codes are bytes.
	f.SetFallback(func(code int) (string, bool) {
		if r, ok := runeByName(program, code); ok {
			return string(r), true
		}
		if r, ok := runeByCharacterMap(program, maps, code); ok {
			return string(r), true
		}
		return "", false
	})
}

// runeByName asks the program's own encoding what it calls a code, and reads
// the character out of that name. Only a PostScript program — a Type 1 one, or
// the CFF outlines of an OpenType font — carries such an encoding.
func runeByName(program *opentype.Font, code int) (rune, bool) {
	gid, ok := program.GlyphIndexByCode(byte(code))
	if !ok {
		return 0, false
	}
	// A glyph the program does not name comes back as no name at all,
	// which names no character either.
	name, _ := program.GlyphName(gid)
	return pdffont.RuneOfGlyphName(name)
}

// characterMaps says which of a program's cmap subtables are worth addressing,
// by index, or -1 for one the program does not carry.
type characterMaps struct {
	// symbol is the Microsoft Symbol subtable, platform 3 encoding 0: a font's
	// own codes, conventionally written at 0xF000 + code.
	symbol int
	// mac is the Macintosh Roman subtable, platform 1 encoding 0, indexed by
	// single bytes.
	mac int
	// unicode is a Unicode subtable — Microsoft Unicode, platform 3 encoding 1
	// or 10, or anything on platform 0 — indexed by codepoint. This is the one
	// that is inverted, because it is the only one whose codes are characters.
	unicode int
}

// chooseCharacterMaps picks out the subtables a font is addressed through and
// the one that says which character a glyph is for.
//
// A font can carry several subtables of a kind; the first of each is taken,
// which is what a font that repeats one means by repeating it.
func chooseCharacterMaps(program *opentype.Font) characterMaps {
	m := characterMaps{symbol: -1, mac: -1, unicode: -1}
	for i := range program.NumCharacterMaps() {
		platform, encoding, _, _ := program.CharacterMap(i)
		switch {
		case platform == 3 && encoding == 0:
			if m.symbol < 0 {
				m.symbol = i
			}
		case platform == 1 && encoding == 0:
			if m.mac < 0 {
				m.mac = i
			}
		case platform == 0, platform == 3 && (encoding == 1 || encoding == 10):
			if m.unicode < 0 {
				m.unicode = i
			}
		}
	}
	return m
}

// runeByCharacterMap walks a code through the font's own character map to a
// glyph, and back out of its Unicode character map to the character that glyph
// stands for.
//
// The way in is the one poppler uses for a font the document gave no encoding:
// the Microsoft Symbol subtable if there is one, else the Macintosh Roman one,
// addressed by the raw code and then, failing that, by 0xF000 + code, which is
// where such subtables are conventionally written.
//
// The way out is the Unicode subtable, inverted. Without one there is no way
// out: a glyph on its own says nothing about which character it is, and a
// guess would be exactly the wrong letter the caller's guard exists to refuse.
func runeByCharacterMap(program *opentype.Font, m characterMaps, code int) (rune, bool) {
	if m.unicode < 0 {
		return 0, false
	}
	in := m.symbol
	if in < 0 {
		in = m.mac
	}
	if in < 0 {
		return 0, false
	}
	gid, ok := program.GlyphIndexInMap(in, rune(code))
	if !ok {
		gid, ok = program.GlyphIndexInMap(in, rune(0xF000|code))
	}
	if !ok {
		return 0, false
	}
	r, ok := program.RuneOfGlyphInMap(m.unicode, gid)
	if !ok || !readableRune(r) {
		return 0, false
	}
	return r, true
}

// readableRune reports whether a character recovered this way is worth
// reporting as text.
//
// A private-use codepoint is not: it means whatever the font decided it means
// and nothing outside the font can read it, so a page full of them searches no
// better than a page of nothing and looks, wrongly, like it was read. Control
// characters are refused for the same reason — a page does not say them.
func readableRune(r rune) bool {
	switch {
	case r < 0x20, r >= 0x7F && r <= 0x9F:
		return false
	case r >= 0xE000 && r <= 0xF8FF:
		return false
	case r >= 0xF0000:
		return false
	}
	return true
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
