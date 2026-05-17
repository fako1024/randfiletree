//go:build linux

package randfiletree

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fako1024/randfiletree/diff"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestRunAppliesSpecialModeBits(t *testing.T) {
	t.Parallel()
	requireSpecialModeBitSupport(t)

	base := filepath.Join(t.TempDir(), "tree")
	g := newMetadataConfiguredGenerator(t, base,
		WithDirModeGenerator(FileModeGeneratorConstant(0o3775)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o6754)),
	)

	require.NoError(t, g.Run())

	dirInfo, err := os.Stat(base)
	require.NoError(t, err)
	require.NotZero(t, dirInfo.Mode()&os.ModeSticky)
	require.NotZero(t, dirInfo.Mode()&os.ModeSetgid)

	fileInfo, err := os.Stat(filepath.Join(base, "file"))
	require.NoError(t, err)
	require.NotZero(t, fileInfo.Mode()&os.ModeSetuid)
	require.NotZero(t, fileInfo.Mode()&os.ModeSetgid)
}

func TestRunAppliesNanosecondTimestamps(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	atime := time.Unix(1_700_000_000, 123_456_789)
	mtime := time.Unix(1_700_000_005, 987_654_321)

	g := newMetadataConfiguredGenerator(t, base,
		WithTimestamps(atime, mtime),
	)

	require.NoError(t, g.Run())

	filePath := filepath.Join(base, "file")
	fileAtime, fileMtime, ok := statTimespecNsec(filePath)
	if !ok {
		t.Skip("filesystem does not preserve requested nanosecond timestamp precision")
	}
	require.Equal(t, atime.UnixNano(), fileAtime)
	require.Equal(t, mtime.UnixNano(), fileMtime)

	baseAtime, baseMtime, ok := statTimespecNsec(base)
	if !ok {
		t.Skip("filesystem does not preserve requested nanosecond timestamp precision")
	}
	require.Equal(t, atime.UnixNano(), baseAtime)
	require.Equal(t, mtime.UnixNano(), baseMtime)
}

func TestRunAppliesOwnershipMetadata(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	uid := os.Getuid()
	gid := os.Getgid()

	g := newMetadataConfiguredGenerator(t, base,
		WithOwnership(uid, gid),
	)

	require.NoError(t, g.Run())

	var stat unix.Stat_t
	require.NoError(t, unix.Lstat(filepath.Join(base, "file"), &stat))
	require.EqualValues(t, uid, stat.Uid)
	require.EqualValues(t, gid, stat.Gid)
}

func TestRunOwnershipPermissionErrorIsActionable(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("requires unprivileged test user")
	}

	base := filepath.Join(t.TempDir(), "tree")
	g := newMetadataConfiguredGenerator(t, base,
		WithOwnership(0, 0),
	)

	err := g.Run()
	if err == nil {
		t.Skip("environment permits ownership change to uid=0 gid=0")
	}

	require.ErrorIs(t, err, ErrOwnershipMetadataPermissionDenied)
	require.ErrorContains(t, err, "uid=0 gid=0")
}

func TestRunRejectsIncompleteMetadataConfiguration(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := newMetadataConfiguredGenerator(t, base,
		func(next *Generator) error {
			next.ownershipUIDGen = NumberGeneratorConstant(1)
			return nil
		},
	)

	err := g.Run()
	require.Error(t, err)
	require.ErrorContains(t, err, ErrOwnershipMetadataConfigurationIncomplete.Error())
}

func TestStrictLinuxDiffModeParityWithSpecialBits(t *testing.T) {
	t.Parallel()
	requireSpecialModeBitSupport(t)

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")
	require.NoError(t, os.MkdirAll(left, 0o750))
	require.NoError(t, os.MkdirAll(right, 0o750))

	require.NoError(t, os.WriteFile(filepath.Join(left, "file"), []byte("same"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(right, "file"), []byte("same"), 0o600))

	require.NoError(t, os.Chmod(left, 0o3775))
	require.NoError(t, os.Chmod(right, 0o3775))
	require.NoError(t, os.Chmod(filepath.Join(left, "file"), 0o6754))
	require.NoError(t, os.Chmod(filepath.Join(right, "file"), 0o6754))

	normalizedTime := time.Unix(1_700_000_000, 123_456_789)
	require.NoError(t, normalizeTreeTimes(left, normalizedTime))
	require.NoError(t, normalizeTreeTimes(right, normalizedTime))

	require.NoError(t, diff.PathsWithOptions(left, right, diff.StrictLinuxOptions()))

	require.NoError(t, os.Chmod(filepath.Join(right, "file"), 0o0754))
	require.NoError(t, normalizeTreeTimes(left, normalizedTime))
	require.NoError(t, normalizeTreeTimes(right, normalizedTime))

	err := diff.PathsWithOptions(left, right, diff.StrictLinuxOptions())
	require.Error(t, err)
	require.ErrorContains(t, err, "mismatch (-want +got)")
}

func newMetadataConfiguredGenerator(t *testing.T, base string, extra ...Option) *Generator {
	t.Helper()

	g := New(base)
	options := []Option{
		WithRunMode(RunModeReplace),
		WithSeed(7),
		WithDirNameGenerator(func(r *rand.Rand, length int) string { return "dir" }),
		WithDirNameLengthGenerator(NumberGeneratorConstant(3)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(0)),
		WithFileNameGenerator(func(r *rand.Rand, length int) string { return "file" }),
		WithFileNameLengthGenerator(NumberGeneratorConstant(4)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o640)),
		WithDataGenerator(DataGeneratorFixedString("payload")),
		WithPathDepthGenerator(NumberGeneratorConstant(1)),
		WithSymlinkProbability(0),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
	}

	options = append(options, extra...)

	require.NoError(t, g.Configure(options...))

	return g
}

func requireSpecialModeBitSupport(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "probe-file")
	require.NoError(t, os.WriteFile(filePath, []byte("x"), 0o600))
	require.NoError(t, os.Chmod(filePath, 0o6755))

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		t.Skipf("failed to stat special-mode probe file: %v", err)
	}
	if fileInfo.Mode()&os.ModeSetuid == 0 || fileInfo.Mode()&os.ModeSetgid == 0 {
		t.Skip("filesystem does not preserve setuid/setgid bits on regular files")
	}

	dirPath := filepath.Join(tmpDir, "probe-dir")
	require.NoError(t, os.Mkdir(dirPath, 0o755))
	require.NoError(t, os.Chmod(dirPath, 0o3775))

	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		t.Skipf("failed to stat special-mode probe directory: %v", err)
	}
	if dirInfo.Mode()&os.ModeSticky == 0 || dirInfo.Mode()&os.ModeSetgid == 0 {
		t.Skip("filesystem does not preserve sticky/setgid bits on directories")
	}
}

func statTimespecNsec(path string) (atime int64, mtime int64, ok bool) {
	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		return 0, 0, false
	}

	return unix.TimespecToNsec(stat.Atim), unix.TimespecToNsec(stat.Mtim), true
}

func normalizeTreeTimes(basePath string, ts time.Time) error {
	return filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		return os.Chtimes(path, ts, ts)
	})
}
