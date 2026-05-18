//go:build linux

package diff

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fako1024/randfiletree/internal/aclxattr"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestPathsWithOptionsXAttrParity(t *testing.T) {
	t.Parallel()
	requireXAttrSupport(t)

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")
	require.NoError(t, os.MkdirAll(left, 0o750))
	require.NoError(t, os.MkdirAll(right, 0o750))

	leftFile := filepath.Join(left, "file.txt")
	rightFile := filepath.Join(right, "file.txt")
	require.NoError(t, os.WriteFile(leftFile, []byte("same"), 0o600))
	require.NoError(t, os.WriteFile(rightFile, []byte("same"), 0o600))

	require.NoError(t, unix.Lsetxattr(leftFile, "user.test", []byte("value"), 0))
	require.NoError(t, unix.Lsetxattr(rightFile, "user.test", []byte("value"), 0))

	opts := DefaultOptions()
	opts.CompareXAttrs = true
	require.NoError(t, PathsWithOptions(left, right, opts))

	require.NoError(t, unix.Lsetxattr(rightFile, "user.test", []byte("different"), 0))
	err := PathsWithOptions(left, right, opts)
	require.Error(t, err)
	require.ErrorContains(t, err, "xattr mismatch")
}

func TestPathsWithOptionsXAttrUnavailableExplicit(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")
	require.NoError(t, os.MkdirAll(left, 0o750))
	require.NoError(t, os.MkdirAll(right, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(left, "file.txt"), []byte("same"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(right, "file.txt"), []byte("same"), 0o600))

	opts := DefaultOptions()
	opts.CompareXAttrs = true

	err := PathsWithOptions(left, right, opts)
	if err == nil {
		return
	}

	require.True(t,
		errors.Is(err, ErrXAttrCollectionUnsupported) || errors.Is(err, ErrXAttrMetadataUnavailable),
		"unexpected xattr failure: %v",
		err,
	)
}

func TestPathsWithOptionsACLUnsupportedExplicit(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")
	require.NoError(t, os.MkdirAll(left, 0o750))
	require.NoError(t, os.MkdirAll(right, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(left, "file.txt"), []byte("same"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(right, "file.txt"), []byte("same"), 0o600))

	opts := DefaultOptions()
	opts.CompareACLs = true

	err := PathsWithOptions(left, right, opts)
	if err == nil {
		return
	}

	require.True(t,
		errors.Is(err, ErrACLCollectionUnsupported) || errors.Is(err, ErrACLMetadataUnavailable),
		"unexpected ACL failure: %v",
		err,
	)
}

func TestPathsWithOptionsACLParity(t *testing.T) {
	t.Parallel()
	requireACLXAttrSupport(t)

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")
	require.NoError(t, os.MkdirAll(left, 0o750))
	require.NoError(t, os.MkdirAll(right, 0o750))

	leftDir := filepath.Join(left, "dir")
	rightDir := filepath.Join(right, "dir")
	require.NoError(t, os.MkdirAll(leftDir, 0o750))
	require.NoError(t, os.MkdirAll(rightDir, 0o750))

	setACL(t, leftDir, "u::rwx", "g::r-x", "o::---", "default:u::rwx", "default:g::r-x", "default:o::---")
	setACL(t, rightDir, "u::rwx", "g::r-x", "o::---", "default:u::rwx", "default:g::r-x", "default:o::---")

	fixed := time.Unix(1_779_000_000, 0)
	require.NoError(t, os.Chtimes(leftDir, fixed, fixed))
	require.NoError(t, os.Chtimes(rightDir, fixed, fixed))

	opts := DefaultOptions()
	opts.CompareACLs = true
	require.NoError(t, PathsWithOptions(left, right, opts))

	setACL(t, rightDir, "u::rwx", "g::r-x", "o::---", "default:u::rwx", "default:g::--x", "default:o::---")
	require.NoError(t, os.Chtimes(leftDir, fixed, fixed))
	require.NoError(t, os.Chtimes(rightDir, fixed, fixed))
	err := PathsWithOptions(left, right, opts)
	require.Error(t, err)
	require.ErrorContains(t, err, "ACL mismatch")
}

func requireXAttrSupport(t *testing.T) {
	t.Helper()

	filePath := filepath.Join(t.TempDir(), "probe")
	require.NoError(t, os.WriteFile(filePath, []byte("x"), 0o600))

	if err := unix.Lsetxattr(filePath, "user.probe", []byte("v"), 0); err != nil {
		if errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EINVAL) {
			t.Skipf("xattr not supported in this test environment: %s", err)
		}
		t.Skipf("xattr probe failed in this test environment: %s", err)
	}

	if err := unix.Lremovexattr(filePath, "user.probe"); err != nil {
		t.Skipf("xattr remove probe failed in this test environment: %s", err)
	}
}

func requireACLXAttrSupport(t *testing.T) {
	t.Helper()
	probeDir := filepath.Join(t.TempDir(), "probe")
	require.NoError(t, os.MkdirAll(probeDir, 0o750))

	setACL(t, probeDir, "u::rwx", "g::r-x", "o::---", "default:u::rwx", "default:g::r-x", "default:o::---")
}

func setACL(t *testing.T, path string, entries ...string) {
	t.Helper()

	accessEntries, defaultEntries, err := aclxattr.ParseTextEntries(entries)
	require.NoError(t, err)

	clearACLXAttr(t, path, aclxattr.XAttrAccess)
	clearACLXAttr(t, path, aclxattr.XAttrDefault)

	if len(accessEntries) > 0 {
		accessPayload, marshalErr := aclxattr.Marshal(accessEntries)
		require.NoError(t, marshalErr)
		if setErr := unix.Lsetxattr(path, aclxattr.XAttrAccess, accessPayload, 0); setErr != nil {
			skipIfACLUnsupported(t, setErr)
			require.NoError(t, setErr)
		}
	}

	if len(defaultEntries) > 0 {
		defaultPayload, marshalErr := aclxattr.Marshal(defaultEntries)
		require.NoError(t, marshalErr)
		if setErr := unix.Lsetxattr(path, aclxattr.XAttrDefault, defaultPayload, 0); setErr != nil {
			skipIfACLUnsupported(t, setErr)
			require.NoError(t, setErr)
		}
	}
}

func clearACLXAttr(t *testing.T, path, name string) {
	t.Helper()

	err := unix.Lremovexattr(path, name)
	if err == nil || errors.Is(err, unix.ENODATA) {
		return
	}

	skipIfACLUnsupported(t, err)
	require.NoError(t, err)
}

func skipIfACLUnsupported(t *testing.T, err error) {
	t.Helper()

	if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
		t.Skipf("ACL xattr unavailable in this test environment: %v", err)
	}
}
