//go:build linux

package randfiletree

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestSetupCrossDeviceScenarioProducesDistinctDevices(t *testing.T) {
	scenario := requireCrossDeviceScenario(t)
	require.NotEqual(t, scenario.Primary.DeviceID, scenario.Secondary.DeviceID)
}

func TestCrossDeviceRenameReturnsEXDEV(t *testing.T) {
	scenario := requireCrossDeviceScenario(t)

	sourcePath := filepath.Join(scenario.Primary.Path, "source.txt")
	destinationPath := filepath.Join(scenario.Secondary.Path, "renamed.txt")
	require.NoError(t, os.WriteFile(sourcePath, []byte("payload"), 0o600))

	err := os.Rename(sourcePath, destinationPath)
	require.Error(t, err)
	require.ErrorIs(t, err, unix.EXDEV)

	_, sourceErr := os.Stat(sourcePath)
	require.NoError(t, sourceErr)
	_, destinationErr := os.Stat(destinationPath)
	require.ErrorIs(t, destinationErr, os.ErrNotExist)
}

func TestCrossDeviceHardlinkReturnsEXDEV(t *testing.T) {
	scenario := requireCrossDeviceScenario(t)

	sourcePath := filepath.Join(scenario.Primary.Path, "source.txt")
	hardlinkPath := filepath.Join(scenario.Secondary.Path, "hardlink.txt")
	require.NoError(t, os.WriteFile(sourcePath, []byte("payload"), 0o600))

	err := os.Link(sourcePath, hardlinkPath)
	require.Error(t, err)
	require.ErrorIs(t, err, unix.EXDEV)

	_, sourceErr := os.Stat(sourcePath)
	require.NoError(t, sourceErr)
	_, hardlinkErr := os.Stat(hardlinkPath)
	require.ErrorIs(t, hardlinkErr, os.ErrNotExist)
}

func requireCrossDeviceScenario(t *testing.T) *CrossDeviceScenario {
	t.Helper()

	scenario, err := SetupCrossDeviceScenario(t.TempDir())
	if err != nil {
		if errors.Is(err, ErrCrossDeviceScenarioUnavailable) || errors.Is(err, ErrMountPermissionDenied) || errors.Is(err, ErrMountUnsupported) {
			t.Skipf("cross-device scenario unavailable in this test environment: %v", err)
		}

		require.NoError(t, err)
	}

	if !scenario.IsCrossDevice() {
		t.Skip("cross-device scenario did not produce distinct device IDs")
	}

	t.Cleanup(func() {
		require.NoError(t, scenario.Close())
	})

	return scenario
}
