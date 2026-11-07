package extract

import (
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

// ⛔ A SPAN SAYS WHICH BYTES DREW THE RUN. Everything else here follows from
// that one claim, so it is checked against the stream's own text rather than
// against a remembered offset: the test reads content[From:To] back and
// compares it with the operator it expects to find there.
func TestARunSaysWhichOperatorDrewIt(t *testing.T) {
	const content = "BT /F1 10 Tf 20 100 Td (one) Tj 0 -20 Td (two) Tj ET"
	runs, err := Runs(pageWith(t, content, nil), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("the page drew %d run(s), want 2", len(runs))
	}
	for i, want := range []string{"(one) Tj", "(two) Tj"} {
		r := runs[i]
		if r.Op.Empty() {
			t.Errorf("run %d (%q) carries no span", i, r.Text)
			continue
		}
		if r.Op.From < 0 || r.Op.To > len(content) {
			t.Fatalf("run %d spans %d..%d, outside a stream of %d bytes",
				i, r.Op.From, r.Op.To, len(content))
		}
		if got := content[r.Op.From:r.Op.To]; got != want {
			t.Errorf("run %d (%q) spans %q, want %q", i, r.Text, got, want)
		}
		if r.InForm {
			t.Errorf("run %d came from the page itself and says it is in a form", i)
		}
	}
}

// ⛔⛔ THE RUNS OF ONE TJ SHARE ITS SPAN, and a caller has to be told so. One
// operator shows several strings, so removing a span removes ALL of them —
// anybody wanting to be rid of one piece must rewrite the operator. A caller
// that deleted the span would silently take away text it was not asked about.
func TestTheRunsOfOneTJShareItsSpan(t *testing.T) {
	const content = "BT /F1 10 Tf 20 100 Td [(one) -500 (two)] TJ ET"
	runs, err := Runs(pageWith(t, content, nil), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("the array drew %d run(s), want 2", len(runs))
	}
	if runs[0].Op != runs[1].Op {
		t.Errorf("two pieces of one TJ span %+v and %+v", runs[0].Op, runs[1].Op)
	}
	if got, want := content[runs[0].Op.From:runs[0].Op.To], "[(one) -500 (two)] TJ"; got != want {
		t.Errorf("the span reads %q, want %q", got, want)
	}
	// And they really are two pieces, drawn apart: the kern moved the pen.
	if runs[0].X == runs[1].X {
		t.Errorf("both pieces start at x=%g, so this fixture proves nothing", runs[0].X)
	}
}

// ⛔⛔ A RUN FROM A FORM MUST SAY SO, because its span indexes the FORM's
// stream and not the page's. A caller editing the page at those offsets would
// cut somewhere else entirely — and a Form XObject is shared, so editing it to
// change one page changes every page that draws it.
func TestARunInsideAFormSaysSo(t *testing.T) {
	const content = "BT /F1 10 Tf 20 150 Td (page) Tj ET q 1 0 0 1 0 0 cm /X1 Do Q"
	const inner = "BT /F1 10 Tf 20 50 Td (inside the form) Tj ET"
	d := pageWith(t, content, func(w *reader.Writer, res reader.Dict) {
		form := w.Add(&reader.Stream{Dict: reader.Dict{
			"Type": reader.Name("XObject"), "Subtype": reader.Name("Form"),
			"BBox": reader.Array{reader.Integer(0), reader.Integer(0),
				reader.Integer(200), reader.Integer(200)},
			"Resources": reader.Dict{"Font": reader.Dict{"F1": w.Add(simpleFont(w))}},
		}, Raw: []byte(inner)})
		res["XObject"] = reader.Dict{"X1": form}
	})
	runs, err := Runs(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	var page, form *Run
	for i := range runs {
		switch {
		case strings.Contains(runs[i].Text, "page"):
			page = &runs[i]
		case strings.Contains(runs[i].Text, "form"):
			form = &runs[i]
		}
	}
	if page == nil || form == nil {
		t.Fatalf("the fixture gave %d run(s) and not the two expected", len(runs))
	}
	if page.InForm {
		t.Error("the page's own text says it is inside a form")
	}
	if !form.InForm {
		t.Error("text drawn by a form does not say so, so a caller would cut the page at the form's offsets")
	}
	// The form's span is into the form's stream: it reads as the form's
	// operator, which is what makes it useless for editing the page.
	if form.Op.Empty() || form.Op.To > len(inner) {
		t.Fatalf("the form's run spans %+v, outside its own %d-byte stream", form.Op, len(inner))
	}
	if got, want := inner[form.Op.From:form.Op.To], "(inside the form) Tj"; got != want {
		t.Errorf("the form's span reads %q of its own stream, want %q", got, want)
	}
}

// ⛔⛔ THE POINT OF THE WHOLE FEATURE: a span is enough to take a run OUT.
// Without this the span is a number nobody has used, and the first caller to
// try would be the test. Cutting the second operator's bytes leaves a stream
// that still draws the first run, in the same place, and no longer draws the
// second — which is what a redaction needs and the reason Op exists.
func TestASpanIsEnoughToRemoveARun(t *testing.T) {
	const content = "BT /F1 10 Tf 20 100 Td (keep me) Tj 0 -20 Td (remove me) Tj ET"
	runs, err := Runs(pageWith(t, content, nil), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("the page drew %d run(s), want 2", len(runs))
	}
	keep, drop := runs[0], runs[1]
	cut := content[:drop.Op.From] + content[drop.Op.To:]
	if strings.Contains(cut, "remove me") {
		t.Fatalf("the cut stream still holds the string: %q", cut)
	}

	after, err := Runs(pageWith(t, cut, nil), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 {
		t.Fatalf("the cut page draws %d run(s), want 1", len(after))
	}
	if after[0].Text != keep.Text {
		t.Errorf("what is left says %q, want %q", after[0].Text, keep.Text)
	}
	// ⛔ AND THE SURVIVOR DID NOT MOVE. Removing one operator must not shift
	// the text that stays: that is the difference between a redaction and a
	// re-typesetting, and it is why the span covers the operands too.
	if after[0].X != keep.X || after[0].Y != keep.Y {
		t.Errorf("the surviving run moved from (%g,%g) to (%g,%g)",
			keep.X, keep.Y, after[0].X, after[0].Y)
	}
	if after[0].Width != keep.Width {
		t.Errorf("the surviving run is %g wide, was %g", after[0].Width, keep.Width)
	}
}
