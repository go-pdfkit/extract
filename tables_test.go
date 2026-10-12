package extract

import (
	"reflect"
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

// The test font gives every character half an em, so at ten points a letter is
// five points wide and a space is five points too. The default boundary is one
// and a half spaces, so 7.5 points: the fixtures below put columns forty
// points apart and words five.

func TestATableIsTheGapEveryRowShares(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (Name) Tj 40 0 Td (Age) Tj ET",
		"BT /F1 10 Tf 20 130 Td (Ada) Tj 40 0 Td (36) Tj ET",
	}, "\n"), nil)
	tables, err := Tables(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 {
		t.Fatalf("%d tables, want 1: %+v", len(tables), tables)
	}
	want := [][]string{{"Name", "Age"}, {"Ada", "36"}}
	if !reflect.DeepEqual(tables[0].Rows, want) {
		t.Errorf("the table reads %q, want %q", tables[0].Rows, want)
	}
	if tables[0].Page != 1 {
		t.Errorf("the table says it is on page %d", tables[0].Page)
	}
}

// ⛔ THE CONTROL. Prose is text at places too, and a detector that cannot say
// no to a paragraph says yes to every page in the corpus. The word gaps here
// are one space wide, and one space is a word break rather than a column.
func TestAParagraphIsNotATable(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (one) Tj 20 0 Td (two) Tj 20 0 Td (three) Tj ET",
		"BT /F1 10 Tf 20 130 Td (four) Tj 25 0 Td (five) Tj 25 0 Td (six) Tj ET",
	}, "\n"), nil)
	tables, err := Tables(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 0 {
		t.Errorf("a paragraph came back as %d table(s): %+v", len(tables), tables)
	}
}

func TestAPageWithNothingWrittenOnItHasNoTables(t *testing.T) {
	d := pageWith(t, "", nil)
	tables, err := Tables(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if tables != nil {
		t.Errorf("an empty page came back as %+v", tables)
	}
}

func TestATableOnAPageThatIsNotThere(t *testing.T) {
	d := pageWith(t, "BT /F1 10 Tf 20 150 Td (x) Tj ET", nil)
	if _, err := Tables(d, 9); err == nil {
		t.Error("page nine of a one-page document was read")
	}
}

// A line with nothing but spaces on it has no gaps of its own, so taking it
// seriously would cut the table in two at the whitespace its author drew.
func TestALineOfSpacesDoesNotCutATableInHalf(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (Name) Tj 40 0 Td (Age) Tj ET",
		"BT /F1 10 Tf 20 140 Td (  ) Tj ET",
		"BT /F1 10 Tf 20 130 Td (Ada) Tj 40 0 Td (36) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 1 {
		t.Fatalf("%d tables, want 1: %+v", len(tables), tables)
	}
	want := [][]string{{"Name", "Age"}, {"Ada", "36"}}
	if !reflect.DeepEqual(tables[0].Rows, want) {
		t.Errorf("the table reads %q, want %q", tables[0].Rows, want)
	}
}

// A line that shares no gap closes the block it is in and starts the next one,
// which is how a page holding two tables with a sentence between them gives
// two tables and not one.
func TestTwoTablesWithASentenceBetweenThem(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 180 Td (Name) Tj 40 0 Td (Age) Tj ET",
		"BT /F1 10 Tf 20 165 Td (Ada) Tj 40 0 Td (36) Tj ET",
		"BT /F1 10 Tf 20 150 Td (and then the second one) Tj ET",
		"BT /F1 10 Tf 20 135 Td (City) Tj 40 0 Td (Code) Tj ET",
		"BT /F1 10 Tf 20 120 Td (Caen) Tj 40 0 Td (14000) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 2 {
		t.Fatalf("%d tables, want 2: %+v", len(tables), tables)
	}
	if got := tables[0].Rows[0][0]; got != "Name" {
		t.Errorf("the first table starts with %q", got)
	}
	if got := tables[1].Rows[0][0]; got != "City" {
		t.Errorf("the second table starts with %q", got)
	}
}

