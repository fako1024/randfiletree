//go:build linux

package randfiletree

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/fako1024/randfiletree/internal/aclxattr"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestRunAppliesConfiguredXAttrs(t *testing.T) {
	t.Parallel()
	requireMutationXAttrSupport(t)

	base := filepath.Join(t.TempDir(), "tree")
	g := newMetadataConfiguredGenerator(t, base,
		WithXAttr("user.alpha", []byte("alpha")),
		WithXAttrValueGenerator("user.binary", DataGeneratorFixed([]byte{0x00, 0x7f, 0x80, 0xff})),
	)

	require.NoError(t, g.Run())

	filePath := filepath.Join(base, "file")
	alpha, err := getPathXAttr(filePath, "user.alpha")
	require.NoError(t, err)
	require.Equal(t, []byte("alpha"), alpha)

	binary, err := getPathXAttr(filePath, "user.binary")
	require.NoError(t, err)
	require.Equal(t, []byte{0x00, 0x7f, 0x80, 0xff}, binary)
}

func TestRunTrustedXAttrRequiresOptIn(t *testing.T) {
	t.Parallel()
	requireMutationXAttrSupport(t)

	base := filepath.Join(t.TempDir(), "tree")
	g := newMetadataConfiguredGenerator(t, base,
		WithXAttr("trusted.demo", []byte("x")),
	)

	err := g.Run()
	require.Error(t, err)
	require.ErrorContains(t, err, ErrXAttrNamespaceNotAllowed.Error())
}

func TestRunTrustedXAttrOptInSurfacePermissionError(t *testing.T) {
	t.Parallel()
	requireMutationXAttrSupport(t)

	base := filepath.Join(t.TempDir(), "tree")
	g := newMetadataConfiguredGenerator(t, base,
		WithTrustedXAttrNamespace(true),
		WithXAttr("trusted.demo", []byte("x")),
	)

	err := g.Run()
	if err == nil {
		filePath := filepath.Join(base, "file")
		_, getErr := getPathXAttr(filePath, "trusted.demo")
		require.NoError(t, getErr)
		return
	}

	require.True(t,
		errorIsAny(err, ErrXAttrPermissionDenied, ErrXAttrUnsupported),
		"unexpected trusted namespace failure: %v",
		err,
	)
}

func TestRunACLInvalidEntry(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := newMetadataConfiguredGenerator(t, base,
		WithACL("invalid-entry"),
	)

	err := g.Run()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrACLInvalidEntry)
}

func TestRunACLRequiresDirectoryForDefaultEntries(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := newMetadataConfiguredGenerator(t, base,
		WithACL("default:u::rwx", "default:g::r-x", "default:o::---"),
	)

	err := g.Run()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrACLInvalidEntry)
}

func TestRunACLAppliedWhenSupported(t *testing.T) {
	t.Parallel()
	requireMutationXAttrSupport(t)

	base := filepath.Join(t.TempDir(), "tree")
	g := newMetadataConfiguredGenerator(t, base,
		WithACL("u::rwx", "g::r-x", "o::---", "m::r-x"),
	)

	err := g.Run()
	if err != nil {
		if errors.Is(err, ErrACLUnsupported) || errors.Is(err, ErrACLPermissionDenied) || errors.Is(err, ErrXAttrUnsupported) {
			t.Skipf("ACL application unavailable in environment: %v", err)
		}

		require.NoError(t, err)
	}

	entries, readErr := readPathACL(filepath.Join(base, "file"))
	if readErr != nil && errors.Is(readErr, ErrACLUnsupported) {
		t.Skipf("ACL read unsupported in environment: %v", readErr)
	}
	require.NoError(t, readErr)

	require.Contains(t, entries, "user::rwx")
	require.Contains(t, entries, "group::r-x")
	require.Contains(t, entries, "other::---")
	require.Contains(t, entries, "mask::r-x")
}

func errorIsAny(err error, candidates ...error) bool {
	for _, candidate := range candidates {
		if candidate != nil && errors.Is(err, candidate) {
			return true
		}
	}

	return false
}

func readPathACL(path string) ([]string, error) {
	accessRaw, err := readACLXAttr(path, aclxattr.XAttrAccess)
	if err != nil {
		return nil, err
	}

	defaultRaw, err := readACLXAttr(path, aclxattr.XAttrDefault)
	if err != nil {
		return nil, err
	}

	return aclxattr.FormatTextEntries(accessRaw, defaultRaw), nil
}

func readACLXAttr(path, name string) ([]aclxattr.Entry, error) {
	size, err := unix.Lgetxattr(path, name, nil)
	if err != nil {
		switch {
		case errors.Is(err, unix.ENODATA):
			return nil, nil
		case errors.Is(err, unix.ENOTSUP), errors.Is(err, unix.EOPNOTSUPP), errors.Is(err, unix.EINVAL):
			return nil, ErrACLUnsupported
		default:
			return nil, err
		}
	}

	if size == 0 {
		return nil, nil
	}

	buf := make([]byte, size)
	readSize, err := unix.Lgetxattr(path, name, buf)
	if err != nil {
		return nil, err
	}

	entries, err := aclxattr.Parse(buf[:readSize])
	if err != nil {
		return nil, err
	}

	return entries, nil
}
