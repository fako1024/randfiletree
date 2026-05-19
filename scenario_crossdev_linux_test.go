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

func TestSetupCrossDeviceScenarioRejectsSymlinkBasePath(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	symlinkPath := filepath.Join(base, "base-link")
	require.NoError(t, os.MkdirAll(target, 0o750))
	require.NoError(t, os.Symlink(target, symlinkPath))

	_, err := SetupCrossDeviceScenario(symlinkPath)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrBasePathSymlink)
}

func TestSetupCrossDeviceScenarioRejectsSymlinkParentPath(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	symlinkParent := filepath.Join(base, "parent-link")
	require.NoError(t, os.MkdirAll(target, 0o750))
	require.NoError(t, os.Symlink(target, symlinkParent))

	_, err := SetupCrossDeviceScenario(filepath.Join(symlinkParent, "child"))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrBasePathSymlink)
}

func TestCrossDeviceScenarioCloseKeepsMountStateOnUnmountFailure(t *testing.T) {
	scenario := &CrossDeviceScenario{
		Secondary: DeviceRoot{Path: t.TempDir()},

		secondaryMounted: true,
	}

	err := scenario.Close()
	require.Error(t, err)
	require.True(t, scenario.secondaryMounted)
}

func TestCrossDeviceScenarioCloseKeepsBindSourceStateOnCleanupFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires unprivileged test user")
	}

	parent := t.TempDir()
	bindSource := filepath.Join(parent, "bind-source")
	require.NoError(t, os.MkdirAll(bindSource, 0o700))
	require.NoError(t, os.Chmod(parent, 0o500))
	t.Cleanup(func() {
		require.NoError(t, os.Chmod(parent, 0o700))
		require.NoError(t, os.RemoveAll(bindSource))
	})

	scenario := &CrossDeviceScenario{
		bindSourcePath: bindSource,
	}

	err := scenario.Close()
	require.Error(t, err)
	require.Equal(t, bindSource, scenario.bindSourcePath)
}

func TestCrossDeviceScenarioCloseIsRetryableOnUnmountFailure(t *testing.T) {
	scenario := requireCrossDeviceScenario(t)

	require.NoError(t, os.MkdirAll(filepath.Join(scenario.Secondary.Path, "busy"), 0o750))
	held, err := os.Open(filepath.Join(scenario.Secondary.Path, "busy"))
	require.NoError(t, err)
	t.Cleanup(func() {
		if held != nil {
			_ = held.Close()
		}
	})

	err = scenario.Close()
	if err == nil {
		require.NoError(t, held.Close())
		held = nil
		t.Skip("unmount did not fail while a directory handle was open")
	}

	require.True(t, scenario.secondaryMounted || scenario.bindSourcePath != "")
	require.NoError(t, held.Close())
	held = nil

	require.NoError(t, scenario.Close())
}

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
