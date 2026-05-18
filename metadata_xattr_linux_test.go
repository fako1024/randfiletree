//go:build linux

package randfiletree

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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

func TestRunACLRequiresCommandBackendOptIn(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := newMetadataConfiguredGenerator(t, base,
		WithACL("u::rw-", "g::r--"),
	)

	err := g.Run()
	require.Error(t, err)
	require.ErrorContains(t, err, ErrACLConfigurationIncomplete.Error())
}

func TestRunACLToolingAvailabilityIsExplicit(t *testing.T) {
	t.Parallel()

	if _, err := execLookPath("setfacl"); err == nil {
		t.Skip("setfacl present in environment")
	}

	base := filepath.Join(t.TempDir(), "tree")
	g := newMetadataConfiguredGenerator(t, base,
		WithACL("u::rw-", "g::r--"),
		WithACLCommandBackend(true),
	)

	err := g.Run()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrACLToolingUnavailable)
}

func TestRunACLAppliedWhenToolingAvailable(t *testing.T) {
	t.Parallel()
	requireMutationACLTooling(t)

	base := filepath.Join(t.TempDir(), "tree")
	g := newMetadataConfiguredGenerator(t, base,
		WithACL("u::rw-", "g::r--", "m::rw-", "o::---"),
		WithACLCommandBackend(true),
	)

	err := g.Run()
	if err != nil {
		if errors.Is(err, ErrACLUnsupported) || errors.Is(err, ErrACLPermissionDenied) {
			t.Skipf("ACL application unavailable in environment: %v", err)
		}

		require.NoError(t, err)
	}

	entries, readErr := readACL(filepath.Join(base, "file"))
	if readErr != nil {
		t.Skipf("failed to read ACL in environment: %v", readErr)
	}

	require.True(t, containsACLEntryPrefix(entries, "user::rw-"), "expected user ACL entry, got: %v", entries)
	require.True(t, containsACLEntryPrefix(entries, "group::"), "expected group ACL entry, got: %v", entries)
	require.True(t, containsACLEntryPrefix(entries, "other::---"), "expected other ACL entry, got: %v", entries)
}

func readACL(path string) ([]string, error) {
	if _, err := execLookPath("getfacl"); err != nil {
		return nil, err
	}

	cmd := exec.Command("getfacl", "--absolute-names", "--omit-header", path) // #nosec G204
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	entries := make([]string, 0, len(lines))
	for _, line := range lines {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func containsACLEntryPrefix(entries []string, prefix string) bool {
	for _, entry := range entries {
		if strings.HasPrefix(entry, prefix) {
			return true
		}
	}

	return false
}

func errorIsAny(err error, candidates ...error) bool {
	for _, candidate := range candidates {
		if candidate != nil && errors.Is(err, candidate) {
			return true
		}
	}

	return false
}

func requireMutationACLTooling(t *testing.T) {
	t.Helper()

	if _, err := execLookPath("setfacl"); err != nil {
		t.Skipf("setfacl unavailable: %v", err)
	}

	if _, err := execLookPath("getfacl"); err != nil {
		t.Skipf("getfacl unavailable: %v", err)
	}
}

var execLookPath = exec.LookPath
