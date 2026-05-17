package diff

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

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

func TestHardlinkTopologyParity(t *testing.T) {
	t.Parallel()
	requireHardlinkSupport(t)

	base := t.TempDir()
	pathA := filepath.Join(base, "a")
	pathB := filepath.Join(base, "b")

	require.NoError(t, os.MkdirAll(pathA, 0o750))
	require.NoError(t, os.MkdirAll(pathB, 0o750))

	aTarget := filepath.Join(pathA, "target.txt")
	bTarget := filepath.Join(pathB, "target.txt")
	require.NoError(t, os.WriteFile(aTarget, []byte("shared-data"), 0o600))
	require.NoError(t, os.WriteFile(bTarget, []byte("shared-data"), 0o600))

	require.NoError(t, os.Link(aTarget, filepath.Join(pathA, "link1.txt")))
	require.NoError(t, os.Link(aTarget, filepath.Join(pathA, "link2.txt")))
	require.NoError(t, os.Link(bTarget, filepath.Join(pathB, "link1.txt")))
	require.NoError(t, os.Link(bTarget, filepath.Join(pathB, "link2.txt")))

	require.NoError(t, Paths(pathA, pathB))
}

func TestHardlinkTopologyMismatch(t *testing.T) {
	t.Parallel()
	requireHardlinkSupport(t)

	base := t.TempDir()
	pathA := filepath.Join(base, "a")
	pathB := filepath.Join(base, "b")

	require.NoError(t, os.MkdirAll(pathA, 0o750))
	require.NoError(t, os.MkdirAll(pathB, 0o750))

	aTarget := filepath.Join(pathA, "target.txt")
	bTarget := filepath.Join(pathB, "target.txt")
	require.NoError(t, os.WriteFile(aTarget, []byte("shared-data"), 0o600))
	require.NoError(t, os.WriteFile(bTarget, []byte("shared-data"), 0o600))

	require.NoError(t, os.Link(aTarget, filepath.Join(pathA, "link.txt")))
	require.NoError(t, os.WriteFile(filepath.Join(pathB, "link.txt"), []byte("shared-data"), 0o600))

	err := Paths(pathA, pathB)
	require.Error(t, err)
	require.ErrorContains(t, err, "hardlink topology mismatch")
}

func TestPathsWithOptionsHashToggle(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	pathA := filepath.Join(base, "a")
	pathB := filepath.Join(base, "b")
	require.NoError(t, os.MkdirAll(pathA, 0o750))
	require.NoError(t, os.MkdirAll(pathB, 0o750))

	fileA := filepath.Join(pathA, "file.txt")
	fileB := filepath.Join(pathB, "file.txt")
	require.NoError(t, os.WriteFile(fileA, []byte("left1"), 0o600))
	require.NoError(t, os.WriteFile(fileB, []byte("right"), 0o600))

	ts := time.Unix(1_700_000_000, 0)
	require.NoError(t, os.Chtimes(fileA, ts, ts))
	require.NoError(t, os.Chtimes(fileB, ts, ts))

	optsNoHash := DefaultOptions()
	optsNoHash.CompareContentHash = false
	require.NoError(t, PathsWithOptions(pathA, pathB, optsNoHash))

	optsHash := DefaultOptions()
	err := PathsWithOptions(pathA, pathB, optsHash)
	require.Error(t, err)
	require.ErrorContains(t, err, "mismatch (-want +got)")
}

