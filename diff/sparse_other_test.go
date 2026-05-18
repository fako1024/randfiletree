//go:build !linux

package diff

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPathsWithOptionsSparsenessUnsupportedOnNonLinux(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")
	require.NoError(t, os.MkdirAll(left, 0o750))
	require.NoError(t, os.MkdirAll(right, 0o750))

	require.NoError(t, os.WriteFile(filepath.Join(left, "file.txt"), []byte("same"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(right, "file.txt"), []byte("same"), 0o600))

	opts := DefaultOptions()
	opts.CompareSparseness = true

	err := PathsWithOptions(left, right, opts)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSparsenessCollectionUnsupported)
}
