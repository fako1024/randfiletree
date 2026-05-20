//go:build !linux

package randfiletree

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetupFilesystemFeatureScenarioLinuxOnly(t *testing.T) {
	t.Parallel()

	_, err := SetupFilesystemFeatureScenario(t.TempDir(), FilesystemFeatureImmutable)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrFilesystemFeatureScenarioLinuxOnly)
}

func TestProbeFilesystemFeaturesLinuxOnlyStatuses(t *testing.T) {
	t.Parallel()

	statuses, err := ProbeFilesystemFeatures(t.TempDir(), FilesystemFeatureImmutable)
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	require.Equal(t, FilesystemFeatureImmutable, statuses[0].Feature)
	require.Equal(t, FilesystemFeatureAvailabilityUnsupported, statuses[0].Availability)
	require.Contains(t, statuses[0].Diagnostic, ErrFilesystemFeatureScenarioLinuxOnly.Error())
}
