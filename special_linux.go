//go:build linux

package randfiletree

import (
	"errors"
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

func createPlannedSpecialFile(path string, fileType SpecialFileType, mode uint32, major, minor int) error {
	if err := validateSpecialFileType(fileType); err != nil {
		return err
	}

	switch fileType {
	case SpecialFileTypeFIFO:
		if err := unix.Mkfifo(path, mode&0o7777); err != nil {
			return mapSpecialFileCreationError(path, fileType, err)
		}
	case SpecialFileTypeSocket:
		if err := createUnixSocketPath(path); err != nil {
			return mapSpecialFileCreationError(path, fileType, err)
		}
	case SpecialFileTypeCharDevice:
		if err := createDeviceNode(path, unix.S_IFCHR, mode, major, minor); err != nil {
			return mapSpecialFileCreationError(path, fileType, err)
		}
	case SpecialFileTypeBlockDevice:
		if err := createDeviceNode(path, unix.S_IFBLK, mode, major, minor); err != nil {
			return mapSpecialFileCreationError(path, fileType, err)
		}
	}

	if err := os.Chmod(path, os.FileMode(mode)); err != nil {
		return fmt.Errorf("failed to chmod special file `%s` (%s) to %#o: %w", path, fileType, mode, err)
	}

	return nil
}

func createUnixSocketPath(path string) error {
	listener, err := net.Listen("unix", path)
	if err != nil {
		return err
	}

	unixListener, ok := listener.(*net.UnixListener)
	if !ok {
		_ = listener.Close()
		return fmt.Errorf("unexpected unix listener type %T", listener)
	}

	unixListener.SetUnlinkOnClose(false)

	if err := unixListener.Close(); err != nil {
		return fmt.Errorf("failed to close unix listener for `%s`: %w", path, err)
	}

	return nil
}

func createDeviceNode(path string, kind uint32, mode uint32, major, minor int) error {
	if major < 0 {
		return fmt.Errorf("special device major must be >= 0, got %d", major)
	}
	if minor < 0 {
		return fmt.Errorf("special device minor must be >= 0, got %d", minor)
	}

	dev := unix.Mkdev(uint32(major), uint32(minor))
	mknodMode := kind | (mode & 0o7777)

	if err := unix.Mknod(path, mknodMode, int(dev)); err != nil {
		return err
	}

	return nil
}

func mapSpecialFileCreationError(path string, fileType SpecialFileType, err error) error {
	switch {
	case errors.Is(err, unix.EPERM), errors.Is(err, unix.EACCES):
		return fmt.Errorf("%w for `%s` (%s): %v", ErrSpecialFilePermissionDenied, path, fileType, err)
	case errors.Is(err, unix.ENOTSUP), errors.Is(err, unix.EOPNOTSUPP), errors.Is(err, unix.EINVAL):
		return fmt.Errorf("%w for `%s` (%s): %v", ErrSpecialFileUnsupported, path, fileType, err)
	default:
		return fmt.Errorf("failed to create special file `%s` (%s): %w", path, fileType, err)
	}
}
