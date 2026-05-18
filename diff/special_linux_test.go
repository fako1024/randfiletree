//go:build linux

package diff

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestPathsWithOptionsFIFOParity(t *testing.T) {
	t.Parallel()
	requireFIFOSupport(t)

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")
	require.NoError(t, os.MkdirAll(left, 0o750))
	require.NoError(t, os.MkdirAll(right, 0o750))

	leftPath := filepath.Join(left, "fifo")
	rightPath := filepath.Join(right, "fifo")
	require.NoError(t, unix.Mkfifo(leftPath, 0o640))
	require.NoError(t, unix.Mkfifo(rightPath, 0o640))

	fixed := time.Unix(1_779_100_000, 0)
	normalizePathTimes(t, left, fixed)
	normalizePathTimes(t, right, fixed)

	require.NoError(t, PathsWithOptions(left, right, DefaultOptions()))
}

func TestPathsWithOptionsSpecialTypeMismatch(t *testing.T) {
	t.Parallel()
	requireFIFOSupport(t)
	requireUnixSocketPathSupport(t)

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")
	require.NoError(t, os.MkdirAll(left, 0o750))
	require.NoError(t, os.MkdirAll(right, 0o750))

	leftPath := filepath.Join(left, "node")
	rightPath := filepath.Join(right, "node")
	require.NoError(t, unix.Mkfifo(leftPath, 0o640))
	createUnixSocketPath(t, rightPath)

	err := PathsWithOptions(left, right, DefaultOptions())
	require.Error(t, err)
	require.ErrorContains(t, err, "mismatch (-want +got)")
}

func TestPathsWithOptionsDeviceMajorMinorMismatch(t *testing.T) {
	t.Parallel()
	requireDeviceNodeSupport(t)

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")
	require.NoError(t, os.MkdirAll(left, 0o750))
	require.NoError(t, os.MkdirAll(right, 0o750))

	leftPath := filepath.Join(left, "dev")
	rightPath := filepath.Join(right, "dev")
	require.NoError(t, unix.Mknod(leftPath, unix.S_IFCHR|0o640, int(unix.Mkdev(1, 7))))
	require.NoError(t, unix.Mknod(rightPath, unix.S_IFCHR|0o640, int(unix.Mkdev(1, 8))))

	fixed := time.Unix(1_779_200_000, 0)
	normalizePathTimes(t, left, fixed)
	normalizePathTimes(t, right, fixed)

	err := PathsWithOptions(left, right, DefaultOptions())
	require.Error(t, err)
	require.ErrorContains(t, err, "mismatch (-want +got)")
}

func createUnixSocketPath(t *testing.T, path string) {
	t.Helper()

	listener, err := net.Listen("unix", path)
	require.NoError(t, err)

	unixListener, ok := listener.(*net.UnixListener)
	require.True(t, ok)
	unixListener.SetUnlinkOnClose(false)
	require.NoError(t, unixListener.Close())
}

func normalizePathTimes(t *testing.T, root string, ts time.Time) {
	t.Helper()

	require.NoError(t, filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		err := unix.UtimesNanoAt(unix.AT_FDCWD, path, []unix.Timespec{
			unix.NsecToTimespec(ts.UnixNano()),
			unix.NsecToTimespec(ts.UnixNano()),
		}, unix.AT_SYMLINK_NOFOLLOW)
		if err != nil {
			if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) || errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EINVAL) {
				t.Skipf("timestamp normalization unavailable for special inode parity check: %v", err)
			}
			return err
		}

		return nil
	}))
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

func requireDeviceNodeSupport(t *testing.T) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "dev-probe")
	err := unix.Mknod(path, unix.S_IFCHR|0o600, int(unix.Mkdev(1, 7)))
	if err != nil {
		if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
			t.Skipf("device node creation requires elevated privileges in this test environment: %v", err)
		}
		if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EINVAL) {
			t.Skipf("device node creation unsupported in this test environment: %v", err)
		}
		t.Skipf("device node probe failed in this test environment: %v", err)
	}
}
