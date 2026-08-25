package extract

import (
	"github.com/go-pdfkit/pdffont"
	"github.com/go-pdfkit/reader"
)

// A matrix is the six numbers a PDF transform is written as. Extraction needs
// only to multiply them and to move a point, so it carries its own rather than
// depending on a drawing library it has no other use for.
type matrix struct{ a, b, c, d, e, f float64 }

// identity is the transform that changes nothing.
var identity = matrix{1, 0, 0, 1, 0, 0}

// mul is the transform that applies m first and then n, which is the order a
// PDF composes them in.
func (m matrix) mul(n matrix) matrix {
	return matrix{
		a: m.a*n.a + m.b*n.c,
		b: m.a*n.b + m.b*n.d,
		c: m.c*n.a + m.d*n.c,
		d: m.c*n.b + m.d*n.d,
		e: m.e*n.a + m.f*n.c + n.e,
		f: m.e*n.b + m.f*n.d + n.f,
	}
}

// apply moves a point.
func (m matrix) apply(x, y float64) (float64, float64) {
	return m.a*x + m.c*y + m.e, m.b*x + m.d*y + m.f
}

// scale is how much the transform stretches a length, taken as the geometric
// mean of its two axes so that a rotation counts as no stretch at all.
func (m matrix) scale() float64 {
	x := m.a*m.a + m.b*m.b
	y := m.c*m.c + m.d*m.d
	return sqrt(sqrt(x * y))
}

// sqrt without pulling in the whole of math for one use.
func sqrt(v float64) float64 {
	if v <= 0 {
		return 0
	}
	x := v
	for i := 0; i < 24; i++ {
		x = (x + v/x) / 2
	}
	return x
}

// A textState is everything the showing operators read besides the string.
type textState struct {
	font      *pdffont.Font
	fontName  string
	size      float64
	charSpace float64
	wordSpace float64
	scale     float64 // horizontal scaling, as a fraction
	leading   float64
	rise      float64
	mode      int
}

// A state is the graphics state, of which extraction needs only the transform
// and the text parameters.
type state struct {
	ctm  matrix
	text textState
}

// initialState is the transform a page starts in, which for extraction is the
// one that undoes the page's own box: text comes back in the page's
// coordinates, counting up from the bottom left of what is visible.
func initialState(d *reader.Document, page reader.Dict) state {
	box := pageBox(d, page)
	return state{
		ctm:  matrix{1, 0, 0, 1, -box[0], -box[1]},
		text: textState{scale: 1},
	}
}

// pageBox is the area of the page that is visible.
func pageBox(d *reader.Document, page reader.Dict) [4]float64 {
	for _, key := range []reader.Name{"CropBox", "MediaBox"} {
		if b, ok := rectangle(d, page.Get(key)); ok {
			return b
		}
	}
	return [4]float64{0, 0, 612, 792}
}

// rectangle reads a PDF rectangle, put the right way round.
func rectangle(d *reader.Document, o reader.Object) ([4]float64, bool) {
	var out [4]float64
	arr, ok := reader.ToArray(resolve(d, o))
	if !ok || len(arr) < 4 {
		return out, false
	}
	for i := 0; i < 4; i++ {
		v, ok := reader.ToFloat(resolve(d, arr[i]))
		if !ok {
			return out, false
		}
		out[i] = v
	}
	if out[0] > out[2] {
		out[0], out[2] = out[2], out[0]
	}
	if out[1] > out[3] {
		out[1], out[3] = out[3], out[1]
	}
	return out, true
}

// resolve follows an indirect reference. A document that opened cannot fail to
// resolve one.
func resolve(d *reader.Document, o reader.Object) reader.Object {
	out, _ := d.Resolve(o)
	return out
}

// numbers reads the operands that are numbers.
func numbers(operands []reader.Object) []float64 {
	out := make([]float64, 0, len(operands))
	for _, o := range operands {
		if v, ok := reader.ToFloat(o); ok {
			out = append(out, v)
		}
	}
	return out
}
