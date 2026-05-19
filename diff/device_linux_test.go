//go:build linux

package diff

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	randfiletree "github.com/fako1024/randfiletree"
	"github.com/stretchr/testify/require"
)

func TestPathsWithOptionsDeviceIDMismatch(t *testing.T) {
	scenario := requireDiffCrossDeviceScenario(t)

	leftFile := filepath.Join(scenario.Primary.Path, "file.txt")
	rightFile := filepath.Join(scenario.Secondary.Path, "file.txt")
	require.NoError(t, os.WriteFile(leftFile, []byte("same"), 0o600))
	require.NoError(t, os.WriteFile(rightFile, []byte("same"), 0o600))

	ts := time.Unix(1_700_000_000, 0)
	require.NoError(t, os.Chtimes(leftFile, ts, ts))
	require.NoError(t, os.Chtimes(rightFile, ts, ts))

	opts := DefaultOptions()
	opts.CompareDeviceIDs = true

	err := PathsWithOptions(scenario.Primary.Path, scenario.Secondary.Path, opts)
	require.Error(t, err)
	require.ErrorContains(t, err, "mismatch (-want +got)")
}

func TestPathsWithOptionsDeviceIDToggle(t *testing.T) {
	scenario := requireDiffCrossDeviceScenario(t)

	leftFile := filepath.Join(scenario.Primary.Path, "file.txt")
	rightFile := filepath.Join(scenario.Secondary.Path, "file.txt")
	require.NoError(t, os.WriteFile(leftFile, []byte("same"), 0o600))
	require.NoError(t, os.WriteFile(rightFile, []byte("same"), 0o600))

	ts := time.Unix(1_700_000_000, 0)
	require.NoError(t, os.Chtimes(leftFile, ts, ts))
	require.NoError(t, os.Chtimes(rightFile, ts, ts))

	optsNoDevice := DefaultOptions()
	optsNoDevice.CompareDeviceIDs = false
	require.NoError(t, PathsWithOptions(scenario.Primary.Path, scenario.Secondary.Path, optsNoDevice))

	optsDevice := DefaultOptions()
	optsDevice.CompareDeviceIDs = true
	err := PathsWithOptions(scenario.Primary.Path, scenario.Secondary.Path, optsDevice)
	require.Error(t, err)
	require.ErrorContains(t, err, "mismatch (-want +got)")
}

func requireDiffCrossDeviceScenario(t *testing.T) *randfiletree.CrossDeviceScenario {
	t.Helper()

	scenario, err := randfiletree.SetupCrossDeviceScenario(t.TempDir())
	if err != nil {
		if errors.Is(err, randfiletree.ErrCrossDeviceScenarioUnavailable) ||
			errors.Is(err, randfiletree.ErrMountPermissionDenied) ||
			errors.Is(err, randfiletree.ErrMountUnsupported) {
			t.Skipf("cross-device diff scenario unavailable in this test environment: %v", err)
		}

		require.NoError(t, err)
	}

	if !scenario.IsCrossDevice() {
		t.Skip("cross-device diff scenario did not produce distinct device IDs")
	}

	t.Cleanup(func() {
		require.NoError(t, scenario.Close())
	})

	return scenario
}
