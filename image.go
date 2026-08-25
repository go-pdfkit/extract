package extract

import (
	"github.com/go-pdfkit/reader"
)

// An Image is a picture the page places, and where it puts it.
//
// What comes back is the picture as the file holds it — the bytes and the
// dictionary describing them — rather than pixels. A JPEG comes out as a JPEG,
// which is what anyone pulling pictures out of a document wants: re-encoding
// it would lose something for nothing. Turning the other kinds into pixels
// means reading their colour space, which is the renderer's work rather than
// this package's.
type Image struct {
	// Name is what the page's resources call it, or empty for one written
	// into the content stream.
	Name string
	// Width and Height are the picture's own, in samples.
	Width, Height int
	// X and Y are where its bottom left corner lands on the page, and
	// DrawnWidth and DrawnHeight how large it is drawn, in points. A picture
	// turned or skewed by the page still reports the box it covers.
	X, Y                    float64
	DrawnWidth, DrawnHeight float64
	// Filter is the last filter still on the data: DCTDecode for a JPEG that
	// comes out as one, JPXDecode for a JPEG 2000, and empty for data this
	// has unfiltered into plain samples.
	Filter reader.Name
	// Data is the picture's bytes, with every filter this understands undone.
	Data []byte
	// Dict is the image's own dictionary, which says how to read the data:
	// its colour space, how many bits a sample takes, and whether it is a
	// mask.
	Dict reader.Dict
	// Inline reports a picture written into the content stream rather than
	// held as a resource of its own.
	Inline bool
}

// Images is every picture the page places, in the order it places them. The
// same picture placed twice comes back twice, since where it lands is part of
// what is being asked for.
func Images(d *reader.Document, page int) ([]Image, error) {
	e, err := walk(d, page)
	if err != nil {
		return nil, err
	}
	return e.images, nil
}

// doXObject handles the Do operator: a form is walked into, and a picture is
// noted where it lands.
func (e *extractor) doXObject(g *state, operands []reader.Object, resources reader.Dict, depth int) {
	if len(operands) == 0 {
		return
	}
	name, ok := reader.ToName(operands[0])
	if !ok {
		return
	}
	xobjects, ok := e.doc.GetDict(resources, "XObject")
	if !ok {
		return
	}
	stream, ok := reader.ToStream(resolve(e.doc, xobjects.Get(name)))
	if !ok {
		return
	}
	switch sub, _ := reader.ToName(resolve(e.doc, stream.Dict.Get("Subtype"))); sub {
	case "Form":
		e.doForm(g, stream, resources, depth)
	case "Image":
		e.noteImage(g, string(name), stream.Dict, stream.Raw, false)
	}
}

// doForm walks into a form, which is a piece of a page held apart and drawn
// where the page says.
func (e *extractor) doForm(g *state, stream *reader.Stream, resources reader.Dict, depth int) {
	data, img, err := e.doc.DecodeStream(stream)
	if err != nil || img != "" {
		return
	}
	inner := *g
	if m := floatArray(e.doc, stream.Dict.Get("Matrix")); len(m) >= 6 {
		inner.ctm = matrix{m[0], m[1], m[2], m[3], m[4], m[5]}.mul(g.ctm)
	}
	own, ok := e.doc.GetDict(stream.Dict, "Resources")
	if !ok {
		own = resources
	}
	e.run(data, own, inner, depth+1)
}

// inlineImage notes a picture written into the content stream.
func (e *extractor) inlineImage(g *state, im *reader.InlineImage) {
	if im == nil {
		return
	}
	e.noteImage(g, "", im.Expanded(), im.Raw, true)
}

// noteImage works out where a picture lands and records it. A picture occupies
// the unit square of the transform in force, so its corners are where that
// square's are.
func (e *extractor) noteImage(g *state, name string, dict reader.Dict, raw []byte, inline bool) {
	minX, minY, maxX, maxY := unitSquareBox(g.ctm)
	img := Image{
		Name:        name,
		Width:       int(intOf(resolve(e.doc, dict.Get("Width")), 0)),
		Height:      int(intOf(resolve(e.doc, dict.Get("Height")), 0)),
		X:           minX,
		Y:           minY,
		DrawnWidth:  maxX - minX,
		DrawnHeight: maxY - minY,
		Dict:        dict,
		Inline:      inline,
	}
	data, filter, err := reader.Decode(dict, raw, e.doc.Resolver())
	if err == nil {
		img.Data, img.Filter = data, filter
	} else {
		img.Data = raw
	}
	e.images = append(e.images, img)
}

// unitSquareBox is the box the unit square covers once the transform has had
// its way with it.
func unitSquareBox(m matrix) (minX, minY, maxX, maxY float64) {
	first := true
	for _, c := range [][2]float64{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
		x, y := m.apply(c[0], c[1])
		if first {
			minX, minY, maxX, maxY = x, y, x, y
			first = false
			continue
		}
		minX, maxX = smaller(minX, x), larger(maxX, x)
		minY, maxY = smaller(minY, y), larger(maxY, y)
	}
	return minX, minY, maxX, maxY
}

func smaller(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func larger(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// intOf reads a whole number, or gives a default.
func intOf(o reader.Object, def int64) int64 {
	if v, ok := reader.ToInt(o); ok {
		return v
	}
	return def
}

// floatArray reads an array of numbers, or nothing.
func floatArray(d *reader.Document, o reader.Object) []float64 {
	arr, ok := reader.ToArray(resolve(d, o))
	if !ok {
		return nil
	}
	out := make([]float64, 0, len(arr))
	for _, e := range arr {
		v, ok := reader.ToFloat(resolve(d, e))
		if !ok {
			return nil
		}
		out = append(out, v)
	}
	return out
}
