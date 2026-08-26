package extract

import (
	"strings"

	"github.com/go-pdfkit/pdffont"
	"github.com/go-pdfkit/reader"
)

// maxFormDepth is how deeply one form may draw another before a page is taken
// to be drawing itself.
const maxFormDepth = 12

// An extractor walks a content stream keeping just enough state to say where
// each piece of text was drawn.
type extractor struct {
	doc   *reader.Document
	runs  []Run
	fonts map[int]*pdffont.Font
	// tm is where the next glyph goes and tlm where the line began; both
	// belong to the page rather than to the graphics state.
	tm, tlm matrix
	images  []Image
	// wantPictures says whether anybody is going to read Image.Data. When
	// nobody is, the filters are left undone.
	wantPictures bool
}

// run walks a content stream.
func (e *extractor) run(content []byte, resources reader.Dict, g state, depth int) {
	if depth > maxFormDepth {
		return
	}
	// The operations are taken one at a time rather than all at once. The
	// walk only ever goes forwards, so nothing needs the list; keeping it
	// costs a struct and a slice header for every operator on the page, and
	// pages are not always small. One arXiv figure holds a single page of
	// 84.8 MB of content stream and 4 579 973 operations: holding them cost
	// 2 166 MB where reading them one by one costs 935 MB.
	scan := reader.NewContentScanner(content)
	var stack []state
	for {
		op, more := scan.Next()
		if !more {
			break
		}
		n := numbers(op.Operands)
		switch op.Operator {
		case "q":
			stack = append(stack, g)
		case "Q":
			if len(stack) > 0 {
				g = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
		case "cm":
			if len(n) >= 6 {
				g.ctm = matrix{n[0], n[1], n[2], n[3], n[4], n[5]}.mul(g.ctm)
			}
		case "BT":
			e.tm, e.tlm = identity, identity
		case "ET":
			e.tm, e.tlm = identity, identity
		case "Tf":
			e.setFont(&g, op.Operands, n, resources)
		case "Tc":
			if len(n) >= 1 {
				g.text.charSpace = n[0]
			}
		case "Tw":
			if len(n) >= 1 {
				g.text.wordSpace = n[0]
			}
		case "Tz":
			if len(n) >= 1 {
				g.text.scale = n[0] / 100
			}
		case "TL":
			if len(n) >= 1 {
				g.text.leading = n[0]
			}
		case "Ts":
			if len(n) >= 1 {
				g.text.rise = n[0]
			}
		case "Tr":
			if len(n) >= 1 {
				g.text.mode = int(n[0])
			}
		case "Td":
			if len(n) >= 2 {
				e.tlm = matrix{1, 0, 0, 1, n[0], n[1]}.mul(e.tlm)
				e.tm = e.tlm
			}
		case "TD":
			if len(n) >= 2 {
				g.text.leading = -n[1]
				e.tlm = matrix{1, 0, 0, 1, n[0], n[1]}.mul(e.tlm)
				e.tm = e.tlm
			}
		case "Tm":
			if len(n) >= 6 {
				e.tlm = matrix{n[0], n[1], n[2], n[3], n[4], n[5]}
				e.tm = e.tlm
			}
		case "T*":
			e.nextLine(&g)
		case "Tj":
			e.showOperands(&g, op.Operands)
		case "'":
			e.nextLine(&g)
			e.showOperands(&g, op.Operands)
		case "\"":
			if len(n) >= 2 {
				g.text.wordSpace, g.text.charSpace = n[0], n[1]
			}
			e.nextLine(&g)
			e.showOperands(&g, op.Operands)
		case "TJ":
			e.showArray(&g, op.Operands)
		case "Do":
			e.doXObject(&g, op.Operands, resources, depth)
		case "BI":
			e.inlineImage(&g, op.Image)
		}
	}
}

// setFont reads the font a Tf operator names.
func (e *extractor) setFont(g *state, operands []reader.Object, n []float64, resources reader.Dict) {
	if len(operands) < 2 {
		return
	}
	name, ok := reader.ToName(operands[0])
	if !ok {
		return
	}
	if len(n) >= 1 {
		g.text.size = n[len(n)-1]
	}
	g.text.fontName = string(name)
	g.text.font = e.font(name, resources)
}

// font reads a font out of the page's resources, remembering the ones already
// read so that a page setting the same font a thousand times reads it once.
func (e *extractor) font(name reader.Name, resources reader.Dict) *pdffont.Font {
	fonts, ok := e.doc.GetDict(resources, "Font")
	if !ok {
		return nil
	}
	if ref, ok := fonts.Get(name).(reader.Ref); ok {
		if f, seen := e.fonts[ref.Num]; seen {
			return f
		}
		dict, ok := e.doc.GetDict(fonts, name)
		if !ok {
			return nil
		}
		f := pdffont.Read(e.doc, dict)
		attachProgram(f)
		e.fonts[ref.Num] = f
		return f
	}
	dict, ok := e.doc.GetDict(fonts, name)
	if !ok {
		return nil
	}
	f := pdffont.Read(e.doc, dict)
	attachProgram(f)
	return f
}

// nextLine moves down by the leading, which is what T* and the quote
// operators do.
func (e *extractor) nextLine(g *state) {
	e.tlm = matrix{1, 0, 0, 1, 0, -g.text.leading}.mul(e.tlm)
	e.tm = e.tlm
}

// showOperands shows the string a Tj-like operator carries.
func (e *extractor) showOperands(g *state, operands []reader.Object) {
	for _, o := range operands {
		if s, ok := reader.ToString(o); ok {
			e.show(g, s)
			return
		}
	}
}

// showArray shows the pieces a TJ operator carries, moving the pen by the
// numbers between them.
func (e *extractor) showArray(g *state, operands []reader.Object) {
	for _, o := range operands {
		arr, ok := reader.ToArray(o)
		if !ok {
			continue
		}
		for _, item := range arr {
			if s, ok := reader.ToString(item); ok {
				e.show(g, s)
				continue
			}
			if v, ok := reader.ToFloat(item); ok {
				e.advance(g, -v/1000*g.text.size*g.text.scale)
			}
		}
	}
}

// show reads one string as text and records where it was drawn.
func (e *extractor) show(g *state, s []byte) {
	f := g.text.font
	if f == nil || len(s) == 0 {
		return
	}
	start := e.tm.mul(g.ctm)
	x, y := start.apply(0, g.text.rise)
	var text strings.Builder
	unreadable := false
	width := 0.0
	for _, code := range f.Codes(s) {
		if t, ok := f.Text(code); ok {
			text.WriteString(pdffont.TrimText(t))
		} else {
			unreadable = true
		}
		adv := (f.Width(code)*g.text.size + g.text.charSpace) * g.text.scale
		if f.Kind() != pdffont.Composite && code == ' ' {
			adv += g.text.wordSpace * g.text.scale
		}
		width += adv
	}
	e.advance(g, width)
	if text.Len() == 0 && !unreadable {
		return
	}
	// How far a length in text space reaches on the page: the text matrix and
	// the page's transform together, which is the same matrix the run's own
	// position came through.
	//
	// Only the page's transform used to be counted here, and the text matrix
	// was left out. A document is free to put the whole of its scale there —
	// "/F1 1 Tf" and then a text matrix of ten is how TeX writes every PDF it
	// has ever produced — and such a document reported its text as one point
	// tall with letters half a point wide, while saying quite correctly where
	// on the page each run began.
	//
	// Those are the two numbers a word break is decided by. A gap measured in
	// points was being weighed against a space width ten times too small, so
	// every ordinary kern between two letters looked like a space: "Original
	// Domain" came back as "Or ig inal D omain". Across four thousand arXiv
	// figures, 57% of the words a reader got back were one or two letters
	// long. Nothing failed, and nothing was slow; the text was simply no
	// longer text.
	scale := start.scale()
	e.runs = append(e.runs, Run{
		Text:       text.String(),
		X:          x,
		Y:          y,
		Width:      width * scale,
		Size:       g.text.size * scale,
		Font:       g.text.fontName,
		Space:      f.Width(' ') * g.text.size * g.text.scale * scale,
		Invisible:  g.text.mode == 3,
		Unreadable: unreadable,
	})
}

// advance moves the pen along the line by a distance in text space.
func (e *extractor) advance(g *state, by float64) {
	e.tm = matrix{1, 0, 0, 1, by, 0}.mul(e.tm)
}
