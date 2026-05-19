//go:build linux

package randfiletree

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestMapMountErrorPermissionDenied(t *testing.T) {
	t.Parallel()

	mapped := mapMountError(unix.EPERM)
	require.ErrorIs(t, mapped, ErrMountPermissionDenied)
}

func TestMapMountErrorUnsupported(t *testing.T) {
	t.Parallel()

	mapped := mapMountError(unix.ENOTSUP)
	require.ErrorIs(t, mapped, ErrMountUnsupported)
}

func TestMapMountErrorInvalidArgumentPreserved(t *testing.T) {
	t.Parallel()

	mapped := mapMountError(unix.EINVAL)
	require.ErrorIs(t, mapped, unix.EINVAL)
	require.False(t, errors.Is(mapped, ErrMountUnsupported))
}
