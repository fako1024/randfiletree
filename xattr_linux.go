//go:build linux

package randfiletree

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

func setPathXAttr(path, name string, value []byte) error {
	normalizedName, err := validateXAttrName(name)
	if err != nil {
		return err
	}

	if err := unix.Lsetxattr(path, normalizedName, value, 0); err != nil {
		switch {
		case errors.Is(err, unix.EPERM), errors.Is(err, unix.EACCES):
			return fmt.Errorf("%w for `%s` name `%s`: %v", ErrXAttrPermissionDenied, path, normalizedName, err)
		case errors.Is(err, unix.ENOTSUP), errors.Is(err, unix.EOPNOTSUPP), errors.Is(err, unix.EINVAL):
			return fmt.Errorf("%w for `%s` name `%s`: %v", ErrXAttrUnsupported, path, normalizedName, err)
		default:
			return fmt.Errorf("failed to set xattr `%s` on `%s`: %w", normalizedName, path, err)
		}
	}

	return nil
}

func removePathXAttr(path, name string) error {
	normalizedName, err := validateXAttrName(name)
	if err != nil {
		return err
	}

	if err := unix.Lremovexattr(path, normalizedName); err != nil {
		switch {
		case errors.Is(err, unix.EPERM), errors.Is(err, unix.EACCES):
			return fmt.Errorf("%w for `%s` name `%s`: %v", ErrXAttrPermissionDenied, path, normalizedName, err)
		case errors.Is(err, unix.ENOTSUP), errors.Is(err, unix.EOPNOTSUPP), errors.Is(err, unix.EINVAL):
			return fmt.Errorf("%w for `%s` name `%s`: %v", ErrXAttrUnsupported, path, normalizedName, err)
		default:
			return fmt.Errorf("failed to remove xattr `%s` on `%s`: %w", normalizedName, path, err)
		}
	}

	return nil
}

func listPathXAttrNames(path string) ([]string, error) {
	size, err := unix.Llistxattr(path, nil)
	if err != nil {
		switch {
		case errors.Is(err, unix.ENOTSUP), errors.Is(err, unix.EOPNOTSUPP):
			return nil, fmt.Errorf("%w for `%s`: %v", ErrXAttrUnsupported, path, err)
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

func getPathXAttr(path, name string) ([]byte, error) {
	normalizedName, err := validateXAttrName(name)
	if err != nil {
		return nil, err
	}

	size, err := unix.Lgetxattr(path, normalizedName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read xattr `%s` on `%s`: %w", normalizedName, path, err)
	}

	if size == 0 {
		return nil, nil
	}

	buf := make([]byte, size)
	readSize, err := unix.Lgetxattr(path, normalizedName, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read xattr `%s` on `%s`: %w", normalizedName, path, err)
	}

	if readSize < 0 {
		return nil, nil
	}

	return append([]byte(nil), buf[:readSize]...), nil
}

func scanPathXAttrSet(path string) (map[string]struct{}, error) {
	names, err := listPathXAttrNames(path)
	if err != nil {
		if errors.Is(err, ErrXAttrUnsupported) {
			return map[string]struct{}{}, nil
		}

		return nil, err
	}

	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}

	return set, nil
}
