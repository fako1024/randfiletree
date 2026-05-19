//go:build linux

package randfiletree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestApplyOperationsRenameAcrossDevicesReturnsEXDEV(t *testing.T) {
	scenario := requireCrossDeviceScenario(t)

	sourcePath := filepath.Join(scenario.BasePath, "left", "source.txt")
	require.NoError(t, os.WriteFile(sourcePath, []byte("payload"), 0o600))

	err := ApplyOperations(scenario.BasePath, []Operation{{
		Kind:        OperationKindRename,
		Path:        "/left/source.txt",
		Destination: "/right/renamed.txt",
	}})
	require.Error(t, err)
	require.ErrorIs(t, err, unix.EXDEV)
}

func TestApplyOperationsHardlinkAcrossDevicesReturnsEXDEV(t *testing.T) {
	scenario := requireCrossDeviceScenario(t)

	sourcePath := filepath.Join(scenario.BasePath, "left", "source.txt")
	require.NoError(t, os.WriteFile(sourcePath, []byte("payload"), 0o600))

	err := ApplyOperations(scenario.BasePath, []Operation{{
		Kind:       OperationKindCreateHardlink,
		Path:       "/right/hardlink.txt",
		SourcePath: "/left/source.txt",
	}})
	require.Error(t, err)
	require.ErrorIs(t, err, unix.EXDEV)
}
