package randfiletree

import (
	"fmt"
	"hash/fnv"
)

// coverageNameClass enumerates the byte-edge name dimensions exercised by the
// deterministic coverage scenario. Values are stable and define the
// dimension's cardinality; new entries append at the end.
type coverageNameClass uint8

const (
	coverageNameClassBasic coverageNameClass = iota
	coverageNameClassLeadingSpaces
	coverageNameClassTrailingSpaces
	coverageNameClassLeadingDots
	coverageNameClassNewlineTab
	coverageNameClassControlChars
	coverageNameClassInvalidUTF8
	coverageNameClassUnicodeNormalization
	coverageNameClassNearMaxBytes
)

func (c coverageNameClass) String() string {
	switch c {
	case coverageNameClassBasic:
		return "basic"
	case coverageNameClassLeadingSpaces:
		return "leading-spaces"
	case coverageNameClassTrailingSpaces:
		return "trailing-spaces"
	case coverageNameClassLeadingDots:
		return "leading-dots"
	case coverageNameClassNewlineTab:
		return "newline-tab"
	case coverageNameClassControlChars:
		return "control-chars"
	case coverageNameClassInvalidUTF8:
		return "invalid-utf8"
	case coverageNameClassUnicodeNormalization:
		return "unicode-normalization"
	case coverageNameClassNearMaxBytes:
		return "near-max-bytes"
	default:
		return fmt.Sprintf("unknown(%d)", c)
	}
}

var coverageNameClassAll = []coverageNameClass{
	coverageNameClassBasic,
	coverageNameClassLeadingSpaces,
	coverageNameClassTrailingSpaces,
	coverageNameClassLeadingDots,
	coverageNameClassNewlineTab,
	coverageNameClassControlChars,
	coverageNameClassInvalidUTF8,
	coverageNameClassUnicodeNormalization,
	coverageNameClassNearMaxBytes,
}

// coverageBasicAlphabet is the safe basic byte alphabet used as the fill
// material around the edge-case bytes. It must never contain NUL or '/' bytes,
// since both are forbidden in path components.
var coverageBasicAlphabet = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_")

// coverageInvalidUTF8Sequences mirrors the ByteNamePresetInvalidUTF8 set with
// fixed deterministic ordering.
var coverageInvalidUTF8Sequences = [][]byte{
	{0x80}, {0xBF}, {0xC0}, {0xC1},
	{0xF5}, {0xFF},
	{0xC2, 0x80}, {0xE0, 0x80},
}

// coverageCombiningSequence mirrors ByteNamePresetUnicodeNormalization's
// fixed combining-byte sequence (NFD combining marks following 'e').
var coverageCombiningSequence = []byte{0xCC, 0x81, 0xCC, 0x82, 0xCC, 0x83, 0xCC, 0x84}

const (
	coverageNameMinLen        = 8
	coverageNameDefaultLen    = 16
	coverageNameTrailingDigit = 4
)

// coverageDeterministicName returns a byte-string name for the (class, cellID)
// pair. The output depends only on its arguments — no rand.Rand, no map
// iteration. The name is suffixed with a fixed-width hex digest of cellID so
// distinct cells in the same class never collide.
//
// targetLen is a soft target. NearMaxBytes ignores it and emits a name close to
// NameMaxBytes; the others honor it but clamp to a safe minimum.
func coverageDeterministicName(class coverageNameClass, targetLen int, cellID uint64) string {
	if targetLen < coverageNameMinLen {
		targetLen = coverageNameDefaultLen
	}

	suffix := coverageCellIDSuffix(cellID)
	bodyLen := targetLen - len(suffix)
	if bodyLen < 1 {
		bodyLen = 1
	}

	switch class {
	case coverageNameClassNearMaxBytes:
		return coverageNameNearMax(cellID, suffix)
	default:
		body := coverageNameBody(class, bodyLen, cellID)
		return body + suffix
	}
}

// coverageCellIDSuffix is a fixed-width 8-byte hex digest of cellID, prefixed
// with a separator that survives every byte-edge name class (no slashes, no
// NULs, not in the leading-dots / leading-spaces prefix zone).
func coverageCellIDSuffix(cellID uint64) string {
	const hex = "0123456789abcdef"
	out := make([]byte, 1+8)
	out[0] = '_'
	for i := 0; i < 8; i++ {
		out[1+i] = hex[(cellID>>uint(4*(7-i)))&0xF]
	}

	return string(out)
}