// ⛔ The known failure, written down rather than hidden: a cell wide enough to
// reach across a boundary takes that boundary away from EVERY row, because a
// boundary is what all of them share. Two columns become one, and one column
// is not a table.
func TestACellThatReachesAcrossABoundaryTakesItAway(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (Name) Tj 40 0 Td (Age) Tj ET",
		"BT /F1 10 Tf 20 130 Td (a very long note indeed) Tj ET",
		"BT /F1 10 Tf 20 110 Td (Ada) Tj 40 0 Td (36) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 0 {
		t.Errorf("want none, got %d: %+v", len(tables), tables)
	}
}

// A row that says nothing in the middle column says it with an empty string,
// so that every row of a table has the same number of cells.
func TestACellTheRowLeftEmpty(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (Name) Tj 40 0 Td (Age) Tj 40 0 Td (City) Tj ET",
		"BT /F1 10 Tf 20 130 Td (Ada) Tj 80 0 Td (Caen) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 1 {
		t.Fatalf("%d tables, want 1: %+v", len(tables), tables)
	}
	want := [][]string{{"Name", "Age", "City"}, {"Ada", "", "Caen"}}
	if !reflect.DeepEqual(tables[0].Rows, want) {
		t.Errorf("the table reads %q, want %q", tables[0].Rows, want)
	}
}

// Several pieces of text in one cell are joined the way a line is, which is
// what puts the space back between two words a generator drew separately.
func TestTwoPiecesOfTextInOneCell(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (Ada) Tj 20 0 Td (King) Tj 40 0 Td (36) Tj ET",
		"BT /F1 10 Tf 20 130 Td (Bob) Tj 60 0 Td (41) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 1 {
		t.Fatalf("%d tables, want 1: %+v", len(tables), tables)
	}
	if got := tables[0].Rows[0][0]; got != "Ada King" {
		t.Errorf("the first cell reads %q, want %q", got, "Ada King")
	}
}

func TestTheBoxIsWhereTheWordsAre(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (Name) Tj 40 0 Td (Age) Tj ET",
		"BT /F1 10 Tf 20 130 Td (Ada) Tj 40 0 Td (36) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 1 {
		t.Fatalf("%d tables: %+v", len(tables), tables)
	}
	// Leftmost word at 20, rightmost ends at 60+15; lowest baseline 130, and
	// the top line is ten points tall above 150.
	got := tables[0]
	if got.X != 20 || got.Y != 130 || got.Width != 55 || got.Height != 30 {
		t.Errorf("the box is (%v,%v) %v by %v, want (20,130) 55 by 30",
			got.X, got.Y, got.Width, got.Height)
	}
}

// ⛔ Runs are ordered by where they START, so the right-hand edge of what has
// been drawn so far is a running maximum. Read it off the previous run instead
// and a wide cell with a short one inside it opens a gap that is not there.
func TestAShortRunInsideAWideOne(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		// Ten letters from 20 reach 70; the (x) at 30 sits inside them.
		"BT /F1 10 Tf 20 150 Td (AAAAAAAAAA) Tj ET",
		"BT /F1 10 Tf 30 150 Td (x) Tj ET",
		"BT /F1 10 Tf 100 150 Td (right) Tj ET",
		// A row whose own gaps are [35,50] and [65,100].
		"BT /F1 10 Tf 20 130 Td (Ada) Tj 30 0 Td (bbb) Tj 50 0 Td (ccc) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 1 {
		t.Fatalf("%d tables, want 1: %+v", len(tables), tables)
	}
	// The only boundary both rows share is [70,100]: two columns, not three.
	if n := len(tables[0].Rows[0]); n != 2 {
		t.Errorf("%d columns, want 2: %q", n, tables[0].Rows)
	}
}

