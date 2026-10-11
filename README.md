# extract

[![CI](https://github.com/go-pdfkit/extract/actions/workflows/ci.yml/badge.svg)](https://github.com/go-pdfkit/extract/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-pdfkit/extract.svg)](https://pkg.go.dev/github.com/go-pdfkit/extract)
[![License: BSD-3-Clause](https://img.shields.io/badge/License-BSD--3--Clause-blue.svg)](LICENSE)
[![Coverage](https://img.shields.io/badge/coverage-100%25-brightgreen.svg)](#how-it-is-checked)

Reads a PDF page back: the text on it, with where each piece of it sits, and
the images it places.

```go
text, err := extract.Text(doc, 1)     // the page as words, lines put back together
runs, err := extract.Runs(doc, 1)     // every piece, with its place and its size
imgs, err := extract.Images(doc, 1)   // every picture, and where it lands
tabs, err := extract.Tables(doc, 1)   // the grids its text lines up into
```

Pure Go, no C. It builds for `GOOS=js/wasm` and every architecture the fleet
targets.

## What a page actually holds

Not text. A page holds instructions for drawing glyphs, and the text is what
those glyphs were meant to say — which the document says only if it was asked
to. Where it does not, this says so rather than guessing: a run whose codes
could not be worked out comes back marked `Unreadable`, so a caller can tell
*this page says nothing* from *this page could not be read*.

Three things are tried, in order:

1. the font's `/ToUnicode` map, which is the document's own word on what its
   text says;
2. the name its encoding gives the code — but only a name the document chose
   itself, since a **symbolic** font read through an assumed encoding gives
   the wrong letter with nothing to say so;
3. what the **embedded font program** calls the code, read with
   [`go-opentype`](https://github.com/go-opentype/opentype) — which is the
   honest answer for a symbolic font, and the reason a page of mathematics
   comes back as mathematics.

Text drawn in the mode that puts no ink on the page — how a scanner writes
what it read underneath the picture it read it from — is returned and marked
`Invisible`.

## Images

A picture comes back as the file holds it, with the box it covers on the page.
A JPEG comes out as a JPEG: re-encoding it would lose something for nothing.
Turning the other kinds into pixels means reading their colour space, which is
[`render`](https://github.com/go-pdfkit/render)'s work rather than this
package's.

## Tables

A page holds no table. It holds text put at places, and a table is what a
reader sees when those places line up — so `Tables` reports an **inference**,
and there is nothing in the file to check it against.

A column boundary is a stretch of the page that **every row of the table
leaves empty**. The first line of a block proposes its own gaps, each line
after it takes away whatever it draws in them, and a line that leaves nothing
closes the block and begins the next. That is the one decision everything else
follows from, and it is **subtraction** rather than the intersection of the
lines' gaps, which is where this started:

| | |
|---|---|
| a cell whose text **wraps** | leaves a line with one piece of text on it and so no gaps of its own. Intersecting with nothing ends the table, and a four-row table with one wrapped cell came back as **two tables of two rows**. A wrapped line draws nothing in the boundary beside it, so subtracting keeps it. |

Three things are not columns, and each one was found by running this over real
documents rather than over a fixture:

| | |
|---|---|
| a **bullet** | a bulleted list *is* a two-column grid. What tells them apart is that one column says the same thing wherever it says anything — and a column like that distinguishes nothing. A mark of one or two characters, repeated, is dropped; `CNRS` repeated down a column is data and is kept. |
| a column **nothing was drawn in** | that is a stretch of page nothing was drawn in, which is what a boundary already is. A run of nothing but spaces splits a boundary in two and leaves an empty column between the halves. |
| a line **far below** the row above | a page number sits in the left-hand column and draws nothing in any boundary, which is exactly what a row of a table does. What says it is not one is the distance: more than three times the row's own text size. |

Measured on one 213-page document, as each of those went in:

```
588 tables, 1841 rows   gaps the rows share
213 tables, 1631 rows   + boundaries the rows leave empty (wrapped cells)
 83 tables,  268 rows    + a repeated mark is a bullet
 40 tables,  120 rows     + an empty column is not a column, and a page
                            number is not a row
```

The forty are the document's `Code` / `Intitulé` pairs, one per job
description, with their second cells wrapped. A 3-page prose report in the same
run reports **none**, which is the control: a detector that cannot say no to a
paragraph says yes to every page there is.

⛔ **What it still gets wrong.** A page laid out in two columns of prose is a
grid too — the gutter is a stretch every line leaves empty. What saves the
ordinary case is that prose does not stay out of its gutter for long, since one
full-width line closes the block. And ruled lines are not read at all: this is
the *text* strategy, the one a bordered table does not need, and a table whose
columns are held apart by rules rather than by whitespace is not found by it.

## How it is checked

100% of statements, including every branch that handles a page saying
something it should not.

Then on the corpus — 118 833 real PDFs, 121 946 pages:

```
pages read         121946
  with text        94865 (77.8%)
  saying nothing   27081, of which 4812 did draw glyphs
characters         33976414 (17272226 of them letters)
runs               10797045
  unreadable       203607 (1.89%)
  invisible        29917
images             300685
panics             0
```

The 22% of pages that say nothing are figures — this corpus is arXiv's, and
most of it is plots. Of the pages that *did* draw glyphs and still said
nothing, nearly all are Type 3 fonts with no `/ToUnicode`: dvips bitmap fonts
whose glyphs are called `a1` and `s32`, which no reader can turn back into
letters.

## The rest of the family

[**reader**](https://github.com/go-pdfkit/reader) reads and writes the format ·
[**ops**](https://github.com/go-pdfkit/ops) is the verbs and the `pdfops` command ·
[**render**](https://github.com/go-pdfkit/render) turns a page into pixels ·
[**pdffont**](https://github.com/go-pdfkit/pdffont) is what a document says about a font ·
[**extract**](https://github.com/go-pdfkit/extract) reads a page back ·
[**coedit**](https://github.com/go-pdfkit/coedit) is a PDF several people edit at once ·
[**app**](https://github.com/go-pdfkit/app) is the browser workbench.

Documentation for all of it: <https://go-pdfkit.github.io/docs/>

## Licence

BSD-3-Clause.
