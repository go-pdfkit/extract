package extract

import (
	"bytes"
	"fmt"
)

// The cipher a Type 1 program is written in.
const (
	eexecC1  = 52845
	eexecC2  = 22719
	eexecKey = 55665
	csKey    = 4330
)

// eexecEncrypt writes a section the way a Type 1 program carries it.
func eexecEncrypt(plain []byte, key uint16, lead int) []byte {
	r := key
	buf := append(bytes.Repeat([]byte{'X'}, lead), plain...)
	out := make([]byte, 0, len(buf))
	for _, p := range buf {
		c := p ^ byte(r>>8)
		r = (uint16(c)+r)*eexecC1 + eexecC2
		out = append(out, c)
	}
	return out
}

// t1num encodes numbers the way a Type 1 charstring carries them.
func t1num(vs ...int) []byte {
	var out []byte
	for _, v := range vs {
		switch {
		case v >= -107 && v <= 107:
			out = append(out, byte(v+139))
		default:
			v -= 108
			out = append(out, byte(v/256+247), byte(v%256))
		}
	}
	return out
}

// t1Box is a charstring drawing a square of the given width.
func t1Box(width int) []byte {
	cs := append(t1num(0, width), 13) // hsbw
	cs = append(cs, t1num(50, 50)...)
	cs = append(cs, 21) // rmoveto
	cs = append(cs, t1num(400)...)
	cs = append(cs, 6) // hlineto
	cs = append(cs, t1num(400)...)
	cs = append(cs, 7, 9, 14) // vlineto closepath endchar
	return cs
}

// type1Program builds a Type 1 font program whose own encoding puts the named
// glyphs at the given codes, which is what a symbolic font is addressed
// through.
func type1Program(encoding map[byte]string) []byte {
	var clear bytes.Buffer
	clear.WriteString("%!PS-AdobeFont-1.0: Synthetic 001.001\n/FontName /Synthetic def\n")
	clear.WriteString("/FontMatrix [0.001 0 0 0.001 0 0] readonly def\n/Encoding 256 array\n")
	for code := 0; code < 256; code++ {
		if name, ok := encoding[byte(code)]; ok {
			fmt.Fprintf(&clear, "dup %d /%s put\n", code, name)
		}
	}
	clear.WriteString("readonly def\ncurrentdict end\ncurrentfile eexec\n")

	var priv bytes.Buffer
	priv.WriteString("XXXX dup /Private 8 dict dup begin\n/lenIV 4 def\n")
	fmt.Fprintf(&priv, "/CharStrings %d dict dup begin\n", len(encoding)+1)
	notdef := eexecEncrypt([]byte{14}, csKey, 4)
	fmt.Fprintf(&priv, "/.notdef %d RD ", len(notdef))
	priv.Write(notdef)
	priv.WriteString(" ND\n")
	for _, name := range encoding {
		enc := eexecEncrypt(t1Box(600), csKey, 4)
		fmt.Fprintf(&priv, "/%s %d RD ", name, len(enc))
		priv.Write(enc)
		priv.WriteString(" ND\n")
	}
	priv.WriteString("end\nend\n")
	return append(clear.Bytes(), eexecEncrypt(priv.Bytes(), eexecKey, 4)...)
}