func TestTheSmallestGridWorthReporting(t *testing.T) {
	content := strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (Name) Tj 40 0 Td (Age) Tj ET",
		"BT /F1 10 Tf 20 130 Td (Ada) Tj 40 0 Td (36) Tj ET",
	}, "\n")
	d := pageWith(t, content, nil)
	if tables, _ := TablesWithOptions(d, 1, TableOptions{MinRows: 3}); len(tables) != 0 {
		t.Errorf("two rows passed a three-row minimum: %+v", tables)
	}
	if tables, _ := TablesWithOptions(d, 1, TableOptions{MinColumns: 3}); len(tables) != 0 {
		t.Errorf("two columns passed a three-column minimum: %+v", tables)
	}
	if tables, _ := TablesWithOptions(d, 1, TableOptions{MinRows: 2, MinColumns: 2}); len(tables) != 1 {
		t.Errorf("the grid it is exactly big enough for was refused: %+v", tables)
	}
}

// ⛔ A bulleted list IS a two-column grid, and 588 of them in one 213-page
// document was what this detector first reported. What tells them apart is
// that one column says the same thing on every row — and a column that says
// the same thing on every row distinguishes nothing.
func TestABulletedListIsNotATable(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (\225) Tj 40 0 Td (conduct the project) Tj ET",
		"BT /F1 10 Tf 20 130 Td (\225) Tj 40 0 Td (inform the trustees) Tj ET",
		"BT /F1 10 Tf 20 110 Td (\225) Tj 40 0 Td (federate the staff) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 0 {
		t.Errorf("a bulleted list came back as %d table(s): %+v", len(tables), tables)
	}
}

// ⛔ A MARK, not the same mark twice. Insisting on one repeated mark read
// "• • - -" as a table of four rows, because the bullets were not all the
// same character — and a reader sees two lists there.
func TestTwoDifferentMarksAreStillMarks(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (\225) Tj 40 0 Td (conduct the project) Tj ET",
		"BT /F1 10 Tf 20 130 Td (\225) Tj 40 0 Td (inform the trustees) Tj ET",
		"BT /F1 10 Tf 20 110 Td (-) Tj 40 0 Td (federate the staff) Tj ET",
		"BT /F1 10 Tf 20 90 Td (-) Tj 40 0 Td (and animate it) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 0 {
		t.Errorf("two kinds of bullet came back as %d table(s): %+v", len(tables), tables)
	}
}

// ⛔ A letter or a digit is information, which is what keeps a real table. A
// numbering somebody may want is not an ornament, and a table of years is
// nothing but digits.
func TestAFirstColumnOfLabelsOrNumbersIsStillAColumn(t *testing.T) {
	for _, first := range []struct{ a, b string }{
		{"A", "B"},   // labels
		{"1.", "2."}, // a numbering
		{"90", "91"}, // years
		// Three characters is not short. A mark is one or two; anything
		// longer was drawn to be read.
		{"---", "---"},
	} {
		d := pageWith(t, strings.Join([]string{
			"BT /F1 10 Tf 20 150 Td (" + first.a + ") Tj 40 0 Td (one) Tj ET",
			"BT /F1 10 Tf 20 130 Td (" + first.b + ") Tj 40 0 Td (two) Tj ET",
		}, "\n"), nil)
		tables, _ := Tables(d, 1)
		if len(tables) != 1 {
			t.Fatalf("%q/%q: %d tables, want 1: %+v", first.a, first.b, len(tables), tables)
		}
		want := [][]string{{first.a, "one"}, {first.b, "two"}}
		if !reflect.DeepEqual(tables[0].Rows, want) {
			t.Errorf("%q/%q: the table reads %q, want %q", first.a, first.b, tables[0].Rows, want)
		}
	}
}

