//go:build linux

package randfiletree

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestRunCreatesSpecialFIFOs(t *testing.T) {
	t.Parallel()
	requireFIFOSupport(t)

	base := filepath.Join(t.TempDir(), "tree")
	g := newSpecialConfiguredGenerator(t, base, 2,
		WithSpecialFileProbability(1),
		WithSpecialFileTypeGenerator(func(r *rand.Rand) SpecialFileType {
			return SpecialFileTypeFIFO
		}),
	)

	require.NoError(t, g.Run())

	paths := listNonDirectoryEntries(t, base)
	require.Len(t, paths, 2)

	for _, path := range paths {
		info, err := os.Lstat(path)
		require.NoError(t, err)
		require.NotZero(t, info.Mode()&os.ModeNamedPipe)
	}
}

func TestRunCreatesSpecialUnixSockets(t *testing.T) {
	t.Parallel()
	requireUnixSocketPathSupport(t)

	base := filepath.Join(t.TempDir(), "tree")
	g := newSpecialConfiguredGenerator(t, base, 1,
		WithSpecialFileProbability(1),
		WithSpecialFileTypeGenerator(func(r *rand.Rand) SpecialFileType {
			return SpecialFileTypeSocket
		}),
	)

	require.NoError(t, g.Run())

	info, err := os.Lstat(filepath.Join(base, "n00"))
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSocket)
}

func TestRunSpecialCharDeviceCapabilityGated(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := newSpecialConfiguredGenerator(t, base, 1,
		WithSpecialFileProbability(1),
		WithSpecialFileTypeGenerator(func(r *rand.Rand) SpecialFileType {
			return SpecialFileTypeCharDevice
		}),
		WithSpecialDeviceNumbers(1, 7),
	)

	err := g.Run()
	if err != nil {
		if errors.Is(err, ErrSpecialFilePermissionDenied) || errors.Is(err, ErrSpecialFileUnsupported) {
			t.Skipf("special char device creation unavailable in this test environment: %v", err)
		}

		require.NoError(t, err)
	}

	path := filepath.Join(base, "n00")
	info, err := os.Lstat(path)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeDevice)
	require.NotZero(t, info.Mode()&os.ModeCharDevice)

	var stat unix.Stat_t
	require.NoError(t, unix.Lstat(path, &stat))
	require.EqualValues(t, 1, unix.Major(uint64(stat.Rdev)))
	require.EqualValues(t, 7, unix.Minor(uint64(stat.Rdev)))
}

func TestRunSpecialDeviceTypeRequiresDeviceNumbers(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := newSpecialConfiguredGenerator(t, base, 1,
		WithSpecialFileProbability(1),
		WithSpecialFileTypeGenerator(func(r *rand.Rand) SpecialFileType {
			return SpecialFileTypeBlockDevice
		}),
	)

	err := g.Run()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSpecialDeviceNumbersRequired)
}

func newSpecialConfiguredGenerator(t *testing.T, base string, nFiles int, extra ...Option) *Generator {
	t.Helper()

	nameIdx := 0
	g := New(base)
	options := []Option{
		WithRunMode(RunModeReplace),
		WithSeed(11),
		WithDirNameGenerator(func(r *rand.Rand, length int) string { return "dir" }),
		WithDirNameLengthGenerator(NumberGeneratorConstant(3)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(nFiles)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(0)),
		WithFileNameGenerator(func(r *rand.Rand, length int) string {
			name := fmt.Sprintf("n%02d", nameIdx)
			nameIdx++
			return name
		}),
		WithFileNameLengthGenerator(NumberGeneratorConstant(3)),
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

func listNonDirectoryEntries(t *testing.T, path string) []string {
	t.Helper()

	entries, err := os.ReadDir(path)
	require.NoError(t, err)

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		paths = append(paths, filepath.Join(path, entry.Name()))
	}

	sort.Strings(paths)

	return paths
}

func requireFIFOSupport(t *testing.T) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fifo-probe")
	err := unix.Mkfifo(path, 0o600)
	if err != nil {
		if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EINVAL) {
			t.Skipf("fifo creation not supported in this test environment: %v", err)
		}
		t.Skipf("fifo probe failed in this test environment: %v", err)
	}
}

func requireUnixSocketPathSupport(t *testing.T) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "socket-probe")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Skipf("unix socket path creation unavailable in this test environment: %v", err)
	}

	unixListener, ok := listener.(*net.UnixListener)
	require.True(t, ok)
	unixListener.SetUnlinkOnClose(false)
	require.NoError(t, unixListener.Close())
}
