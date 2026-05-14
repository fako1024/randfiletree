package randfiletree

import (
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultOptions(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "randfiletree")

	g := New(path)
	require.NoError(t, g.RemoveAll())
	require.NoError(t, g.Run())
	n1 := 0
	require.NoError(t, g.Walk(func(path string, info fs.FileInfo, err error) error {
		n1++
		return nil
	}))

	require.NoError(t, g.Run())
	n2 := 0
	require.NoError(t, g.Walk(func(path string, info fs.FileInfo, err error) error {
		n2++
		return nil
	}))

	require.Greater(t, n2, n1)
}

func TestWriteRelSymlinkUsesDirectoryRelativeTarget(t *testing.T) {
	t.Parallel()
	requireSymlinkSupport(t)

	base := t.TempDir()
	dir := filepath.Join(base, "nested")
	require.NoError(t, os.MkdirAll(dir, 0o750))

	target := filepath.Join(base, "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("content"), 0o600))

	g := New(base)
	g.fileNameGen = func(rnd *rand.Rand, length int) string {
		return "rel_link"
	}

	require.NoError(t, g.writeRelSymlink(dir, target))

	linkPath := filepath.Join(dir, "rel_link")
	linkTarget, err := os.Readlink(linkPath)
	require.NoError(t, err)

	expected, err := filepath.Rel(dir, target)
	require.NoError(t, err)
	require.Equal(t, expected, linkTarget)

	resolved := filepath.Clean(filepath.Join(dir, linkTarget))
	require.Equal(t, target, resolved)
}

func TestWriteRelSymlinkRejectsEmptyTarget(t *testing.T) {
	t.Parallel()
	requireSymlinkSupport(t)

	dir := t.TempDir()
	g := New(dir)

	err := g.writeRelSymlink(dir, "")
	require.Error(t, err)
	require.ErrorContains(t, err, "empty symlink target")
}

func requireSymlinkSupport(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")
	link := filepath.Join(tmpDir, "link.txt")

	require.NoError(t, os.WriteFile(target, []byte("data"), 0o600))
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported in this test environment: %s", err)
	}
}
