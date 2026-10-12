package extract

import (
	"sort"
	"strings"
	"unicode"

	"github.com/go-pdfkit/reader"
)

// A Table is a grid of text read off a page.
//
// A PDF page does not contain a table. It contains text put at places, and a
// table is what a reader sees when those places line up. So a Table here is
// an inference, and [Table.Rows] is what the alignment said rather than what
// the author declared — there is nothing in the file to declare it with.
type Table struct {
	// Page is the page the table was found on, counting from one.
	Page int

	// Rows holds the cells, top row first and left to right within a row.
	// Every row has the same number of cells, and a cell the page left empty
	// is an empty string.
	Rows [][]string

	// X and Y are the bottom left corner of the TEXT the table is made of,
	// and Width and Height its extent, in the page's own coordinates. A table
	// drawn inside a frame has a frame a little larger than this; a table
	// drawn without one has no frame at all, which is the usual case and the
	// reason the box is measured from the words.
	//
	// It is every word of the block, including a bullet left out of
	// [Table.Rows]: the box says where to look on the page, and a reader
	// looking there wants to see the bullets too.
	X, Y, Width, Height float64
}

// TableOptions tunes what counts as a table. A zero field means the default,
// so TableOptions{} is the default everywhere and a caller changes one thing
// by naming one field.
type TableOptions struct {
	// ColumnGapSpaces is how wide a gap must be, counted in spaces, before it
	// separates two columns rather than two words. The default is 1.5: one
	// space is a word break, and a column boundary is wider than a word break
	// or there would be no way to tell a table from a sentence.
	ColumnGapSpaces float64

	// MinRows and MinColumns are the smallest grid worth reporting, both 2 by
	// default. One row is a line of text and one column is a paragraph.
	MinRows, MinColumns int
}

// The defaults, named so that the test and the documentation cannot drift
// apart from the code.
const (
	defaultColumnGapSpaces = 1.5
	defaultMinRows         = 2
	defaultMinColumns      = 2

	// markerRunes is how long a cell can be and still be a bullet rather than
	// data. One rune covers the bullet characters, two covers "a." and "1)".
	markerRunes = 2

	// rowSpacingLimit is how far below the row above a line can sit and still
	// be the next row, counted in the taller row's own text size. Ordinary
	// leading is 1.2 of it and a generous table's is about 2, so three leaves
	// room for a table set loosely while still putting the bottom of the page
	// out of reach.
	rowSpacingLimit = 3
)

// Tables is every table on the page, top to bottom, with the default options.
func Tables(d *reader.Document, page int) ([]Table, error) {
	return TablesWithOptions(d, page, TableOptions{})
}

// TablesWithOptions is [Tables] with the thresholds said out loud.
//
// ⛔ A page laid out in two columns of prose is a grid too: the gutter is a
// stretch every line leaves empty, so a short enough stretch of such a page
// comes back as a two-column table whose cells are lines of text. There is
// nothing in the page that distinguishes the two. What saves the ordinary case
// is that prose does not stay out of the gutter for long — one full-width line
// closes the block — but a caller who knows the document is prose should not
// be asking. This reports alignment; it does not read minds.
func TablesWithOptions(d *reader.Document, page int, o TableOptions) ([]Table, error) {
	runs, err := Runs(d, page)
	if err != nil {
		return nil, err
	}
	return TablesInRuns(runs, page, o), nil
}

// TablesInRuns is [TablesWithOptions] over runs a caller chose, and page is
// only the number it will put in each [Table.Page].
//
// ⛔ It exists so that a caller who knows something this does not can take
// lines OUT before asking. A numbered heading above a table is the case that
// made it necessary: "2. Vue d'ensemble" leaves a gap after its number, the
// table below it leaves that same gap empty on every row, so the heading and
// the table became ONE two-column table — and the real four-column table
// under it was lost, because the heading's own text reached across every one
// of its boundaries.
//
// Nothing in the alignment says that line is a heading. Its SIZE says so, and
// what counts as a heading size is a property of the whole document, which is
// [go-pdfkit/document]'s business and not this package's.
//
// [go-pdfkit/document]: https://pkg.go.dev/github.com/go-pdfkit/document
func TablesInRuns(runs []Run, page int, o TableOptions) []Table {
	return tablesIn(runs, page, o.withDefaults())
}

