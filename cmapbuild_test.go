package extract

import (
	"encoding/binary"
	"sort"

	"github.com/go-opentype/fonts"
)

// A symbolic TrueType font embedded in a PDF is addressed through its own
// character map, and which subtables it carries decides what can be read back
// out of it. Real fonts carry whichever their maker chose, so these build one
// carrying exactly the subtables a test is about, around the glyphs of a real
// font so the rest of the program stays true.

// cmapSpec is one subtable of a synthetic cmap table: the platform and
// encoding it claims to be written for, and the codes it maps.
type cmapSpec struct {
	platform uint16
	encoding uint16
	codes    map[rune]uint16
}

// fontWithCmaps re-emits a real TrueType font with its character map replaced
// by the given subtables, leaving its glyphs, metrics and every other table
// alone.
func fontWithCmaps(specs []cmapSpec) []byte {
	src := fonts.MostLegible()
	type table struct {
		tag  string
		data []byte
	}
	var tables []table
	n := int(binary.BigEndian.Uint16(src[4:]))
	for i := range n {
		rec := src[12+i*16:]
		tag := string(rec[:4])
		if tag == "cmap" {
			continue
		}
		off := int(binary.BigEndian.Uint32(rec[8:]))
		length := int(binary.BigEndian.Uint32(rec[12:]))
		tables = append(tables, table{tag, src[off : off+length]})
	}
	tables = append(tables, table{"cmap", cmapTableOf(specs)})
	sort.Slice(tables, func(i, j int) bool { return tables[i].tag < tables[j].tag })

	out := make([]byte, 12+16*len(tables))
	binary.BigEndian.PutUint32(out, 0x00010000)
	binary.BigEndian.PutUint16(out[4:], uint16(len(tables)))
	for i, t := range tables {
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
		rec := out[12+i*16:]
		copy(rec[:4], t.tag)
		binary.BigEndian.PutUint32(rec[8:], uint32(len(out)))
		binary.BigEndian.PutUint32(rec[12:], uint32(len(t.data)))
		out = append(out, t.data...)
	}
	return out
}

// cmapTableOf builds a cmap table whose records carry the platform and
// encoding each spec asks for.
func cmapTableOf(specs []cmapSpec) []byte {
	head := make([]byte, 4+8*len(specs))
	binary.BigEndian.PutUint16(head[2:], uint16(len(specs)))
	body := []byte{}
	for i, s := range specs {
		rec := head[4+i*8:]
		binary.BigEndian.PutUint16(rec, s.platform)
		binary.BigEndian.PutUint16(rec[2:], s.encoding)
		binary.BigEndian.PutUint32(rec[4:], uint32(len(head)+len(body)))
		body = append(body, cmap4Of(s.codes)...)
	}
	return append(head, body...)
}

// cmap4Of builds a format-4 subtable, one segment per code, with the sentinel
// segment the format requires.
func cmap4Of(codes map[rune]uint16) []byte {
	runes := make([]int, 0, len(codes))
	for r := range codes {
		runes = append(runes, int(r))
	}
	sort.Ints(runes)
	runes = append(runes, 0xFFFF)

	be := binary.BigEndian
	seg := len(runes)
	out := make([]byte, 14+8*seg+2)
	be.PutUint16(out, 4)
	be.PutUint16(out[2:], uint16(len(out)))
	be.PutUint16(out[6:], uint16(seg*2))
	for i, r := range runes {
		// A code the caller did not ask for -- the sentinel among them --
		// takes the delta the sentinel segment is required to carry, which
		// lands 0xFFFF on .notdef.
		delta := uint16(1)
		if g, ok := codes[rune(r)]; ok {
			delta = g - uint16(r)
		}
		be.PutUint16(out[14+2*i:], uint16(r))       // endCode
		be.PutUint16(out[16+2*seg+2*i:], uint16(r)) // startCode
		be.PutUint16(out[16+4*seg+2*i:], delta)     // idDelta
	}
	return out
}