// The mark has to be SHORT. A first column that repeats a word is a column
// that repeats a word, and throwing it away would throw away the data.
func TestAFirstColumnThatRepeatsAWordIsStillAColumn(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (CNRS) Tj 40 0 Td (one) Tj ET",
		"BT /F1 10 Tf 20 130 Td (CNRS) Tj 40 0 Td (two) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 1 {
		t.Fatalf("%d tables, want 1: %+v", len(tables), tables)
	}
	want := [][]string{{"CNRS", "one"}, {"CNRS", "two"}}
	if !reflect.DeepEqual(tables[0].Rows, want) {
		t.Errorf("the table reads %q, want %q", tables[0].Rows, want)
	}
}

// ⛔ THE DEFECT THAT CHANGED THE ALGORITHM. A cell whose text wraps leaves a
// line with one piece of text on it and so with no gaps of its own. Asking
// what gaps two lines SHARE ends the table there, and a four-row table with
// one wrapped cell came back as two tables of two rows — measured on a real
// document, not on a fixture. Asking instead what either line DRAWS in the
// boundary keeps it: a wrapped line sits inside a column and draws nothing in
// the boundary beside it.
func TestAWrappedCellDoesNotSplitATableInTwo(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 180 Td (Activity) Tj 90 0 Td (Outcome) Tj ET",
		"BT /F1 10 Tf 20 160 Td (design) Tj 90 0 Td (ten petabytes) Tj ET",
		// The wrapped remainder of the cell above: one run, no gaps of its
		// own, and nothing drawn in the boundary at 85.
		"BT /F1 10 Tf 110 145 Td (of flash) Tj ET",
		"BT /F1 10 Tf 20 125 Td (operate) Tj 90 0 Td (a mesocentre) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 1 {
		t.Fatalf("%d tables, want 1: %+v", len(tables), tables)
	}
	want := [][]string{
		{"Activity", "Outcome"},
		{"design", "ten petabytes"},
		{"", "of flash"},
		{"operate", "a mesocentre"},
	}
	if !reflect.DeepEqual(tables[0].Rows, want) {
		t.Errorf("the table reads %q, want %q", tables[0].Rows, want)
	}
}

// A boundary a run lands inside survives as the pieces either side of it, so
// that run gets a column of its own and the table goes on. Dropping the
// boundary instead would end the table at the first value anybody centred.
func TestARunInsideABoundaryMakesAColumnOfItsOwn(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		// A boundary from 60 to 140.
		"BT /F1 10 Tf 20 150 Td (Name) Tj 120 0 Td (Age) Tj ET",
		// A run from 70 to 100 leaves 60 to 70 and 100 to 140 of it.
		"BT /F1 10 Tf 20 130 Td (Ada) Tj 50 0 Td (xxxxxx) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 1 {
		t.Fatalf("%d tables, want 1: %+v", len(tables), tables)
	}
	want := [][]string{{"Name", "", "Age"}, {"Ada", "xxxxxx", ""}}
	if !reflect.DeepEqual(tables[0].Rows, want) {
		t.Errorf("the table reads %q, want %q", tables[0].Rows, want)
	}
}

// ⛔ A page number sits in the left-hand column and draws nothing in any
// boundary, which is precisely what a row of a table does. What says it is not
// one is that it is a long way below the last row.
func TestAPageNumberIsNotTheLastRow(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (Name) Tj 40 0 Td (Age) Tj ET",
		"BT /F1 10 Tf 20 130 Td (Ada) Tj 40 0 Td (36) Tj ET",
		"BT /F1 10 Tf 20 40 Td (7) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 1 {
		t.Fatalf("%d tables, want 1: %+v", len(tables), tables)
	}
	if n := len(tables[0].Rows); n != 2 {
		t.Errorf("%d rows, want 2: %q", n, tables[0].Rows)
	}
}

