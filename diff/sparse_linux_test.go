//go:build linux

package diff

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestPathsWithOptionsSparsenessToggle(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")
	require.NoError(t, os.MkdirAll(left, 0o750))
	require.NoError(t, os.MkdirAll(right, 0o750))

	leftFile := filepath.Join(left, "file.bin")
	rightFile := filepath.Join(right, "file.bin")

	require.NoError(t, createSparseProbeFile(leftFile, 8*1024*1024))
	require.NoError(t, createDenseProbeFile(rightFile, 8*1024*1024))

	leftStat := statAllocatedBlocks(t, leftFile)
	rightStat := statAllocatedBlocks(t, rightFile)
	leftSparse := leftStat.Blocks*512 < leftStat.Size
	rightSparse := rightStat.Blocks*512 < rightStat.Size
	if leftSparse == rightSparse {
		t.Skip("filesystem did not produce different sparseness parity for sparse vs dense probes")
	}

	fixed := time.Unix(1_779_300_000, 0)
	require.NoError(t, os.Chtimes(leftFile, fixed, fixed))
	require.NoError(t, os.Chtimes(rightFile, fixed, fixed))

	opts := DefaultOptions()
	opts.CompareContentHash = false
	opts.CompareSparseness = false
	require.NoError(t, PathsWithOptions(left, right, opts))

	opts.CompareSparseness = true
	err := PathsWithOptions(left, right, opts)
	require.Error(t, err)
	require.ErrorContains(t, err, "mismatch (-want +got)")
}

func TestPathsWithOptionsSparsenessParityMatch(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")
	require.NoError(t, os.MkdirAll(left, 0o750))
	require.NoError(t, os.MkdirAll(right, 0o750))

	leftFile := filepath.Join(left, "file.bin")
	rightFile := filepath.Join(right, "file.bin")

	require.NoError(t, createSparseProbeFile(leftFile, 4*1024*1024))
	require.NoError(t, createSparseProbeFile(rightFile, 4*1024*1024))

	leftStat := statAllocatedBlocks(t, leftFile)
	rightStat := statAllocatedBlocks(t, rightFile)
	leftSparse := leftStat.Blocks*512 < leftStat.Size
	rightSparse := rightStat.Blocks*512 < rightStat.Size
	if !leftSparse || !rightSparse {
		t.Skip("filesystem did not preserve sparse parity for sparse probes")
	}

	fixed := time.Unix(1_779_300_001, 0)
	require.NoError(t, os.Chtimes(leftFile, fixed, fixed))
	require.NoError(t, os.Chtimes(rightFile, fixed, fixed))

	opts := DefaultOptions()
	opts.CompareContentHash = false
	opts.CompareSparseness = true
	require.NoError(t, PathsWithOptions(left, right, opts))
}

func createSparseProbeFile(path string, size int64) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	if _, err := f.WriteAt([]byte("A"), 0); err != nil {
		return err
	}
	if _, err := f.WriteAt([]byte("Z"), size-1); err != nil {
		return err
	}

	return f.Close()
}

func createDenseProbeFile(path string, size int64) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	buf := make([]byte, 64*1024)
	for i := range buf {
		buf[i] = byte(i % 251)
	}

	offset := int64(0)
	for offset < size {
		chunkLen := int(minInt64Test(size-offset, int64(len(buf))))
		nWritten, err := f.Write(buf[:chunkLen])
		if err != nil {
			return err
		}
		if nWritten != chunkLen {
			return os.ErrInvalid
		}
		offset += int64(chunkLen)
	}

	return f.Close()
}

func statAllocatedBlocks(t *testing.T, path string) unix.Stat_t {
	t.Helper()

	var stat unix.Stat_t
	require.NoError(t, unix.Lstat(path, &stat))

	return stat
}

func minInt64Test(a, b int64) int64 {
	if a < b {
		return a
	}

	return b
}
