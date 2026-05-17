//go:build linux

package randfiletree

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func applyMetadata(path string, mode uint32, metadata metadataConfig) error {
	if metadata.hasOwnership {
		if metadata.uid < 0 {
			return fmt.Errorf("uid must be >= 0, got %d", metadata.uid)
		}
		if metadata.gid < 0 {
			return fmt.Errorf("gid must be >= 0, got %d", metadata.gid)
		}

		if err := os.Lchown(path, metadata.uid, metadata.gid); err != nil {
			if errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
				return fmt.Errorf(
					"%w for `%s` (uid=%d gid=%d): %v",
					ErrOwnershipMetadataPermissionDenied,
					path,
					metadata.uid,
					metadata.gid,
					err,
				)
			}

			return fmt.Errorf("failed to set ownership metadata for `%s` to uid=%d gid=%d: %w", path, metadata.uid, metadata.gid, err)
		}
	}

	if err := os.Chmod(path, os.FileMode(mode)); err != nil {
		return fmt.Errorf("failed to chmod `%s` to %#o: %w", path, mode, err)
	}

	if metadata.hasTimestamps {
		ts := []unix.Timespec{
			unix.NsecToTimespec(metadata.atime.UnixNano()),
			unix.NsecToTimespec(metadata.mtime.UnixNano()),
		}

		if err := unix.UtimesNanoAt(unix.AT_FDCWD, path, ts, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return fmt.Errorf(
				"failed to set atime/mtime metadata for `%s` with nanosecond precision: %w",
				path,
				err,
			)
		}
	}

	return nil
}