func TestPathsWithOptionsTimestampPrecisionToggle(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	pathA := filepath.Join(base, "a")
	pathB := filepath.Join(base, "b")
	require.NoError(t, os.MkdirAll(pathA, 0o750))
	require.NoError(t, os.MkdirAll(pathB, 0o750))

	fileA := filepath.Join(pathA, "file.txt")
	fileB := filepath.Join(pathB, "file.txt")
	require.NoError(t, os.WriteFile(fileA, []byte("same"), 0o600))
	require.NoError(t, os.WriteFile(fileB, []byte("same"), 0o600))

	timeA := time.Unix(1_700_000_000, 100)
	timeB := time.Unix(1_700_000_000, 900)
	require.NoError(t, os.Chtimes(fileA, timeA, timeA))
	require.NoError(t, os.Chtimes(fileB, timeB, timeB))

	infoA, err := os.Stat(fileA)
	require.NoError(t, err)
	infoB, err := os.Stat(fileB)
	require.NoError(t, err)

	if infoA.ModTime().UnixNano() == infoB.ModTime().UnixNano() {
		t.Skip("filesystem does not preserve nanosecond mtime differences")
	}
	require.Equal(t, infoA.ModTime().Unix(), infoB.ModTime().Unix())

	optsSeconds := DefaultOptions()
	optsSeconds.TimestampPrecision = TimestampPrecisionSeconds
	require.NoError(t, PathsWithOptions(pathA, pathB, optsSeconds))

	optsNanoseconds := DefaultOptions()
	optsNanoseconds.TimestampPrecision = TimestampPrecisionNanoseconds
	err = PathsWithOptions(pathA, pathB, optsNanoseconds)
	require.Error(t, err)
	require.ErrorContains(t, err, "mismatch (-want +got)")
}

func TestPathsWithOptionsOwnershipToggle(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	pathA := filepath.Join(base, "a")
	pathB := filepath.Join(base, "b")
	require.NoError(t, os.MkdirAll(pathA, 0o750))
	require.NoError(t, os.MkdirAll(pathB, 0o750))

	require.NoError(t, os.WriteFile(filepath.Join(pathA, "file.txt"), []byte("same"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(pathB, "file.txt"), []byte("same"), 0o600))

	opts := DefaultOptions()
	opts.CompareOwnership = true

	err := PathsWithOptions(pathA, pathB, opts)
	if runtime.GOOS == "linux" {
		require.NoError(t, err)
		return
	}

	require.Error(t, err)
	require.ErrorContains(t, err, "ownership comparison requested but metadata unavailable")
}

func TestPathsWithOptionsHardlinkTopologyToggle(t *testing.T) {
	t.Parallel()
	requireHardlinkSupport(t)

	base := t.TempDir()
	pathA := filepath.Join(base, "a")
	pathB := filepath.Join(base, "b")

	require.NoError(t, os.MkdirAll(pathA, 0o750))
	require.NoError(t, os.MkdirAll(pathB, 0o750))

	aTarget := filepath.Join(pathA, "target.txt")
	bTarget := filepath.Join(pathB, "target.txt")
	require.NoError(t, os.WriteFile(aTarget, []byte("shared-data"), 0o600))
	require.NoError(t, os.WriteFile(bTarget, []byte("shared-data"), 0o600))
	require.NoError(t, os.Link(aTarget, filepath.Join(pathA, "link.txt")))
	require.NoError(t, os.WriteFile(filepath.Join(pathB, "link.txt"), []byte("shared-data"), 0o600))

	ts := time.Unix(1_700_000_000, 0)
	require.NoError(t, os.Chtimes(aTarget, ts, ts))
	require.NoError(t, os.Chtimes(bTarget, ts, ts))
	require.NoError(t, os.Chtimes(filepath.Join(pathB, "link.txt"), ts, ts))

	optsNoHardlink := DefaultOptions()
	optsNoHardlink.CompareHardlinkTopology = false
	require.NoError(t, PathsWithOptions(pathA, pathB, optsNoHardlink))

	err := PathsWithOptions(pathA, pathB, DefaultOptions())
	require.Error(t, err)
	require.ErrorContains(t, err, "hardlink topology mismatch")
}

func TestPathsWithOptionsMetadataHookValidation(t *testing.T) {
	t.Parallel()

	t.Run("XAttrComparatorRequired", func(t *testing.T) {
		t.Parallel()

		opts := DefaultOptions()
		opts.CompareXAttrs = true

		err := PathsWithOptions(t.TempDir(), t.TempDir(), opts)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrXAttrComparatorNil)
	})

	t.Run("ACLComparatorRequired", func(t *testing.T) {
		t.Parallel()

		opts := DefaultOptions()
		opts.CompareACLs = true

		err := PathsWithOptions(t.TempDir(), t.TempDir(), opts)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrACLComparatorNil)
	})
}

