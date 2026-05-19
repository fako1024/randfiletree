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
		return mapMountError(fmt.Sprintf("mount tmpfs on target `%s`", target), err)
	}

	return nil
}

// MountBind creates a bind mount from source to target.
func MountBind(source, target string) error {
	if err := unix.Mount(source, target, "", uintptr(unix.MS_BIND), ""); err != nil {
		return mapMountError(fmt.Sprintf("bind mount source `%s` on target `%s`", source, target), err)
	}

	return nil
}

// Unmount unmounts an existing mount target.
func Unmount(target string) error {
	if err := unix.Unmount(target, 0); err != nil {
		return mapMountError(fmt.Sprintf("unmount target `%s`", target), err)
	}

	return nil
}

func mapMountError(operation string, err error) error {
	switch {
	case errors.Is(err, unix.EPERM), errors.Is(err, unix.EACCES):
		return fmt.Errorf("failed to %s: %v; %w", operation, err, ErrMountPermissionDenied)
	case errors.Is(err, unix.ENOTSUP), errors.Is(err, unix.EOPNOTSUPP), errors.Is(err, unix.EINVAL), errors.Is(err, unix.ENODEV):
		return fmt.Errorf("failed to %s: %v; %w", operation, err, ErrMountUnsupported)
	default:
		return fmt.Errorf("failed to %s: %w", operation, err)
	}
}