// withDefaults fills in the fields left at zero.
func (o TableOptions) withDefaults() TableOptions {
	if o.ColumnGapSpaces <= 0 {
		o.ColumnGapSpaces = defaultColumnGapSpaces
	}
	if o.MinRows <= 0 {
		o.MinRows = defaultMinRows
	}
	if o.MinColumns <= 0 {
		o.MinColumns = defaultMinColumns
	}
	return o
}

// an interval is a stretch of the x axis.
type interval struct{ start, end float64 }

// tablesIn finds the tables among a page's runs.
//
// A column boundary is a stretch of the page that EVERY row of the table
// leaves empty. So the first line of a block proposes its own gaps as
// boundaries, and each line after that takes away whatever it draws in them.
// A line that leaves nothing of them closes the block and starts the next one.
//
// ⛔ It is subtraction and not the intersection of the lines' gaps, which is
// what this did first and what cost it the tables it was written for. A cell
// whose text wraps leaves a line with ONE piece of text on it and therefore no
// gaps of its own; intersecting with nothing ends the table, so a four-row
// table with one wrapped cell came back as two tables of two rows. Subtracting
// asks the question that was meant all along — is there anything DRAWN there —
// and a wrapped line sitting inside a column draws nothing in the boundary
// beside it.
func tablesIn(runs []Run, page int, o TableOptions) []Table {
	lines := tableLines(runs)
	if len(lines) == 0 {
		return nil
	}
	gap := minGap(runs, o.ColumnGapSpaces)
	var out []Table
	for start := 0; start < len(lines); {
		seps := gapsIn(lines[start], gap)
		end := start + 1
		for ; end < len(lines); end++ {
			if tooFarBelow(lines[end-1], lines[end]) {
				break
			}
			left := clip(seps, lines[end], gap)
			if len(left) == 0 {
				break
			}
			seps = left
		}
		if t, ok := tableOf(lines[start:end], seps, page, o); ok {
			out = append(out, t)
		}
		start = end
	}
	return out
}

