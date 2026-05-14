package diff

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRandomTree(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	pathA := filepath.Join(base, "a")
	pathB := filepath.Join(base, "b")

	require.NoError(t, os.MkdirAll(filepath.Join(pathA, "sub"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(pathB, "sub"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(pathA, "file1.txt"), []byte("test content"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(pathB, "file1.txt"), []byte("test content"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(pathA, "sub", "file2.txt"), []byte("other content"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(pathB, "sub", "file2.txt"), []byte("other content"), 0o600))

	require.NoError(t, Paths(pathA, pathB))
}

func TestExpectedFailures(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	pathA := filepath.Join(base, "a")
	pathB := filepath.Join(base, "b")

	require.NoError(t, os.MkdirAll(pathA, 0o750))
	require.NoError(t, os.MkdirAll(pathB, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(pathA, "file.txt"), []byte("left"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(pathB, "file.txt"), []byte("right"), 0o600))

	err := Paths(pathA, pathB)
	require.Error(t, err)
	require.ErrorContains(t, err, "mismatch (-want +got)")
}

func TestSymlinkTargetMismatch(t *testing.T) {
	t.Parallel()
	requireSymlinkSupport(t)

	base := t.TempDir()
	pathA := filepath.Join(base, "a")
	pathB := filepath.Join(base, "b")

	require.NoError(t, os.MkdirAll(pathA, 0o750))
	require.NoError(t, os.MkdirAll(pathB, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(pathA, "target_a.txt"), []byte("a"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(pathA, "target_b.txt"), []byte("b"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(pathB, "target_a.txt"), []byte("a"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(pathB, "target_b.txt"), []byte("b"), 0o600))

	require.NoError(t, os.Symlink("target_a.txt", filepath.Join(pathA, "link.txt")))
	require.NoError(t, os.Symlink("target_b.txt", filepath.Join(pathB, "link.txt")))

	err := Paths(pathA, pathB)
	require.Error(t, err)
	require.ErrorContains(t, err, "LinkTarget")
}

func TestDanglingSymlinkParity(t *testing.T) {
	t.Parallel()
	requireSymlinkSupport(t)

	base := t.TempDir()
	pathA := filepath.Join(base, "a")
	pathB := filepath.Join(base, "b")

	require.NoError(t, os.MkdirAll(pathA, 0o750))
	require.NoError(t, os.MkdirAll(pathB, 0o750))
	require.NoError(t, os.Symlink("missing_target", filepath.Join(pathA, "link.txt")))
	require.NoError(t, os.Symlink("missing_target", filepath.Join(pathB, "link.txt")))

	require.NoError(t, Paths(pathA, pathB))
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
