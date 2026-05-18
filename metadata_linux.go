//go:build linux

package randfiletree

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

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

	if metadata.hasXAttrs {
		if err := applyXAttrs(path, metadata.xattrs); err != nil {
			return err
		}
	}

	if metadata.hasACL {
		if err := applyACL(path, metadata.aclEntries, metadata.aclUseTools); err != nil {
			return err
		}
	}

	return nil
}

func applyXAttrs(path string, xattrs map[string][]byte) error {
	if len(xattrs) == 0 {
		return nil
	}

	names := make([]string, 0, len(xattrs))
	for name := range xattrs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if err := setPathXAttr(path, name, xattrs[name]); err != nil {
			return err
		}
	}

	return nil
}

func applyACL(path string, entries []string, useTools bool) error {
	if !useTools {
		return ErrACLCommandBackendDisabled
	}

	if _, err := exec.LookPath("setfacl"); err != nil {
		return fmt.Errorf("%w: setfacl not found", ErrACLToolingUnavailable)
	}

	baseArgs := []string{"-b", path}
	if err := runACLCommand(baseArgs...); err != nil {
		return err
	}

	if len(entries) == 0 {
		return nil
	}

	args := []string{"-m", strings.Join(entries, ","), path}
	if err := runACLCommand(args...); err != nil {
		return err
	}

	return nil
}

func runACLCommand(args ...string) error {
	cmd := exec.Command("setfacl", args...) // #nosec G204
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	msg := strings.TrimSpace(string(output))
	if msg == "" {
		msg = err.Error()
	}

	switch {
	case strings.Contains(msg, "Operation not supported"):
		return fmt.Errorf("%w: %s", ErrACLUnsupported, msg)
	case strings.Contains(msg, "Operation not permitted"), strings.Contains(msg, "Permission denied"):
		return fmt.Errorf("%w: %s", ErrACLPermissionDenied, msg)
	case strings.Contains(msg, "Invalid argument"), strings.Contains(msg, "Invalid ACL"):
		return fmt.Errorf("%w: %s", ErrACLInvalidEntry, msg)
	default:
		return fmt.Errorf("setfacl %s failed for `%s`: %s", strings.Join(args[:len(args)-1], " "), filepath.Clean(args[len(args)-1]), msg)
	}
}
