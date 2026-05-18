package randfiletree

import (
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestByteNameGeneratorAlphabetProducesCorrectLength(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(42))
	gen := ByteNameGeneratorAlphabet([]byte("abc"))

	for _, length := range []int{1, 5, 10, 50, 255} {
		name := gen(r, length)
		require.Len(t, name, length, "byte length mismatch for length=%d", length)
	}
}

func TestByteNameGeneratorAlphabetDeterministic(t *testing.T) {
	t.Parallel()

	r1 := rand.New(rand.NewSource(42))
	r2 := rand.New(rand.NewSource(42))
	gen := ByteNameGeneratorAlphabet([]byte("abcdef"))

	name1 := gen(r1, 20)
	name2 := gen(r2, 20)

	require.Equal(t, name1, name2)
}

func TestByteNameGeneratorAlphabetUsesOnlyProvidedBytes(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(123))
	alphabet := []byte("xyz")
	gen := ByteNameGeneratorAlphabet(alphabet)

	name := gen(r, 100)
	for _, b := range []byte(name) {
		require.Contains(t, alphabet, b, "byte %q not in alphabet", b)
	}
}

func TestByteNamePresetLeadingSpaces(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(42))
	name := ByteNamePresetLeadingSpaces(r, 10)

	require.Len(t, name, 10)
	require.True(t, strings.HasPrefix(name, " "), "expected leading space")
}

func TestByteNamePresetTrailingSpaces(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(42))
	name := ByteNamePresetTrailingSpaces(r, 10)

	require.Len(t, name, 10)
	require.True(t, strings.HasSuffix(name, " "), "expected trailing space")
}

func TestByteNamePresetLeadingDots(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(42))
	name := ByteNamePresetLeadingDots(r, 10)

	require.Len(t, name, 10)
	require.True(t, strings.HasPrefix(name, "."), "expected leading dot")
}

func TestByteNamePresetNewlineTab(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(42))
	name := ByteNamePresetNewlineTab(r, 10)

	require.Len(t, name, 10)
	hasSpecial := false
	for _, b := range []byte(name) {
		if b == '\n' || b == '\r' || b == '\t' {
			hasSpecial = true
			break
		}
	}
	require.True(t, hasSpecial, "expected newline or tab character")
}

func TestByteNamePresetControlChars(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(42))
	name := ByteNamePresetControlChars(r, 10)

	require.Len(t, name, 10)
	hasControl := false
	for _, b := range []byte(name) {
		if b > 0 && b < 0x20 {
			hasControl = true
			break
		}
	}
	require.True(t, hasControl, "expected control character")
}

func TestByteNamePresetInvalidUTF8(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(42))
	name := ByteNamePresetInvalidUTF8(r, 20)

	require.Len(t, name, 20)
	require.False(t, utf8.ValidString(name), "expected invalid UTF-8 string")
}

func TestByteNamePresetUnicodeNormalization(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(42))
	name := ByteNamePresetUnicodeNormalization(r, 20)

	require.Len(t, name, 20)
	require.True(t, strings.HasPrefix(name, "e"), "expected base character 'e'")
}

func TestByteNamePresetMinLength(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(42))

	for _, preset := range []func(r *rand.Rand, byteLen int) string{
		ByteNamePresetLeadingSpaces,
		ByteNamePresetTrailingSpaces,
		ByteNamePresetLeadingDots,
		ByteNamePresetNewlineTab,
		ByteNamePresetControlChars,
		ByteNamePresetInvalidUTF8,
		ByteNamePresetUnicodeNormalization,
	} {
		name := preset(r, 1)
		require.GreaterOrEqual(t, len(name), 2, "preset should enforce minimum length of 2")
	}
}

func TestByteNameGeneratorPreset(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewSource(42))
	gen := ByteNameGeneratorPreset(ByteNamePresetLeadingDots)

	name := gen(r, 10)
	require.Len(t, name, 10)
	require.True(t, strings.HasPrefix(name, "."))
}