// tableLines is the page's lines with the blank ones left out. A line of
// nothing but spaces draws nothing anywhere, so keeping it would add a row of
// empty cells to every table that has whitespace drawn with text in it.
func tableLines(runs []Run) [][]Run {
	var out [][]Run
	for _, line := range groupIntoLines(runs) {
		if strings.TrimSpace(joinLine(line)) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

// minGap is how many points wide a gap has to be here. It is measured from
// the page's own text — the middle space width among its runs — because a gap
// of four points separates columns in a footnote and falls inside a word in a
// title.
//
// The middle one, and not the narrowest or the widest, because both of those
// let one line decide for the page and they fail in opposite directions: the
// narrowest turns every word gap in the body text into a column, the widest
// turns every column into a word gap. With an even number of runs it is the
// upper of the two middles, which is a tie-break and not a judgement.
func minGap(runs []Run, spaces float64) float64 {
	widths := make([]float64, 0, len(runs))
	for _, r := range runs {
		w := r.Space
		if w <= 0 {
			// A font that says nothing about its space: a quarter of an em is
			// what [wantsSpace] assumes for the same reason.
			w = r.Size * 0.25
		}
		if w > 0 {
			widths = append(widths, w)
		}
	}
	if len(widths) == 0 {
		// Text set at no size at all. Any positive number will do, since
		// nothing on this page has a width to compare it with; what matters is
		// that it is NOT zero, or every pair of touching runs would be two
		// columns.
		return spaces
	}
	sort.Float64s(widths)
	return widths[len(widths)/2] * spaces
}

// gapsIn is the stretches of a line wide enough to be column boundaries.
//
// The right edge is carried forward as a maximum rather than read off the
// previous run, because runs are ordered by where they START: a wide run
// followed by a narrow one inside it would otherwise look like a gap.
func gapsIn(line []Run, gap float64) []interval {
	var out []interval
	right := line[0].X + line[0].Width
	for _, r := range line[1:] {
		if r.X-right >= gap {
			out = append(out, interval{right, r.X})
		}
		if end := r.X + r.Width; end > right {
			right = end
		}
	}
	return out
}

// tooFarBelow reports that a line is too far down the page to be the next row
// of the block above it.
//
// ⛔ Measured: without this, the page number at the foot of the page became
// the last row of a bullet list two thirds of the way up it. Nothing else
// stopped it — a page number sits in the left-hand column and draws nothing in
// any boundary, which is exactly what a row of a table does — and because its
// cell was neither a bullet nor empty it was also what kept that list from
// being recognised as one.
func tooFarBelow(prev, next []Run) bool {
	size := prev[0].Size
	for _, r := range prev {
		if r.Size > size {
			size = r.Size
		}
	}
	if size <= 0 {
		// Text set at no size says nothing about how far apart its lines
		// should be, so this has no opinion either.
		return false
	}
	return prev[0].Y-next[0].Y > rowSpacingLimit*size
}

// clip takes out of each boundary whatever the line draws in it, and drops
// what is left over of a boundary too narrow to be one. A boundary a run
// crosses in the middle comes back as the two pieces either side of it, so a
// stretch of white can survive as a narrower boundary rather than only
// surviving whole.
func clip(seps []interval, line []Run, gap float64) []interval {
	var out []interval
	for _, s := range seps {
		pieces := []interval{s}
		for _, r := range line {
			pieces = without(pieces, interval{r.X, r.X + r.Width})
		}
		for _, p := range pieces {
			if p.end-p.start >= gap {
				out = append(out, p)
			}
		}
	}
	return out
}

// without is the pieces with the stretch taken out of them.
//
// The two comparisons below are deliberately strict and no test can tell them
// from the loose ones: a loose one manufactures a piece of no width, and a
// piece of no width cannot get past the width check in [clip]. They are strict
// so that nothing downstream has to tolerate one, not because the behaviour
// differs — which is worth saying, because a mutation that swaps them looks
// like an uncovered branch and is not one.
func without(pieces []interval, drawn interval) []interval {
	var out []interval
	for _, p := range pieces {
		if drawn.end <= p.start || drawn.start >= p.end {
			out = append(out, p)
			continue
		}
		if drawn.start > p.start {
			out = append(out, interval{p.start, drawn.start})
		}
		if drawn.end < p.end {
			out = append(out, interval{drawn.end, p.end})
		}
	}
	return out
}

// tableOf turns a block of lines and the boundaries they share into a table,
// or reports that the block is not one.
func tableOf(block [][]Run, seps []interval, page int, o TableOptions) (Table, bool) {
	if len(block) < o.MinRows {
		return Table{}, false
	}
	rows := make([][]string, len(block))
	for i, line := range block {
		rows[i] = cellsOf(line, seps)
	}
	rows = dropEmptyColumns(rows)
	if len(rows[0]) > 1 && markerColumn(rows) {
		for i := range rows {
			rows[i] = rows[i][1:]
		}
	}
	if len(rows[0]) < o.MinColumns {
		return Table{}, false
	}
	t := Table{Page: page, Rows: rows}
	t.X, t.Y, t.Width, t.Height = boxOf(block)
	return t, true
}

// cellsOf puts a line's pieces of text into the columns the boundaries make.
func cellsOf(line []Run, seps []interval) []string {
	in := make([][]Run, len(seps)+1)
	for _, r := range line {
		c := columnOf(seps, r)
		in[c] = append(in[c], r)
	}
	row := make([]string, len(in))
	for c, runs := range in {
		row[c] = joinLine(runs)
	}
	return row
}

// dropEmptyColumns removes the columns that no row said anything in.
//
// ⛔ Measured: a 213-page document gave blocks of six and fifteen rows whose
// second and third columns were empty the whole way down. A column nothing was
// drawn in is not a column — it is a stretch of page nothing was drawn in,
// which is what a boundary already is — and counting it is how one column of
// left-aligned paragraphs passes for a table.
func dropEmptyColumns(rows [][]string) [][]string {
	keep := make([]int, 0, len(rows[0]))
	for c := range rows[0] {
		for _, row := range rows {
			if row[c] != "" {
				keep = append(keep, c)
				break
			}
		}
	}
	if len(keep) == len(rows[0]) {
		return rows
	}
	out := make([][]string, len(rows))
	for i, row := range rows {
		cells := make([]string, len(keep))
		for j, c := range keep {
			cells[j] = row[c]
		}
		out[i] = cells
	}
	return out
}

// markerColumn reports whether the first column holds a list's bullets rather
// than data: short marks, and nothing that is a word or a number.
//
// ⛔ Measured, on a 213-page document of the author's: 588 "tables" of two
// columns, and the first column of nearly all of them held "•" or U+F0A7. A
// bulleted list IS a two-column grid, and no amount of looking at the
// alignment will say otherwise. What distinguishes it is that one of its
// columns holds no DATA — a mark of one or two characters with no letter and
// no digit in it is an ornament, and dropping it leaves a list with one
// column, which is not a table.
//
// ⛔ A MARK, not the same mark twice. Insisting on one repeated mark read
// "• • - -" as a table of four rows, because the bullets were not all the
// same character — and a reader sees two lists there. The question is whether
// the column carries information, and `-` carries exactly as little as `•`.
//
// ⛔ A letter or a digit is information, which is what keeps a real table.
// `A` and `B` down a first column are labels, `1.` and `2.` are a numbering
// somebody may want, and `1990` is a date. Taking those would be deciding
// that a table of years is a list.
//
// ⛔ The empty cells are why this is not simply "every row holds one": a
// bullet's text wraps, and the lines it wraps onto have no bullet of their
// own. Insisting on every row let the wrapped lists back in — which is how
// this read as a fix while a document of nothing but bullet lists still came
// back as fourteen rows of table.
func markerColumn(rows [][]string) bool {
	for _, row := range rows {
		if row[0] == "" {
			continue
		}
		if !isMark(row[0]) {
			return false
		}
	}
	// Reaching here means every cell of the column was empty or a mark, and
	// they cannot all have been empty: [dropEmptyColumns] has already taken
	// away the columns nothing was drawn in. So at least one was a mark, and
	// there is no "found nothing" case to report.
	return true
}

// isMark reports whether a cell is an ornament rather than data.
func isMark(cell string) bool {
	runes := []rune(cell)
	if len(runes) == 0 || len(runes) > markerRunes {
		return false
	}
	for _, r := range runes {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// boxOf is the box the block's text occupies. The block has at least one line
// and a line has at least one run — [groupIntoLines] makes lines out of runs —
// so there is always a first run to start from.
func boxOf(block [][]Run) (x, y, w, h float64) {
	first := block[0][0]
	left, bottom := first.X, first.Y
	right, top := first.X+first.Width, first.Y+first.Size
	for _, line := range block {
		for _, r := range line {
			left, bottom = min(left, r.X), min(bottom, r.Y)
			right, top = max(right, r.X+r.Width), max(top, r.Y+r.Size)
		}
	}
	return left, bottom, right - left, top - bottom
}

// columnOf is which column a run sits in: the number of boundaries that are
// wholly to its left. A run cannot straddle a boundary — a boundary is a
// stretch where no line in the block draws anything — so this is exact rather
// than nearest.
func columnOf(seps []interval, r Run) int {
	n := 0
	for _, s := range seps {
		if s.end <= r.X {
			n++
		}
	}
	return n
}
