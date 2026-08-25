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

## Licence

BSD-3-Clause.
