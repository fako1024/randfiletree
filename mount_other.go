//go:build !linux

package randfiletree

import "fmt"

// MountTmpfs mounts a tmpfs on the provided target path.
func MountTmpfs(target string, sizeBytes int64) error {
	_ = sizeBytes

	return fmt.Errorf("failed to mount tmpfs on target `%s`: %w", target, ErrMountLinuxOnly)
}

// MountBind creates a bind mount from source to target.
func MountBind(source, target string) error {
	return fmt.Errorf("failed to bind mount source `%s` on target `%s`: %w", source, target, ErrMountLinuxOnly)
}

// Unmount unmounts an existing mount target.
func Unmount(target string) error {
	return fmt.Errorf("failed to unmount target `%s`: %w", target, ErrMountLinuxOnly)
}
