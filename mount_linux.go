//go:build linux

package randfiletree

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

const defaultTmpfsMountSizeBytes int64 = 16 << 20

// MountTmpfs mounts a tmpfs on the provided target path.
func MountTmpfs(target string, sizeBytes int64) error {
	if sizeBytes <= 0 {
		sizeBytes = defaultTmpfsMountSizeBytes
	}

	mountData := fmt.Sprintf("size=%d", sizeBytes)
	if err := unix.Mount("tmpfs", target, "tmpfs", 0, mountData); err != nil {
		return fmt.Errorf("failed to mount tmpfs on target `%s`: %w", target, mapMountError(err))
	}

	return nil
}

// MountBind creates a bind mount from source to target.
func MountBind(source, target string) error {
	if err := unix.Mount(source, target, "", uintptr(unix.MS_BIND), ""); err != nil {
		return fmt.Errorf("failed to bind mount source `%s` on target `%s`: %w", source, target, mapMountError(err))
	}

	return nil
}

// Unmount unmounts an existing mount target.
func Unmount(target string) error {
	if err := unix.Unmount(target, 0); err != nil {
		return fmt.Errorf("failed to unmount target `%s`: %w", target, mapMountError(err))
	}

	return nil
}

func mapMountError(err error) error {
	switch {
	case errors.Is(err, unix.EPERM), errors.Is(err, unix.EACCES):
		return ErrMountPermissionDenied
	case errors.Is(err, unix.ENOTSUP), errors.Is(err, unix.EOPNOTSUPP), errors.Is(err, unix.EINVAL), errors.Is(err, unix.ENODEV):
		return ErrMountUnsupported
	default:
		return err
	}
}
