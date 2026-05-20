package randfiletree

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeFilesystemFeaturesDeterministic(t *testing.T) {
	t.Parallel()

	normalized, err := normalizeFilesystemFeatures([]FilesystemFeature{
		FilesystemFeatureReflink,
		FilesystemFeatureImmutable,
		FilesystemFeatureReflink,
		FilesystemFeatureAppendOnly,
	}, false)
	require.NoError(t, err)
	require.Equal(t, []FilesystemFeature{
		FilesystemFeatureImmutable,
		FilesystemFeatureAppendOnly,
		FilesystemFeatureReflink,
	}, normalized)
}

func TestNormalizeFilesystemFeaturesRejectsInvalid(t *testing.T) {
	t.Parallel()

	_, err := normalizeFilesystemFeatures([]FilesystemFeature{FilesystemFeature(255)}, false)
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid filesystem feature")
}

func TestProbeFilesystemFeaturesDefaultsToAllKnown(t *testing.T) {
	t.Parallel()

	statuses, err := ProbeFilesystemFeatures(t.TempDir())
	require.NoError(t, err)

	expected := allFilesystemFeatures()
	require.Len(t, statuses, len(expected))
	for i := range expected {
		require.Equal(t, expected[i], statuses[i].Feature)
		require.NotEmpty(t, string(statuses[i].Availability))
	}
}

func TestSetupFilesystemFeatureScenarioRequiresExplicitOptIn(t *testing.T) {
	t.Parallel()

	_, err := SetupFilesystemFeatureScenario(t.TempDir())
	require.Error(t, err)
	require.ErrorIs(t, err, ErrFilesystemFeatureSelectionEmpty)
}

func TestEnsureFilesystemFeatureAvailabilityIncludesDiagnostics(t *testing.T) {
	t.Parallel()

	err := ensureFilesystemFeatureAvailability("/tmp/example", []FilesystemFeatureStatus{
		{
			Feature:      FilesystemFeatureImmutable,
			Availability: FilesystemFeatureAvailabilityAvailable,
			Diagnostic:   "probe succeeded",
		},
		{
			Feature:      FilesystemFeatureReflink,
			Availability: FilesystemFeatureAvailabilityUnsupported,
			Diagnostic:   "FICLONE returned ENOTSUP",
		},
	})

	require.Error(t, err)
	require.ErrorIs(t, err, ErrFilesystemFeatureScenarioUnavailable)
	require.ErrorContains(t, err, "reflink=unsupported")
	require.ErrorContains(t, err, "FICLONE returned ENOTSUP")
}
