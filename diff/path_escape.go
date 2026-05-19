package diff

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func EscapePathForDisplay(path string) string {
	if path == "" {
		return ""
	}

	var out strings.Builder
	out.Grow(len(path) * 2)

	i := 0
	for i < len(path) {
		r, size := utf8.DecodeRuneInString(path[i:])
		if r == utf8.RuneError && size == 1 {
			out.WriteString("\\x")
			out.WriteString(hexByte(path[i]))
			i++
			continue
		}

		if r < 0x20 || r == 0x7F {
			out.WriteString("\\x")
			out.WriteString(hexByte(path[i]))
			i += size
			continue
		}

		if !unicode.IsPrint(r) && r != '\t' {
			for j := 0; j < size; j++ {
				out.WriteString("\\x")
				out.WriteString(hexByte(path[i+j]))
			}
			i += size
			continue
		}

		out.WriteString(path[i : i+size])
		i += size
	}

	return out.String()
}

var hexDigits = []byte("0123456789abcdef")

func hexByte(b byte) string {
	return string([]byte{hexDigits[b>>4], hexDigits[b&0x0F]})
}
