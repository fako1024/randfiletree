//go:build linux

package diff

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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

func TestPathsWithOptionsACLUnavailableExplicit(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("getfacl"); err == nil {
		t.Skip("getfacl present in environment")
	}

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
	require.Error(t, err)
	require.ErrorIs(t, err, ErrACLToolingUnavailable)
}

func TestPathsWithOptionsACLParity(t *testing.T) {
	t.Parallel()
	requireACLTooling(t)

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")
	require.NoError(t, os.MkdirAll(left, 0o750))
	require.NoError(t, os.MkdirAll(right, 0o750))

	leftFile := filepath.Join(left, "file.txt")
	rightFile := filepath.Join(right, "file.txt")
	require.NoError(t, os.WriteFile(leftFile, []byte("same"), 0o600))
	require.NoError(t, os.WriteFile(rightFile, []byte("same"), 0o600))

	setACL(t, leftFile, "u::rw-,g::r--")
	setACL(t, rightFile, "u::rw-,g::r--")

	requireACEPresent(t, leftFile, "group::r--")
	requireACEPresent(t, rightFile, "group::r--")

	opts := DefaultOptions()
	opts.CompareACLs = true
	require.NoError(t, PathsWithOptions(left, right, opts))

	setACL(t, rightFile, "u::rw-,g::---")
	requireACEPresent(t, rightFile, "group::---")
	err := PathsWithOptions(left, right, opts)
	require.Error(t, err)
	require.ErrorContains(t, err, "ACL mismatch")
}

func requireXAttrSupport(t *testing.T) {
	t.Helper()

	filePath := filepath.Join(t.TempDir(), "probe")
	require.NoError(t, os.WriteFile(filePath, []byte("x"), 0o600))

	if err := unix.Lsetxattr(filePath, "user.probe", []byte("v"), 0); err != nil {
		t.Skipf("xattr not supported in this test environment: %s", err)
	}

	require.NoError(t, unix.Lremovexattr(filePath, "user.probe"))
}

func requireACLTooling(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("setfacl"); err != nil {
		t.Skipf("setfacl unavailable: %v", err)
	}

	if _, err := exec.LookPath("getfacl"); err != nil {
		t.Skipf("getfacl unavailable: %v", err)
	}
}

func setACL(t *testing.T, path, acl string) {
	t.Helper()

	reset := exec.Command("setfacl", "-b", path) // #nosec G204
	if out, err := reset.CombinedOutput(); err != nil {
		t.Skipf("failed to reset ACL on %s: %v (%s)", path, err, string(out))
	}

	cmd := exec.Command("setfacl", "-m", acl, path) // #nosec G204
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("failed to apply ACL on %s: %v (%s)", path, err, string(out))
	}
}

func requireACEPresent(t *testing.T, path, expectedEntry string) {
	t.Helper()

	cmd := exec.Command("getfacl", "--absolute-names", "--omit-header", path) // #nosec G204
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("failed to inspect ACL on %s: %v (%s)", path, err, string(output))
	}

	entries := strings.Split(string(output), "\n")
	for _, entry := range entries {
		if strings.TrimSpace(entry) == expectedEntry {
			return
		}
	}

	t.Skipf("ACL entry %q missing on %s (output: %s)", expectedEntry, path, string(output))
}