// ⛔ A column nothing was drawn in is not a column — it is a stretch of page
// nothing was drawn in, which is what a boundary already is.
//
// This is how one turns up: a run of nothing but spaces sits in the middle of
// a boundary and splits it in two, and the column it makes between the halves
// reads empty because that is what it says.
func TestAColumnNothingWasDrawnInIsNotAColumn(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (Name) Tj 80 0 Td (Age) Tj ET",
		"BT /F1 10 Tf 20 130 Td (Ada) Tj 40 0 Td ( ) Tj 40 0 Td (36) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 1 {
		t.Fatalf("%d tables, want 1: %+v", len(tables), tables)
	}
	want := [][]string{{"Name", "Age"}, {"Ada", "36"}}
	if !reflect.DeepEqual(tables[0].Rows, want) {
		t.Errorf("the table reads %q, want %q", tables[0].Rows, want)
	}
}

// A bullet list whose text wraps: the lines it wraps onto carry no bullet, so
// the mark is in some rows of its column and not others. It is still a list.
func TestABulletedListWhoseTextWraps(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (\225) Tj 40 0 Td (conduct the project) Tj ET",
		"BT /F1 10 Tf 60 135 Td (and animate it) Tj ET",
		"BT /F1 10 Tf 20 120 Td (\225) Tj 40 0 Td (inform the trustees) Tj ET",
		"BT /F1 10 Tf 60 105 Td (of what changes) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 0 {
		t.Errorf("a wrapped bulleted list came back as %d table(s): %+v", len(tables), tables)
	}
}

// The row above is measured by its TALLEST piece of text, not its first: a row
// whose first cell is small and whose second is a heading is as far from the
// next row as that heading is tall.
func TestHowFarIsMeasuredByTheTallestPieceOfTheRow(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 6 Tf 20 150 Td (no) Tj ET BT /F1 20 Tf 60 150 Td (Heading) Tj ET",
		// Fifty points below: more than three of the small size, less than
		// three of the large one.
		"BT /F1 10 Tf 20 100 Td (Ada) Tj 40 0 Td (36) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 1 {
		t.Errorf("the heading's own height did not reach the row below: %+v", tables)
	}
}

// ⛔ A caller who asks for one-row tables is the only one who can see the
// difference between a gap and a boundary on a SINGLE line, because a block of
// two lines or more has a boundary by construction: it grew only for as long
// as the lines shared one. Both the width a gap needs and the two columns a
// table needs are invisible until somebody lowers MinRows.
func TestOneRowIsNotATableEvenWhenTheCallerAllowsOne(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (one) Tj 20 0 Td (two) Tj 20 0 Td (three) Tj ET",
		"BT /F1 10 Tf 20 130 Td (four) Tj 25 0 Td (five) Tj 25 0 Td (six) Tj ET",
	}, "\n"), nil)
	tables, _ := TablesWithOptions(d, 1, TableOptions{MinRows: 1})
	if len(tables) != 0 {
		t.Errorf("a line of prose came back as %d table(s): %+v", len(tables), tables)
	}
}

// Two rows whose gaps overlap by less than a boundary's width do not share a
// boundary: what is shared is the OVERLAP, and five points of it is a word
// break however wide each row's own gap was.
func TestTwoRowsWhoseGapsBarelyOverlap(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		// A gap from 30 to 45, and ...
		"BT /F1 10 Tf 20 150 Td (AA) Tj 25 0 Td (BB) Tj ET",
		// ... one from 40 to 55: five points in common.
		"BT /F1 10 Tf 20 130 Td (AAAA) Tj 35 0 Td (D) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 0 {
		t.Errorf("a five-point overlap was a column: %+v", tables)
	}
}

