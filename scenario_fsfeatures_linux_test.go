//go:build linux

package randfiletree

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestProbeFilesystemFeaturesExplicitSelection(t *testing.T) {
	t.Parallel()

	statuses, err := ProbeFilesystemFeatures(t.TempDir(), FilesystemFeatureImmutable, FilesystemFeatureReflink)
	require.NoError(t, err)
	require.Len(t, statuses, 2)
	require.Equal(t, FilesystemFeatureImmutable, statuses[0].Feature)
	require.Equal(t, FilesystemFeatureReflink, statuses[1].Feature)

	for _, status := range statuses {
		require.NotEmpty(t, status.Availability)
		require.NotEmpty(t, status.Diagnostic)
	}
}

func TestSetupFilesystemFeatureScenarioImmutableCapabilityAware(t *testing.T) {
	t.Parallel()

	scenario, err := SetupFilesystemFeatureScenario(t.TempDir(), FilesystemFeatureImmutable)
	if err != nil {
		require.True(t, isExpectedFilesystemFeatureUnavailable(err), "unexpected immutable setup failure: %v", err)
		t.Skipf("immutable feature unavailable in this environment: %v", err)
	}

	t.Cleanup(func() {
		require.NoError(t, scenario.Close())
	})

	require.NotEmpty(t, scenario.ImmutablePath)

	writeErr := os.WriteFile(scenario.ImmutablePath, []byte("mutated"), 0o640)
	require.Error(t, writeErr)
	require.ErrorIs(t, writeErr, unix.EPERM)
}

func TestSetupFilesystemFeatureScenarioAppendOnlyCapabilityAware(t *testing.T) {
	t.Parallel()

	scenario, err := SetupFilesystemFeatureScenario(t.TempDir(), FilesystemFeatureAppendOnly)
	if err != nil {
		require.True(t, isExpectedFilesystemFeatureUnavailable(err), "unexpected append-only setup failure: %v", err)
		t.Skipf("append-only feature unavailable in this environment: %v", err)
	}

	t.Cleanup(func() {
		require.NoError(t, scenario.Close())
	})

	require.NotEmpty(t, scenario.AppendOnlyPath)
	require.NoError(t, appendToFile(scenario.AppendOnlyPath, []byte("-tail")))

	truncateErr := os.Truncate(scenario.AppendOnlyPath, 0)
	require.Error(t, truncateErr)
	require.ErrorIs(t, truncateErr, unix.EPERM)
}

func TestSetupFilesystemFeatureScenarioReflinkCapabilityAware(t *testing.T) {
	t.Parallel()

	scenario, err := SetupFilesystemFeatureScenario(t.TempDir(), FilesystemFeatureReflink)
	if err != nil {
		require.True(t, isExpectedFilesystemFeatureUnavailable(err), "unexpected reflink setup failure: %v", err)
		t.Skipf("reflink feature unavailable in this environment: %v", err)
	}

	t.Cleanup(func() {
		require.NoError(t, scenario.Close())
	})

	require.NotEmpty(t, scenario.ReflinkSourcePath)
	require.NotEmpty(t, scenario.ReflinkClonePath)

	sourceData, err := os.ReadFile(scenario.ReflinkSourcePath)
	require.NoError(t, err)

	cloneData, err := os.ReadFile(scenario.ReflinkClonePath)
	require.NoError(t, err)
	require.Equal(t, sourceData, cloneData)

	require.NoError(t, os.WriteFile(scenario.ReflinkClonePath, []byte("clone-only-change"), 0o640))

	updatedSourceData, err := os.ReadFile(scenario.ReflinkSourcePath)
	require.NoError(t, err)
	require.Equal(t, sourceData, updatedSourceData)
}

func TestFilesystemFeatureStatusClassification(t *testing.T) {
	t.Parallel()

	statusPermission := statusFromFilesystemFeatureError(
		FilesystemFeatureImmutable,
		ErrFilesystemFeaturePermissionDenied,
	)
	require.Equal(t, FilesystemFeatureAvailabilityPermissionDenied, statusPermission.Availability)

	statusUnsupported := statusFromFilesystemFeatureError(
		FilesystemFeatureReflink,
		ErrFilesystemFeatureUnsupported,
	)
	require.Equal(t, FilesystemFeatureAvailabilityUnsupported, statusUnsupported.Availability)

	statusOther := statusFromFilesystemFeatureError(
		FilesystemFeatureAppendOnly,
		errors.New("unexpected probe failure"),
	)
	require.Equal(t, FilesystemFeatureAvailabilityUnavailable, statusOther.Availability)
}

func appendToFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	_, err = file.Write(data)

	return err
}

func isExpectedFilesystemFeatureUnavailable(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, ErrFilesystemFeatureScenarioUnavailable) ||
		errors.Is(err, ErrFilesystemFeaturePermissionDenied) ||
		errors.Is(err, ErrFilesystemFeatureUnsupported)
}