func coverageNameBody(class coverageNameClass, bodyLen int, cellID uint64) string {
	if bodyLen <= 0 {
		bodyLen = 1
	}

	body := make([]byte, bodyLen)

	// Mix cellID into a deterministic offset stream so different cells in the
	// same class do not produce identical names before the suffix is appended.
	h := fnv.New64a()
	var seedBytes [8]byte
	for i := 0; i < 8; i++ {
		seedBytes[i] = byte(cellID >> uint(8*i))
	}
	_, _ = h.Write(seedBytes[:])
	stream := h.Sum64()

	advance := func() byte {
		// Use a 32-bit step constant (LCG-style) to walk the stream
		// deterministically without overlapping consecutive bytes.
		stream = stream*6364136223846793005 + 1442695040888963407
		return byte(stream >> 56)
	}

	for i := range body {
		body[i] = coverageBasicAlphabet[int(advance())%len(coverageBasicAlphabet)]
	}

	switch class {
	case coverageNameClassBasic:
		// Body already filled.

	case coverageNameClassLeadingSpaces:
		nSpaces := 1 + int(advance())%3
		if nSpaces > bodyLen-1 {
			nSpaces = bodyLen - 1
			if nSpaces < 1 {
				nSpaces = 1
			}
		}
		for i := 0; i < nSpaces && i < bodyLen; i++ {
			body[i] = ' '
		}

	case coverageNameClassTrailingSpaces:
		nSpaces := 1 + int(advance())%3
		if nSpaces > bodyLen-1 {
			nSpaces = bodyLen - 1
			if nSpaces < 1 {
				nSpaces = 1
			}
		}
		for i := 0; i < nSpaces; i++ {
			pos := bodyLen - 1 - i
			if pos < 0 {
				break
			}
			body[pos] = ' '
		}

	case coverageNameClassLeadingDots:
		nDots := 1 + int(advance())%2
		if nDots > bodyLen-1 {
			nDots = bodyLen - 1
			if nDots < 1 {
				nDots = 1
			}
		}
		for i := 0; i < nDots && i < bodyLen; i++ {
			body[i] = '.'
		}

	case coverageNameClassNewlineTab:
		specials := []byte{'\n', '\r', '\t'}
		nSpecial := 1 + int(advance())%3
		if nSpecial > bodyLen-1 {
			nSpecial = bodyLen - 1
			if nSpecial < 1 {
				nSpecial = 1
			}
		}
		for i := 0; i < nSpecial && i < bodyLen; i++ {
			body[i] = specials[int(advance())%len(specials)]
		}

	case coverageNameClassControlChars:
		nControl := 1 + int(advance())%4
		if nControl > bodyLen-1 {
			nControl = bodyLen - 1
			if nControl < 1 {
				nControl = 1
			}
		}
		for i := 0; i < nControl && i < bodyLen; i++ {
			body[i] = byte(1 + int(advance())%31)
		}

	case coverageNameClassInvalidUTF8:
		nInvalid := 1 + int(advance())%3
		pos := 0
		for i := 0; i < nInvalid && pos < bodyLen; i++ {
			seq := coverageInvalidUTF8Sequences[int(advance())%len(coverageInvalidUTF8Sequences)]
			for _, b := range seq {
				if pos >= bodyLen {
					break
				}
				body[pos] = b
				pos++
			}
		}

	case coverageNameClassUnicodeNormalization:
		pos := 0
		body[pos] = 'e'
		pos++
		nCombining := 1 + int(advance())%3
		for i := 0; i < nCombining && pos+1 < bodyLen; i++ {
			body[pos] = coverageCombiningSequence[(i*2)%len(coverageCombiningSequence)]
			pos++
			if pos < bodyLen {
				body[pos] = coverageCombiningSequence[(i*2+1)%len(coverageCombiningSequence)]
				pos++
			}
		}
	}

	return string(body)
}

// coverageNameNearMax produces a name whose total length sits close to
// NameMaxBytes (255). The suffix anchors uniqueness; the body fills the
// remainder with the safe alphabet.
func coverageNameNearMax(cellID uint64, suffix string) string {
	bodyLen := NameMaxBytes - len(suffix)
	if bodyLen < 1 {
		bodyLen = 1
	}

	h := fnv.New64a()
	var seedBytes [8]byte
	for i := 0; i < 8; i++ {
		seedBytes[i] = byte(cellID >> uint(8*i))
	}
	_, _ = h.Write(seedBytes[:])
	stream := h.Sum64()

	body := make([]byte, bodyLen)
	for i := range body {
		stream = stream*6364136223846793005 + 1442695040888963407
		body[i] = coverageBasicAlphabet[int(stream>>56)%len(coverageBasicAlphabet)]
	}

	return string(body) + suffix
}

func coverageCellID(parts ...string) uint64 {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}

	return h.Sum64()
}