// The width of a space is the MIDDLE one on the page, which means a line in
// an unusual size cannot decide what a column boundary is for everything else.
// Both ends of that matter and they fail in opposite directions: the narrowest
// makes every word gap in the body text into a column, the widest makes every
// real column into a word gap.
func TestOneLineInAnUnusualSizeDoesNotDecideForThePage(t *testing.T) {
	fine := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (one) Tj 20 0 Td (two) Tj ET",
		"BT /F1 10 Tf 20 130 Td (six) Tj 20 0 Td (ten) Tj ET",
		"BT /F1 2 Tf 20 60 Td (tiny) Tj ET",
	}, "\n"), nil)
	if tables, _ := Tables(fine, 1); len(tables) != 0 {
		t.Errorf("two-point type made the word gaps above into columns: %+v", tables)
	}
	large := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (Name) Tj 40 0 Td (Age) Tj ET",
		"BT /F1 10 Tf 20 130 Td (Ada) Tj 40 0 Td (36) Tj ET",
		"BT /F1 40 Tf 20 60 Td (X) Tj ET",
	}, "\n"), nil)
	if tables, _ := Tables(large, 1); len(tables) != 1 {
		t.Errorf("forty-point type made a twenty-point column into a word gap: %+v", tables)
	}
}

// The boundary a sizeless page gets is not zero. Two runs that touch have a
// gap of nothing between them, and a boundary of nothing would make every one
// of them into a column.
func TestTwoSizelessRunsThatTouchAreNotTwoColumns(t *testing.T) {
	d := type3Page(t, 0, strings.Join([]string{
		"BT /F3 0 Tf 20 150 Td (a) Tj (b) Tj ET",
		"BT /F3 0 Tf 20 130 Td (a) Tj (b) Tj ET",
	}, "\n"))
	tables, _ := Tables(d, 1)
	if len(tables) != 0 {
		t.Errorf("runs that touch came back as %d table(s): %+v", len(tables), tables)
	}
}

// A block's boundaries narrow as it grows: they are what EVERY line in it
// shares, so the third row takes away what the first two agreed on.
func TestTheThirdRowTakesABoundaryAway(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (A) Tj 40 0 Td (B) Tj 40 0 Td (C) Tj ET",
		"BT /F1 10 Tf 20 130 Td (D) Tj 40 0 Td (E) Tj 40 0 Td (F) Tj ET",
		// Sixty points of G reach from 60 to 120, across the second boundary.
		"BT /F1 10 Tf 20 110 Td (G) Tj 40 0 Td (HHHHHHHHHHHH) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 1 {
		t.Fatalf("%d tables, want 1: %+v", len(tables), tables)
	}
	if n := len(tables[0].Rows[0]); n != 2 {
		t.Errorf("%d columns, want 2: %q", n, tables[0].Rows)
	}
}

// The line that closed a block is the first line of the next one. Skip it and
// the second of two tables that touch loses its header row — and with it the
// two rows that make it a table at all.
func TestTwoTablesWithNothingBetweenThem(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 180 Td (A) Tj 40 0 Td (B) Tj ET",
		"BT /F1 10 Tf 20 160 Td (E) Tj 40 0 Td (F) Tj ET",
		"BT /F1 10 Tf 20 140 Td (CCCCCCCCCC) Tj 80 0 Td (D) Tj ET",
		"BT /F1 10 Tf 20 120 Td (GGGGGGGGGG) Tj 80 0 Td (H) Tj ET",
	}, "\n"), nil)
	tables, _ := Tables(d, 1)
	if len(tables) != 2 {
		t.Fatalf("%d tables, want 2: %+v", len(tables), tables)
	}
	if got := tables[1].Rows[0][0]; got != "CCCCCCCCCC" {
		t.Errorf("the second table starts with %q", got)
	}
}