func TestPathsWithOptionsXAttrHookDeterministicOrder(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	pathA := filepath.Join(base, "a")
	pathB := filepath.Join(base, "b")
	require.NoError(t, os.MkdirAll(pathA, 0o750))
	require.NoError(t, os.MkdirAll(pathB, 0o750))

	require.NoError(t, os.WriteFile(filepath.Join(pathA, "z.txt"), []byte("same"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(pathB, "z.txt"), []byte("same"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(pathA, "a.txt"), []byte("same"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(pathB, "a.txt"), []byte("same"), 0o600))

	paths := make([]string, 0, 2)
	opts := DefaultOptions()
	opts.CompareXAttrs = true
	opts.XAttrComparator = func(path string, _, _ Node) error {
		paths = append(paths, path)
		return nil
	}

	require.NoError(t, PathsWithOptions(pathA, pathB, opts))
	require.Equal(t, []string{"/a.txt", "/z.txt"}, paths)
}

func TestPathsWithOptionsACLHookMismatch(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	pathA := filepath.Join(base, "a")
	pathB := filepath.Join(base, "b")
	require.NoError(t, os.MkdirAll(pathA, 0o750))
	require.NoError(t, os.MkdirAll(pathB, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(pathA, "file.txt"), []byte("same"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(pathB, "file.txt"), []byte("same"), 0o600))

	opts := DefaultOptions()
	opts.CompareACLs = true
	opts.ACLComparator = func(path string, _, _ Node) error {
		return fmt.Errorf("missing ACL parity")
	}

	err := PathsWithOptions(pathA, pathB, opts)
	require.Error(t, err)
	require.EqualError(t, err, "ACL mismatch for path `/file.txt`: missing ACL parity")
}

func TestPathsCompatibilityWrapper(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	pathA := filepath.Join(base, "a")
	pathB := filepath.Join(base, "b")
	require.NoError(t, os.MkdirAll(pathA, 0o750))
	require.NoError(t, os.MkdirAll(pathB, 0o750))

	fileA := filepath.Join(pathA, "file.txt")
	fileB := filepath.Join(pathB, "file.txt")
	require.NoError(t, os.WriteFile(fileA, []byte("left"), 0o600))
	require.NoError(t, os.WriteFile(fileB, []byte("right"), 0o600))

	ts := time.Unix(1_700_000_000, 0)
	require.NoError(t, os.Chtimes(fileA, ts, ts))
	require.NoError(t, os.Chtimes(fileB, ts, ts))

	errLegacy := Paths(pathA, pathB)
	errDefault := PathsWithOptions(pathA, pathB, DefaultOptions())
	require.Error(t, errLegacy)
	require.Error(t, errDefault)
	require.EqualError(t, errLegacy, errDefault.Error())
}

func TestPathsDeterministicMismatchOutput(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	pathA := filepath.Join(base, "a")
	pathB := filepath.Join(base, "b")
	require.NoError(t, os.MkdirAll(pathA, 0o750))
	require.NoError(t, os.MkdirAll(pathB, 0o750))

	for _, name := range []string{"z.txt", "a.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(pathA, name), []byte("left1"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(pathB, name), []byte("right"), 0o600))
		ts := time.Unix(1_700_000_000, 0)
		require.NoError(t, os.Chtimes(filepath.Join(pathA, name), ts, ts))
		require.NoError(t, os.Chtimes(filepath.Join(pathB, name), ts, ts))
	}

	errA := Paths(pathA, pathB)
	errB := Paths(pathA, pathB)
	require.Error(t, errA)
	require.Error(t, errB)
	require.EqualError(t, errA, errB.Error())
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

func requireHardlinkSupport(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")
	link := filepath.Join(tmpDir, "link.txt")

	require.NoError(t, os.WriteFile(target, []byte("data"), 0o600))
	if err := os.Link(target, link); err != nil {
		t.Skipf("hardlink not supported in this test environment: %s", err)
	}
}
