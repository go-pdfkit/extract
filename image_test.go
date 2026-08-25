package extract

import (
	"testing"

	"github.com/go-pdfkit/reader"
)

// greyImage is a two-by-two picture of plain grey samples.
func greyImage(w *reader.Writer) reader.Object {
	return w.Add(&reader.Stream{Dict: reader.Dict{
		"Type": reader.Name("XObject"), "Subtype": reader.Name("Image"),
		"Width": reader.Integer(2), "Height": reader.Integer(2),
		"ColorSpace": reader.Name("DeviceGray"), "BitsPerComponent": reader.Integer(8),
	}, Raw: []byte{0, 64, 128, 255}})
}

func TestWhereAPicturePutsItself(t *testing.T) {
	d := pageWith(t, "q 50 0 0 40 20 30 cm /Im1 Do Q", func(w *reader.Writer, res reader.Dict) {
		res["XObject"] = reader.Dict{"Im1": greyImage(w)}
	})
	imgs, err := Images(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 1 {
		t.Fatalf("%d pictures", len(imgs))
	}
	im := imgs[0]
	if im.Name != "Im1" || im.Width != 2 || im.Height != 2 {
		t.Errorf("the picture is %+v", im)
	}
	if im.X != 20 || im.Y != 30 || im.DrawnWidth != 50 || im.DrawnHeight != 40 {
		t.Errorf("it lands at (%v,%v) %vx%v", im.X, im.Y, im.DrawnWidth, im.DrawnHeight)
	}
	if len(im.Data) != 4 || im.Filter != "" {
		t.Errorf("its data is %d bytes, filter %q", len(im.Data), im.Filter)
	}
	if im.Inline {
		t.Error("a picture held as a resource says it is inline")
	}
	if im.Dict == nil {
		t.Error("its dictionary is not there")
	}
}

func TestAPictureTurnedOnItsSide(t *testing.T) {
	// A picture the page turns still reports the box it covers.
	d := pageWith(t, "q 0 40 -50 0 100 30 cm /Im1 Do Q", func(w *reader.Writer, res reader.Dict) {
		res["XObject"] = reader.Dict{"Im1": greyImage(w)}
	})
	imgs, err := Images(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 1 {
		t.Fatalf("%d pictures", len(imgs))
	}
	if imgs[0].DrawnWidth != 50 || imgs[0].DrawnHeight != 40 {
		t.Errorf("a turned picture covers %vx%v", imgs[0].DrawnWidth, imgs[0].DrawnHeight)
	}
}

func TestAPictureThatCameWithTheContent(t *testing.T) {
	d := pageWith(t, "q 10 0 0 10 5 5 cm BI /W 2 /H 2 /CS /G /BPC 8 ID \x00\x40\x80\xff EI Q", nil)
	imgs, err := Images(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 1 {
		t.Fatalf("%d pictures", len(imgs))
	}
	if !imgs[0].Inline || imgs[0].Width != 2 {
		t.Errorf("the picture is %+v", imgs[0])
	}
}

func TestAPictureThatStaysAJPEG(t *testing.T) {
	// A JPEG comes back as one: re-encoding it would lose something for
	// nothing.
	d := pageWith(t, "q 10 0 0 10 0 0 cm /Im1 Do Q", func(w *reader.Writer, res reader.Dict) {
		res["XObject"] = reader.Dict{"Im1": w.Add(&reader.Stream{Dict: reader.Dict{
			"Subtype": reader.Name("Image"), "Width": reader.Integer(1),
			"Height": reader.Integer(1), "Filter": reader.Name("DCTDecode"),
		}, Raw: []byte("pretend this is a jpeg")})}
	})
	imgs, err := Images(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 1 || imgs[0].Filter != "DCTDecode" {
		t.Fatalf("the picture is %+v", imgs)
	}
	if string(imgs[0].Data) != "pretend this is a jpeg" {
		t.Errorf("its data was changed: %q", imgs[0].Data)
	}
}

func TestAPictureInsideAForm(t *testing.T) {
	// A form is a piece of a page held apart; what it draws lands where the
	// page puts the form.
	d := pageWith(t, "q 2 0 0 2 10 10 cm /Fm Do Q", func(w *reader.Writer, res reader.Dict) {
		inner := reader.Dict{"XObject": reader.Dict{"Im1": greyImage(w)}}
		res["XObject"] = reader.Dict{"Fm": w.Add(&reader.Stream{Dict: reader.Dict{
			"Subtype": reader.Name("Form"),
			"Matrix": reader.Array{reader.Integer(1), reader.Integer(0), reader.Integer(0),
				reader.Integer(1), reader.Integer(5), reader.Integer(0)},
			"Resources": inner,
		}, Raw: []byte("q 10 0 0 10 0 0 cm /Im1 Do Q")})}
	})
	imgs, err := Images(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 1 {
		t.Fatalf("%d pictures", len(imgs))
	}
	// The form is placed at (10,10) doubled, and moves the picture five of
	// its own units across: ten on the page.
	if imgs[0].X != 20 || imgs[0].Y != 10 {
		t.Errorf("the picture lands at (%v,%v)", imgs[0].X, imgs[0].Y)
	}
	if imgs[0].DrawnWidth != 20 {
		t.Errorf("it is %v wide", imgs[0].DrawnWidth)
	}
}

func TestTextInsideAForm(t *testing.T) {
	d := pageWith(t, "/Fm Do", func(w *reader.Writer, res reader.Dict) {
		fontDict := res["Font"].(reader.Dict)
		res["XObject"] = reader.Dict{"Fm": w.Add(&reader.Stream{Dict: reader.Dict{
			"Subtype": reader.Name("Form"), "Resources": reader.Dict{"Font": fontDict},
		}, Raw: []byte("BT /F1 10 Tf 20 100 Td (inside) Tj ET")})}
	})
	if text, _ := Text(d, 1); text != "inside" {
		t.Errorf("the form says %q", text)
	}
	// A form with no resources of its own uses the page's.
	d = pageWith(t, "/Fm Do", func(w *reader.Writer, res reader.Dict) {
		res["XObject"] = reader.Dict{"Fm": w.Add(&reader.Stream{Dict: reader.Dict{
			"Subtype": reader.Name("Form"),
		}, Raw: []byte("BT /F1 10 Tf 20 100 Td (borrowed) Tj ET")})}
	})
	if text, _ := Text(d, 1); text != "borrowed" {
		t.Errorf("a form with no resources says %q", text)
	}
}

func TestAFormThatDrawsItself(t *testing.T) {
	// A form naming itself would go round for ever; the depth it is drawn at
	// is what stops it.
	d := pageWith(t, "/Fm Do", func(w *reader.Writer, res reader.Dict) {
		ref := w.Reserve()
		res["XObject"] = reader.Dict{"Fm": ref}
		w.Put(ref, &reader.Stream{Dict: reader.Dict{
			"Subtype":   reader.Name("Form"),
			"Resources": reader.Dict{"XObject": reader.Dict{"Fm": ref}},
		}, Raw: []byte("/Fm Do")})
	})
	if _, err := Images(d, 1); err != nil {
		t.Fatal(err)
	}
}

func TestThingsNamedWithDoThatAreNotThere(t *testing.T) {
	contents := []string{
		"/Im1 Do",     // no XObject resources at all
		"Do",          // no name
		"7 Do",        // a name that is not one
		"/Missing Do", // a name nothing answers to
	}
	for _, content := range contents {
		d := pageWith(t, content, func(w *reader.Writer, res reader.Dict) {
			res["XObject"] = reader.Dict{"Im1": reader.Integer(3)}
		})
		if _, err := Images(d, 1); err != nil {
			t.Errorf("%q: %v", content, err)
		}
	}
	// One of a kind nobody has heard of, and a form filtered as an image.
	d := pageWith(t, "/A Do /B Do", func(w *reader.Writer, res reader.Dict) {
		res["XObject"] = reader.Dict{
			"A": w.Add(&reader.Stream{Dict: reader.Dict{"Subtype": reader.Name("Nonsense")}, Raw: nil}),
			"B": w.Add(&reader.Stream{Dict: reader.Dict{
				"Subtype": reader.Name("Form"), "Filter": reader.Name("DCTDecode")}, Raw: []byte("no")}),
		}
	})
	imgs, err := Images(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 0 {
		t.Errorf("%d pictures came back", len(imgs))
	}
}

func TestAPictureWhoseDataCannotBeUnfiltered(t *testing.T) {
	d := pageWith(t, "q 10 0 0 10 0 0 cm /Im1 Do Q", func(w *reader.Writer, res reader.Dict) {
		res["XObject"] = reader.Dict{"Im1": w.Add(&reader.Stream{Dict: reader.Dict{
			"Subtype": reader.Name("Image"), "Width": reader.Integer(1),
			"Height": reader.Integer(1), "Filter": reader.Name("FlateDecode"),
		}, Raw: []byte("not deflated at all")})}
	})
	imgs, err := Images(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 1 {
		t.Fatalf("%d pictures", len(imgs))
	}
	if string(imgs[0].Data) != "not deflated at all" {
		t.Errorf("data that could not be unfiltered came back as %q", imgs[0].Data)
	}
}

func TestAFormWithAMatrixThatIsNotOne(t *testing.T) {
	d := pageWith(t, "/Fm Do", func(w *reader.Writer, res reader.Dict) {
		res["XObject"] = reader.Dict{"Fm": w.Add(&reader.Stream{Dict: reader.Dict{
			"Subtype": reader.Name("Form"), "Matrix": reader.Array{reader.Integer(1)},
			"Resources": reader.Dict{"XObject": reader.Dict{"Im1": greyImage(w)}},
		}, Raw: []byte("q 10 0 0 10 0 0 cm /Im1 Do Q")})}
	})
	imgs, err := Images(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 1 || imgs[0].X != 0 {
		t.Errorf("the picture is %+v", imgs)
	}
}

func TestAPictureWrittenIntoTheContentThatIsNotOne(t *testing.T) {
	// A BI that never says ID has no picture in it.
	d := pageWith(t, "BI /W 2 /H 2 EI", nil)
	imgs, err := Images(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 0 {
		t.Errorf("%d pictures came out of nothing", len(imgs))
	}
}

func TestAPictureThatSaysNothingAboutItsSize(t *testing.T) {
	d := pageWith(t, "q 10 0 0 10 0 0 cm /Im1 Do Q", func(w *reader.Writer, res reader.Dict) {
		res["XObject"] = reader.Dict{"Im1": w.Add(&reader.Stream{Dict: reader.Dict{
			"Subtype": reader.Name("Image")}, Raw: []byte{1}})}
	})
	imgs, err := Images(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 1 || imgs[0].Width != 0 || imgs[0].Height != 0 {
		t.Errorf("the picture is %+v", imgs)
	}
}

func TestAFormWhoseMatrixIsNotNumbers(t *testing.T) {
	d := pageWith(t, "/Fm Do", func(w *reader.Writer, res reader.Dict) {
		res["XObject"] = reader.Dict{"Fm": w.Add(&reader.Stream{Dict: reader.Dict{
			"Subtype": reader.Name("Form"),
			"Matrix": reader.Array{reader.Name("a"), reader.Integer(0), reader.Integer(0),
				reader.Integer(1), reader.Integer(0), reader.Integer(0)},
			"Resources": reader.Dict{"XObject": reader.Dict{"Im1": greyImage(w)}},
		}, Raw: []byte("q 10 0 0 10 0 0 cm /Im1 Do Q")})}
	})
	imgs, err := Images(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 1 {
		t.Errorf("%d pictures", len(imgs))
	}
}

func TestAnInlineImageThatIsNotThere(t *testing.T) {
	// A BI the content scanner could make nothing of leaves no picture, and
	// nothing to trip over either.
	e := &extractor{}
	e.inlineImage(&state{}, nil)
	if len(e.images) != 0 {
		t.Errorf("%d pictures came from nothing", len(e.images))
	}
}