// The boundary is counted in spaces, not in points, so the same number works
// for a footnote and for a title.
func TestHowWideAGapHasToBe(t *testing.T) {
	content := strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (one) Tj 20 0 Td (two) Tj ET",
		"BT /F1 10 Tf 20 130 Td (six) Tj 20 0 Td (ten) Tj ET",
	}, "\n")
	d := pageWith(t, content, nil)
	// The gap is one space. At the default of one and a half, it is a word
	// break; asked for less than one, it is a column.
	if tables, _ := TablesWithOptions(d, 1, TableOptions{}); len(tables) != 0 {
		t.Errorf("a one-space gap was a column at the default: %+v", tables)
	}
	tables, _ := TablesWithOptions(d, 1, TableOptions{ColumnGapSpaces: 0.5})
	if len(tables) != 1 {
		t.Fatalf("%d tables at half a space: %+v", len(tables), tables)
	}
	want := [][]string{{"one", "two"}, {"six", "ten"}}
	if !reflect.DeepEqual(tables[0].Rows, want) {
		t.Errorf("the table reads %q, want %q", tables[0].Rows, want)
	}
}

// A font that gives no width to the space it does not draw: the gap has to be
// measured from the size instead, as [wantsSpace] does for the same reason.
func TestAFontThatSaysNothingAboutItsSpace(t *testing.T) {
	d := type3Page(t, 10, strings.Join([]string{
		"BT /F3 10 Tf 20 150 Td (a) Tj 10 0 Td (b) Tj ET",
		"BT /F3 10 Tf 20 130 Td (b) Tj 10 0 Td (a) Tj ET",
	}, "\n"))
	// A quarter of ten points is 2.5, so the boundary is 3.75 and the ten
	// point gap is well over it.
	tables, _ := Tables(d, 1)
	if len(tables) != 1 {
		t.Fatalf("%d tables, want 1: %+v", len(tables), tables)
	}
	if want := [][]string{{"a", "b"}, {"b", "a"}}; !reflect.DeepEqual(tables[0].Rows, want) {
		t.Errorf("the table reads %q, want %q", tables[0].Rows, want)
	}
}

// Text set at no size at all: nothing on the page has a width to compare a
// gap with. What matters is that the boundary is not zero, or every pair of
// touching runs would be two columns.
func TestTextSetAtNoSizeAtAll(t *testing.T) {
	// Three pieces a point apart and then nineteen: at a boundary of nothing
	// the first two are two columns, at a boundary of anything they are one.
	d := type3Page(t, 0, strings.Join([]string{
		"BT /F3 0 Tf 20 150 Td (a) Tj 1 0 Td (b) Tj 19 0 Td (c) Tj ET",
		"BT /F3 0 Tf 20 130 Td (a) Tj 1 0 Td (b) Tj 19 0 Td (c) Tj ET",
	}, "\n"))
	runs, err := Runs(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range runs {
		if r.Size != 0 || r.Space != 0 {
			t.Fatalf("the fixture is not sizeless: %+v", r)
		}
	}
	tables, _ := Tables(d, 1)
	if len(tables) != 1 {
		t.Fatalf("%d tables, want 1: %+v", len(tables), tables)
	}
	if want := [][]string{{"a b", "c"}, {"a b", "c"}}; !reflect.DeepEqual(tables[0].Rows, want) {
		t.Errorf("the table reads %q, want %q", tables[0].Rows, want)
	}
}

// type3Page builds a page set in a Type 3 font with no character procedures,
// which is a font that gives no width to anything it is asked to draw. size is
// only here to be named in the test that reads it back.
func type3Page(t *testing.T, size int, content string) *reader.Document {
	t.Helper()
	_ = size
	return pageWith(t, content, func(w *reader.Writer, res reader.Dict) {
		res["Font"].(reader.Dict)["F3"] = w.Add(reader.Dict{
			"Type": reader.Name("Font"), "Subtype": reader.Name("Type3"),
			"FontMatrix": reader.Array{reader.Real(0.001), reader.Integer(0), reader.Integer(0),
				reader.Real(0.001), reader.Integer(0), reader.Integer(0)},
			"Encoding": w.Add(reader.Dict{"Differences": reader.Array{
				reader.Integer(97), reader.Name("a"), reader.Name("b")}}),
			"CharProcs": reader.Dict{},
		})
	})
}

// ⛔ Lines and Join are exported so that the rule is the SAME rule. The
// control is that they agree with Assemble, which is what the rest of the
// fleet already reads a page with: if the two could disagree, a caller reading
// structure out of a page would be reading a different page from the one
// Text reports.
func TestLinesAndJoinAgreeWithAssemble(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		"BT /F1 10 Tf 20 150 Td (Name) Tj 40 0 Td (Age) Tj ET",
		"BT /F1 10 Tf 20 130 Td (Ada) Tj 40 0 Td (36) Tj ET",
		"BT /F1 10 Tf 20 110 Td (one two) Tj ET",
	}, "\n"), nil)
	runs, err := Runs(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	lines := Lines(runs)
	if len(lines) != 3 {
		t.Fatalf("%d lines, want 3", len(lines))
	}
	var joined []string
	for _, line := range lines {
		joined = append(joined, Join(line))
	}
	if got, want := strings.Join(joined, "\n"), Assemble(runs); got != want {
		t.Errorf("Lines+Join read\n%q\nAssemble read\n%q", got, want)
	}
	// And the pieces are in reading order, which is what a caller walking
	// them depends on.
	if Join(lines[0]) != "Name Age" {
		t.Errorf("the first line reads %q", Join(lines[0]))
	}
}

