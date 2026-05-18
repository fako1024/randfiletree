//go:build linux

package diff

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/fako1024/randfiletree/internal/aclxattr"
	"golang.org/x/sys/unix"
)

func collectPlatformMetadata(path string, node *Node, opts Options) error {
	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		return fmt.Errorf("failed to stat path `%s`: %w", path, err)
	}

	node.UID = stat.Uid
	node.GID = stat.Gid
	node.HasOwnership = true
	node.Atime = stat.Atim.Sec
	node.AtimeNsec = unix.TimespecToNsec(stat.Atim)
	node.HasAccessTime = true
	node.ModTimeNsec = unix.TimespecToNsec(stat.Mtim)

	if opts.CompareXAttrs {
		xattrs, err := collectXAttrs(path)
		if err != nil {
			return err
		}

		node.XAttrs = xattrs
		node.HasXAttrs = true
	}

	if opts.CompareACLs {
		entries, err := collectACLEntries(path)
		if err != nil {
			return err
		}

		node.ACLEntries = entries
		node.HasACL = true
	}

	return nil
}

func collectXAttrs(path string) ([]XAttr, error) {
	names, err := listXAttrNames(path)
	if err != nil {
		return nil, err
	}

	xattrs := make([]XAttr, 0, len(names))
	for _, name := range names {
		value, err := getXAttr(path, name)
		if err != nil {
			return nil, err
		}

		xattrs = append(xattrs, XAttr{
			Name:  name,
			Value: value,
		})
	}

	return xattrs, nil
}

func listXAttrNames(path string) ([]string, error) {
	size, err := unix.Llistxattr(path, nil)
	if err != nil {
		switch {
		case errors.Is(err, unix.ENOTSUP), errors.Is(err, unix.EOPNOTSUPP), errors.Is(err, unix.EINVAL):
			return nil, fmt.Errorf("%w for `%s`: %w", ErrXAttrCollectionUnsupported, path, err)
		default:
			return nil, fmt.Errorf("failed to list xattrs for `%s`: %w", path, err)
		}
	}

	if size == 0 {
		return nil, nil
	}

	buf := make([]byte, size)
	readSize, err := unix.Llistxattr(path, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to list xattrs for `%s`: %w", path, err)
	}

	if readSize <= 0 {
		return nil, nil
	}

	buf = buf[:readSize]
	parts := strings.Split(string(buf), "\x00")
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}

		names = append(names, part)
	}

	sort.Strings(names)

	return names, nil
}

func getXAttr(path, name string) ([]byte, error) {
	size, err := unix.Lgetxattr(path, name, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read xattr `%s` for `%s`: %w", name, path, err)
	}

	if size == 0 {
		return nil, nil
	}

	buf := make([]byte, size)
	readSize, err := unix.Lgetxattr(path, name, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read xattr `%s` for `%s`: %w", name, path, err)
	}

	if readSize < 0 {
		return nil, nil
	}

	return append([]byte(nil), buf[:readSize]...), nil
}

func collectACLEntries(path string) ([]string, error) {
	access, err := readACLXAttr(path, aclxattr.XAttrAccess)
	if err != nil {
		return nil, err
	}

	defaults, err := readACLXAttr(path, aclxattr.XAttrDefault)
	if err != nil {
		return nil, err
	}

	entries := aclxattr.FormatTextEntries(access, defaults)
	sort.Strings(entries)

	return entries, nil
}

func readACLXAttr(path, name string) ([]aclxattr.Entry, error) {
	data, err := getXAttr(path, name)
	if err != nil {
		switch {
		case errors.Is(err, unix.ENODATA):
			return nil, nil
		case errors.Is(err, unix.ENOTSUP), errors.Is(err, unix.EOPNOTSUPP), errors.Is(err, unix.EINVAL):
			return nil, fmt.Errorf("%w for `%s`: %w", ErrACLCollectionUnsupported, path, err)
		default:
			return nil, fmt.Errorf("failed to collect ACL xattr `%s` for `%s`: %w", name, path, err)
		}
	}

	entries, err := aclxattr.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ACL xattr `%s` for `%s`: %w", name, path, err)
	}

	return entries, nil
}
