//go:build linux

package randfiletree

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/fako1024/randfiletree/internal/aclxattr"
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
		if err := applyACL(path, mode, metadata.aclEntries); err != nil {
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

func applyACL(path string, mode uint32, rawEntries []string) error {
	accessEntries, defaultEntries, err := resolveACLEntries(mode, rawEntries)
	if err != nil {
		return err
	}

	if len(defaultEntries) > 0 {
		info, statErr := os.Lstat(path)
		if statErr != nil {
			return fmt.Errorf("failed to stat ACL target `%s`: %w", path, statErr)
		}
		if !info.IsDir() {
			return fmt.Errorf("%w: default ACL entries require a directory target `%s`", ErrACLInvalidEntry, path)
		}
	}

	if err := clearACLXAttrs(path); err != nil {
		return err
	}

	if len(rawEntries) == 0 {
		return nil
	}

	if err := setACLXAttr(path, aclxattr.XAttrAccess, accessEntries); err != nil {
		return err
	}

	if len(defaultEntries) > 0 {
		if err := setACLXAttr(path, aclxattr.XAttrDefault, defaultEntries); err != nil {
			return err
		}
	}

	return nil
}

func resolveACLEntries(mode uint32, rawEntries []string) (accessEntries []aclxattr.Entry, defaultEntries []aclxattr.Entry, err error) {
	if len(rawEntries) == 0 {
		return nil, nil, nil
	}

	accessMap := newACLBaseMap(mode)
	var defaultMap map[aclEntryKey]aclxattr.Entry

	for i, rawEntry := range rawEntries {
		isDefault, entry, parseErr := aclxattr.ParseTextEntry(rawEntry)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("%w at index %d: %v", ErrACLInvalidEntry, i, parseErr)
		}

		key := toACLEntryKey(entry)
		if isDefault {
			if defaultMap == nil {
				defaultMap = newACLBaseMap(mode)
			}

			defaultMap[key] = entry
			continue
		}

		accessMap[key] = entry
	}

	accessEntries = canonicalizeACLEntries(accessMap)
	if err := validateACLEntries(accessEntries); err != nil {
		return nil, nil, err
	}

	if defaultMap != nil {
		defaultEntries = canonicalizeACLEntries(defaultMap)
		if err := validateACLEntries(defaultEntries); err != nil {
			return nil, nil, err
		}
	}

	return accessEntries, defaultEntries, nil
}

func clearACLXAttrs(path string) error {
	for _, name := range []string{aclxattr.XAttrAccess, aclxattr.XAttrDefault} {
		if err := removeACLXAttr(path, name); err != nil {
			return err
		}
	}

	return nil
}

func removeACLXAttr(path, name string) error {
	err := unix.Lremovexattr(path, name)
	if err == nil || errors.Is(err, unix.ENODATA) {
		return nil
	}

	switch {
	case errors.Is(err, unix.EPERM), errors.Is(err, unix.EACCES):
		return fmt.Errorf("%w for `%s` ACL `%s`: %v", ErrACLPermissionDenied, path, name, err)
	case errors.Is(err, unix.ENOTSUP), errors.Is(err, unix.EOPNOTSUPP), errors.Is(err, unix.EINVAL):
		return fmt.Errorf("%w for `%s` ACL `%s`: %v", ErrACLUnsupported, path, name, err)
	default:
		return fmt.Errorf("failed to remove ACL `%s` on `%s`: %w", name, path, err)
	}
}

func setACLXAttr(path, name string, entries []aclxattr.Entry) error {
	payload, err := aclxattr.Marshal(entries)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrACLInvalidEntry, err)
	}

	if len(payload) == 0 {
		return nil
	}

	if err := unix.Lsetxattr(path, name, payload, 0); err != nil {
		switch {
		case errors.Is(err, unix.EPERM), errors.Is(err, unix.EACCES):
			return fmt.Errorf("%w for `%s` ACL `%s`: %v", ErrACLPermissionDenied, path, name, err)
		case errors.Is(err, unix.ENOTSUP), errors.Is(err, unix.EOPNOTSUPP), errors.Is(err, unix.EINVAL):
			return fmt.Errorf("%w for `%s` ACL `%s`: %v", ErrACLUnsupported, path, name, err)
		default:
			return fmt.Errorf("failed to set ACL `%s` on `%s`: %w", name, path, err)
		}
	}

	return nil
}

func validateACLEntries(entries []aclxattr.Entry) error {
	if err := aclxattr.Validate(entries); err != nil {
		return fmt.Errorf("%w: %v", ErrACLInvalidEntry, err)
	}

	return nil
}

func canonicalizeACLEntries(raw map[aclEntryKey]aclxattr.Entry) []aclxattr.Entry {
	entries := make([]aclxattr.Entry, 0, len(raw))
	for _, entry := range raw {
		entries = append(entries, entry)
	}

	if hasNamedACLEntries(entries) && !hasMaskACLEntry(entries) {
		mask := deriveACLMask(entries)
		raw[toACLEntryKey(mask)] = mask
		entries = entries[:0]
		for _, entry := range raw {
			entries = append(entries, entry)
		}
	}

	return aclxattr.SortEntries(entries)
}

func hasNamedACLEntries(entries []aclxattr.Entry) bool {
	for _, entry := range entries {
		if entry.Tag == aclxattr.TagUser || entry.Tag == aclxattr.TagGroup {
			return true
		}
	}

	return false
}

func hasMaskACLEntry(entries []aclxattr.Entry) bool {
	for _, entry := range entries {
		if entry.Tag == aclxattr.TagMask {
			return true
		}
	}

	return false
}

func deriveACLMask(entries []aclxattr.Entry) aclxattr.Entry {
	var perm uint16
	for _, entry := range entries {
		switch entry.Tag {
		case aclxattr.TagGroupObj, aclxattr.TagUser, aclxattr.TagGroup:
			perm |= entry.Perm
		}
	}

	return aclxattr.Entry{Tag: aclxattr.TagMask, Perm: perm, ID: aclxattr.UndefinedID}
}

func newACLBaseMap(mode uint32) map[aclEntryKey]aclxattr.Entry {
	base := aclxattr.AccessEntriesFromMode(mode)
	result := make(map[aclEntryKey]aclxattr.Entry, len(base))
	for _, entry := range base {
		result[toACLEntryKey(entry)] = entry
	}

	return result
}

type aclEntryKey struct {
	tag uint16
	id  uint32
}

func toACLEntryKey(entry aclxattr.Entry) aclEntryKey {
	return aclEntryKey{tag: entry.Tag, id: entry.ID}
}