// ⛔ THE CASE THAT MADE TablesInRuns NECESSARY. A numbered heading leaves a gap
// after its number; the table below it leaves that same gap empty on every
// row; so the two become ONE two-column table, and the real four-column table
// is lost because the heading's own text reaches across every boundary of it.
//
// Nothing in the alignment says that line is a heading — its SIZE does, and
// what counts as a heading size is a property of the whole document. So the
// caller who knows takes the line out and asks again.
func TestAHeadingAboveATableSwallowsIt(t *testing.T) {
	d := pageWith(t, strings.Join([]string{
		// "2." at 20, its text at 40 and running to 170.
		"BT /F1 12 Tf 20 180 Td (2.) Tj 20 0 Td (Vue d'ensemble technique) Tj ET",
		"BT /F1 10 Tf 40 160 Td (Name) Tj 60 0 Td (Age) Tj 60 0 Td (City) Tj ET",
		"BT /F1 10 Tf 40 145 Td (Ada) Tj 60 0 Td (36) Tj 60 0 Td (Caen) Tj ET",
		"BT /F1 10 Tf 40 130 Td (Bob) Tj 60 0 Td (41) Tj 60 0 Td (Lyon) Tj ET",
	}, "\n"), nil)

	// Asked about the page, the heading is in and the table is two columns of
	// nothing useful.
	whole, err := Tables(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(whole) != 1 {
		t.Fatalf("%d tables, want 1: %+v", len(whole), whole)
	}
	if n := len(whole[0].Rows[0]); n != 2 {
		t.Errorf("%d columns with the heading in, want 2 — the defect: %q", n, whole[0].Rows)
	}

	// Asked about the runs below it, the real table comes back.
	runs, err := Runs(d, 1)
	if err != nil {
		t.Fatal(err)
	}
	var below []Run
	for _, r := range runs {
		if r.Size < 12 {
			below = append(below, r)
		}
	}
	got := TablesInRuns(below, 1, TableOptions{})
	if len(got) != 1 {
		t.Fatalf("%d tables without the heading, want 1: %+v", len(got), got)
	}
	want := [][]string{{"Name", "Age", "City"}, {"Ada", "36", "Caen"}, {"Bob", "41", "Lyon"}}
	if !reflect.DeepEqual(got[0].Rows, want) {
		t.Errorf("without the heading the table reads %q, want %q", got[0].Rows, want)
	}
	// And the page number is the caller's to say, since runs do not carry one.
	if got[0].Page != 1 {
		t.Errorf("the table says it is on page %d", got[0].Page)
	}
}
