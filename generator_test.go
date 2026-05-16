package randfiletree

import (
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
	_, err := os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)

	require.NoError(t, g.Run())
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestRunRejectsIncompleteConfigurationWithoutPanic(t *testing.T) {
	t.Parallel()

	g := New(t.TempDir())
	require.NoError(t, g.Configure(WithPathDepthGenerator(NumberGeneratorConstant(1))))

	var err error
	require.NotPanics(t, func() {
		err = g.Run()
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "generator configuration incomplete")
	require.ErrorContains(t, err, "directory name generator")
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
	require.NoError(t, g.Configure(
		WithFileNameGenerator(func(rnd *rand.Rand, length int) string {
			return "rel_link"
		}),
		WithFileNameLengthGenerator(NumberGeneratorConstant(8)),
	))

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
