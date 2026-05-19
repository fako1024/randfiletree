package diff

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEscapePathForDisplayEmpty(t *testing.T) {
	t.Parallel()

	require.Equal(t, "", EscapePathForDisplay(""))
}

func TestEscapePathForDisplayPrintable(t *testing.T) {
	t.Parallel()

	input := "/path/to/normal-file.txt"
	require.Equal(t, input, EscapePathForDisplay(input))
}

func TestEscapePathForDisplayControlChars(t *testing.T) {
	t.Parallel()

	input := "/path\x01/to\x02/file"
	output := EscapePathForDisplay(input)

	require.Contains(t, output, "\\x01")
	require.Contains(t, output, "\\x02")
	require.NotContains(t, output, "\x01")
	require.NotContains(t, output, "\x02")
}

func TestEscapePathForDisplayNewlineTab(t *testing.T) {
	t.Parallel()

	input := "/path\n/to\t/file"
	output := EscapePathForDisplay(input)

	require.Contains(t, output, "\\x0a")
	require.Contains(t, output, "\\x09")
}

func TestEscapePathForDisplayInvalidUTF8(t *testing.T) {
	t.Parallel()

	input := "/path\x80\xff/to/file"
	output := EscapePathForDisplay(input)

	require.Contains(t, output, "\\x80")
	require.Contains(t, output, "\\xff")
}

func TestEscapePathForDisplayValidUTF8(t *testing.T) {
	t.Parallel()

	input := "/path/\xc3\xa4\xc3\xb6\xc3\xbc/file"
	output := EscapePathForDisplay(input)

	require.Equal(t, input, output)
}

func TestEscapePathForDisplayMixedContent(t *testing.T) {
	t.Parallel()

	input := "/normal\x01/path\xc3\xa4\x80/end"
	output := EscapePathForDisplay(input)

	require.Contains(t, output, "normal")
	require.Contains(t, output, "\\x01")
	require.Contains(t, output, "\xc3\xa4")
	require.Contains(t, output, "\\x80")
	require.Contains(t, output, "end")
}

func TestEscapePathForDisplayDeterministic(t *testing.T) {
	t.Parallel()

	input := "/path\x01\xff\xc3\xa4/to/file"

	out1 := EscapePathForDisplay(input)
	out2 := EscapePathForDisplay(input)

	require.Equal(t, out1, out2)
}

func TestEscapePathForDisplayDELChar(t *testing.T) {
	t.Parallel()

	input := "/path\x7f/to/file"
	output := EscapePathForDisplay(input)

	require.Contains(t, output, "\\x7f")
}
