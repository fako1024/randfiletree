package diff

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

func EscapePathForDisplay(path string) string {
	if path == "" {
		return ""
	}

	var out []byte
	i := 0
	for i < len(path) {
		r, size := utf8.DecodeRuneInString(path[i:])
		if r == utf8.RuneError && size == 1 {
			out = append(out, fmt.Sprintf("\\x%02x", path[i])...)
			i++
			continue
		}

		if r < 0x20 || r == 0x7F {
			out = append(out, fmt.Sprintf("\\x%02x", path[i])...)
			i += size
			continue
		}

		if !unicode.IsPrint(r) && r != '\t' {
			for j := 0; j < size; j++ {
				out = append(out, fmt.Sprintf("\\x%02x", path[i+j])...)
			}
			i += size
			continue
		}

		out = append(out, path[i:i+size]...)
		i += size
	}

	return string(out)
}
