//go:build !linux

package randfiletree

import "fmt"

// MountTmpfs mounts a tmpfs on the provided target path.
func MountTmpfs(target string, sizeBytes int64) error {
	_ = sizeBytes

	return fmt.Errorf("%w for `%s`", ErrMountLinuxOnly, target)
}

// MountBind creates a bind mount from source to target.
func MountBind(source, target string) error {
	return fmt.Errorf("%w for `%s` -> `%s`", ErrMountLinuxOnly, source, target)
}

// Unmount unmounts an existing mount target.
func Unmount(target string) error {
	return fmt.Errorf("%w for `%s`", ErrMountLinuxOnly, target)
}
