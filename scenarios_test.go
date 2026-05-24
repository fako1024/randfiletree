package randfiletree

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/fako1024/randfiletree/diff"
	"github.com/stretchr/testify/require"
)

func TestBuiltInScenarioCatalogDeterministicOrder(t *testing.T) {
	t.Parallel()

	catalog := BuiltInScenarioCatalog()
	require.Len(t, catalog, len(builtInScenarioOrder))

	for i, expectedName := range builtInScenarioOrder {
		require.Equal(t, expectedName, catalog[i].Name)
		require.NotEmpty(t, catalog[i].Intent)
		require.NotEmpty(t, catalog[i].RequiredCapabilities)
	}
}

func TestBuildBuiltInScenarioRejectsInvalidNames(t *testing.T) {
	t.Parallel()

	_, err := BuildBuiltInScenario("", 1)
	require.ErrorIs(t, err, ErrScenarioNameEmpty)

	_, err = BuildBuiltInScenario("does-not-exist", 1)
	require.ErrorIs(t, err, ErrScenarioUnknown)
}

func TestBuildBuiltInScenarioNormalizesAliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		inputName string
		expected  string
	}{
		{
			name:      "SpaceDelimited",
			inputName: "hardlink heavy",
			expected:  ScenarioNameHardlinkHeavy,
		},
		{
			name:      "UnderscoreDelimited",
			inputName: "symlink_cycle",
			expected:  ScenarioNameSymlinkCycle,
		},
		{
			name:      "SuffixedTreeAlias",
			inputName: "XATTR_ACL_HEAVY_TREE",
			expected:  ScenarioNameXAttrACLHeavy,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spec, err := BuildBuiltInScenario(tt.inputName, 7)
			require.NoError(t, err)
			require.Equal(t, tt.expected, spec.Descriptor.Name)
			require.Equal(t, int64(7), spec.Seed)
			require.NotEmpty(t, spec.Options)
		})
	}
}

func TestBuildBuiltInScenarioDeterministicSparseLargeSeed(t *testing.T) {
	t.Parallel()

	leftPath := filepath.Join(t.TempDir(), "left")
	rightPath := filepath.Join(t.TempDir(), "right")

	require.NoError(t, runBuiltInScenarioForTest(leftPath, ScenarioNameSparseLarge, 42))
	require.NoError(t, runBuiltInScenarioForTest(rightPath, ScenarioNameSparseLarge, 42))

	// The sparse-large scenario does not configure WithTimestamps, so the
	// created files inherit wall-clock mtime from the filesystem at
	// creation time. On slower I/O paths (notably Windows CI) the two
	// runs can straddle a one-second boundary, and diff.DefaultOptions()
	// includes ModTime in its projection. Normalize mtime on both trees
	// to a fixed value so the diff exercises content/structure parity,
	// which is what this test is asserting.
	normalizationTimestamp := time.Unix(1_700_000_000, 0).UTC()
	require.NoError(t, normalizeTreeMTime(leftPath, normalizationTimestamp))
	require.NoError(t, normalizeTreeMTime(rightPath, normalizationTimestamp))

	require.NoError(t, diff.PathsWithOptions(leftPath, rightPath, diff.DefaultOptions()))
}

func TestBuildBuiltInScenarioDeterministicPlanForAllScenarios(t *testing.T) {
	t.Parallel()

	for _, descriptor := range BuiltInScenarioCatalog() {
		descriptor := descriptor

		t.Run(descriptor.Name, func(t *testing.T) {
			basePath := filepath.Join(t.TempDir(), "plan")

			leftSpec, err := BuildBuiltInScenario(descriptor.Name, 99)
			require.NoError(t, err)

			rightSpec, err := BuildBuiltInScenario(descriptor.Name, 99)
			require.NoError(t, err)

			leftGenerator, err := NewWithOptions(basePath, leftSpec.Options...)
			require.NoError(t, err)

			rightGenerator, err := NewWithOptions(basePath, rightSpec.Options...)
			require.NoError(t, err)

			leftPlan, err := leftGenerator.planRun()
			require.NoError(t, err)

			rightPlan, err := rightGenerator.planRun()
			require.NoError(t, err)

			require.Equal(t, leftPlan, rightPlan)
		})
	}
}

func TestBuildBuiltInScenarioSmoke(t *testing.T) {
	t.Parallel()

	for _, descriptor := range BuiltInScenarioCatalog() {
		descriptor := descriptor

		t.Run(descriptor.Name, func(t *testing.T) {
			requireBuiltInScenarioCapabilities(t, descriptor)

			basePath := filepath.Join(t.TempDir(), descriptor.Name)
			err := runBuiltInScenarioForTest(basePath, descriptor.Name, 17)
			if err != nil && isExpectedBuiltInScenarioEnvironmentError(err) {
				t.Skipf("scenario unavailable in this environment: %v", err)
			}

			require.NoError(t, err)

			info, statErr := os.Stat(basePath)
			require.NoError(t, statErr)
			require.True(t, info.IsDir())

			entries, readErr := os.ReadDir(basePath)
			require.NoError(t, readErr)
			require.NotEmpty(t, entries)
		})
	}
}

func runBuiltInScenarioForTest(basePath, scenarioName string, seed int64) error {
	spec, err := BuildBuiltInScenario(scenarioName, seed)
	if err != nil {
		return err
	}

	g, err := NewWithOptions(basePath, spec.Options...)
	if err != nil {
		return err
	}

	return g.Run()
}

func requireBuiltInScenarioCapabilities(t *testing.T, descriptor BuiltInScenarioDescriptor) {
	t.Helper()

	for _, capability := range descriptor.RequiredCapabilities {
		switch capability {
		case BuiltInScenarioCapabilityHardlinkCreation:
			requireMutationHardlinkSupport(t)
		case BuiltInScenarioCapabilitySymlinkCreation:
			requireMutationSymlinkSupport(t)
		case BuiltInScenarioCapabilityLinuxTimestampMetadata:
			if runtime.GOOS != "linux" {
				t.Skip("timestamp metadata scenario requires linux")
			}
		case BuiltInScenarioCapabilityLinuxXAttrMetadata:
			requireMutationXAttrSupport(t)
		case BuiltInScenarioCapabilityLinuxACLMetadata:
			if runtime.GOOS != "linux" {
				t.Skip("ACL metadata scenario requires linux")
			}
		case BuiltInScenarioCapabilityContentPatterns:
			// No external capability check required.
		}
	}
}

func isExpectedBuiltInScenarioEnvironmentError(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, ErrTimestampMetadataUnsupported) ||
		errors.Is(err, ErrXAttrMetadataUnsupported) ||
		errors.Is(err, ErrACLMetadataUnsupported) ||
		errors.Is(err, ErrXAttrUnsupported) ||
		errors.Is(err, ErrACLUnsupported) ||
		errors.Is(err, ErrXAttrPermissionDenied) ||
		errors.Is(err, ErrACLPermissionDenied)
}
