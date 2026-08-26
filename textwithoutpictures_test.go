package extract

import (
	"bytes"
	"compress/zlib"
	"runtime"
	"testing"

	"github.com/go-pdfkit/reader"
)

// bigImage is a picture whose decompressed form is eight megabytes: a
// thousand by two thousand grey samples, deflated from a run of zeros so the
// stream itself is a few kilobytes. That gap between the stream and what it
// unpacks to is the whole of the measurement below.
func bigImage(w *reader.Writer) reader.Object {
	const width, height = 1000, 2000
	var buf bytes.Buffer
	z := zlib.NewWriter(&buf)
	_, _ = z.Write(make([]byte, width*height*4))
	_ = z.Close()
	return w.Add(&reader.Stream{Dict: reader.Dict{
		"Type": reader.Name("XObject"), "Subtype": reader.Name("Image"),
		"Width": reader.Integer(width), "Height": reader.Integer(height * 4),
		"ColorSpace": reader.Name("DeviceGray"), "BitsPerComponent": reader.Integer(8),
		"Filter": reader.Name("FlateDecode"),
	}, Raw: buf.Bytes()})
}

// allocatedBy is how many bytes one call put on the heap.
func allocatedBy(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// TestTextDoesNotUnpackThePictures is the regression.
//
// Runs, Text and Images all walk the page the same way, and the walk used to
// undo every picture's filters whatever the caller had asked for. A page
// holding one arXiv figure carries 378 MB of image once decompressed, so
// asking that page what it *said* cost 1 094 MB and 1.33 seconds spent
// inflating pictures the answer throws away.
//
// The assertion is on memory rather than time because memory is the thing
// that does not depend on how busy the machine is. The margin is wide: the
// picture is eight megabytes decompressed and the text is a few dozen bytes,
// so anything under a megabyte means the filters were left alone and anything
// over eight means they were not.
func TestTextDoesNotUnpackThePictures(t *testing.T) {
	d := pageWith(t, "q 50 0 0 40 20 30 cm /Im1 Do Q BT /F1 12 Tf 10 700 Td (hello) Tj ET",
		func(w *reader.Writer, res reader.Dict) {
			res["XObject"] = reader.Dict{"Im1": bigImage(w)}
		})

	var runs []Run
	forText := allocatedBy(func() {
		var err error
		runs, err = Runs(d, 1)
		if err != nil {
			t.Fatal(err)
		}
	})
	if len(runs) == 0 {
		t.Fatal("the page says nothing, so the measurement means nothing")
	}
	if forText > 1<<20 {
		t.Errorf("reading the text allocated %d bytes: the pictures were unpacked for an answer that does not hold them", forText)
	}

	var images []Image
	forImages := allocatedBy(func() {
		var err error
		images, err = Images(d, 1)
		if err != nil {
			t.Fatal(err)
		}
	})
	if len(images) != 1 {
		t.Fatalf("%d pictures", len(images))
	}
	// The other half: asking for the pictures must still unpack them.
	if n := len(images[0].Data); n != 8_000_000 {
		t.Fatalf("the picture came back as %d bytes, want 8000000 — asking for the pictures must still unpack them", n)
	}
	if forImages < 8<<20 {
		t.Errorf("reading the pictures allocated only %d bytes; the measurement is not measuring what it thinks", forImages)
	}
}

// TestTextStillKnowsWhereThePicturesAre checks what was deliberately kept:
// where a picture lands costs a matrix multiply, so it is still worked out
// even when nobody asked for the pixels. Only the bytes are left alone.
func TestTextStillKnowsWhereThePicturesAre(t *testing.T) {
	d := pageWith(t, "q 50 0 0 40 20 30 cm /Im1 Do Q", func(w *reader.Writer, res reader.Dict) {
		res["XObject"] = reader.Dict{"Im1": greyImage(w)}
	})
	e, err := walk(d, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.images) != 1 {
		t.Fatalf("%d pictures noted", len(e.images))
	}
	im := e.images[0]
	if im.Name != "Im1" || im.Width != 2 || im.Height != 2 {
		t.Errorf("the picture is %+v", im)
	}
	if im.X != 20 || im.Y != 30 || im.DrawnWidth != 50 || im.DrawnHeight != 40 {
		t.Errorf("it lands at (%v,%v) %vx%v", im.X, im.Y, im.DrawnWidth, im.DrawnHeight)
	}
	if im.Data != nil {
		t.Errorf("its bytes were unpacked anyway: %d of them", len(im.Data))
	}
}
