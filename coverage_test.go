package randfiletree

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCoveragePairwiseCoversEveryPair(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		dimSizes []int
	}{
		{"trivial-single-dim", []int{4}},
		{"two-dims-balanced", []int{3, 3}},
		{"three-dims", []int{5, 4, 3}},
		{"five-dims", []int{7, 5, 4, 3, 2}},
		{"file-shape", []int{7, 5, 6, 9, 4}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := coveragePairwise(tc.dimSizes)

			for d1 := 0; d1 < len(tc.dimSizes)-1; d1++ {
				for d2 := d1 + 1; d2 < len(tc.dimSizes); d2++ {
					for v1 := 0; v1 < tc.dimSizes[d1]; v1++ {
						for v2 := 0; v2 < tc.dimSizes[d2]; v2++ {
							require.True(t, pairwiseContains(result, d1, d2, v1, v2),
								"missing pair (dim %d=%d, dim %d=%d) in dimSizes=%v",
								d1, v1, d2, v2, tc.dimSizes,
							)
						}
					}
				}
			}
		})
	}
}

func pairwiseContains(cases [][]int, d1, d2, v1, v2 int) bool {
	for _, tc := range cases {
		if tc[d1] == v1 && tc[d2] == v2 {
			return true
		}
	}
	return false
}

func TestCoveragePairwiseDeterministic(t *testing.T) {
	t.Parallel()

	dimSizes := []int{5, 4, 6, 3, 7}
	first := coveragePairwise(dimSizes)
	second := coveragePairwise(dimSizes)

	require.Equal(t, first, second)
}

func TestCoverageSpecDeterministicAcrossCalls(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "spec")
	opts := DeterministicCoverageOptions{Effort: DeterministicCoverageEffortLow}

	leftSpec, leftErr := BuildDeterministicCoverageSpec(base, opts)
	rightSpec, rightErr := BuildDeterministicCoverageSpec(base, opts)
	require.NoError(t, leftErr)
	require.NoError(t, rightErr)
	require.Equal(t, leftSpec, rightSpec)
}

func TestCoveragePlanDeterministicAcrossCalls(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "plan")
	opts := DeterministicCoverageOptions{Effort: DeterministicCoverageEffortLow}

	left, _, leftErr := enumerateCoveragePlan(base, normalizeDeterministicCoverageOptions(opts))
	right, _, rightErr := enumerateCoveragePlan(base, normalizeDeterministicCoverageOptions(opts))
	require.NoError(t, leftErr)
	require.NoError(t, rightErr)

	require.Equal(t, len(left.entries), len(right.entries))
	require.True(t, reflect.DeepEqual(left, right), "coverage plan must be deeply equal between calls")
}

func TestCoverageEffortMonotonicAndDimensionStable(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "monotonic")

	specs := make(map[DeterministicCoverageEffort]DeterministicCoverageSpec)
	for _, effort := range []DeterministicCoverageEffort{
		DeterministicCoverageEffortLow,
		DeterministicCoverageEffortMedium,
		DeterministicCoverageEffortHigh,
		DeterministicCoverageEffortXHigh,
	} {
		spec, err := BuildDeterministicCoverageSpec(base, DeterministicCoverageOptions{Effort: effort})
		require.NoError(t, err)
		specs[effort] = spec
	}

	require.Less(t, specs[DeterministicCoverageEffortLow].PlannedEntries, specs[DeterministicCoverageEffortMedium].PlannedEntries)
	require.Less(t, specs[DeterministicCoverageEffortMedium].PlannedEntries, specs[DeterministicCoverageEffortHigh].PlannedEntries)
	require.Less(t, specs[DeterministicCoverageEffortHigh].PlannedEntries, specs[DeterministicCoverageEffortXHigh].PlannedEntries)

	// Enabled-dimension set is stable across effort levels.
	low := specs[DeterministicCoverageEffortLow].EnabledDimensions
	for _, effort := range []DeterministicCoverageEffort{
		DeterministicCoverageEffortMedium,
		DeterministicCoverageEffortHigh,
		DeterministicCoverageEffortXHigh,
	} {
		require.Equal(t, low, specs[effort].EnabledDimensions,
			"enabled-dimension set must not change with effort (effort=%s)", effort,
		)
	}
}

func TestCoverageLowExercisesEveryEnabledDimension(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "completeness")
	opts := normalizeDeterministicCoverageOptions(DeterministicCoverageOptions{Effort: DeterministicCoverageEffortLow})
	plan, caps, err := enumerateCoveragePlan(base, opts)
	require.NoError(t, err)

	enabled := coverageCapabilitySet(caps)

	dirPaths := collectEntriesByType(plan, plannedEntryTypeDir)
	filePaths := collectEntriesByType(plan, plannedEntryTypeFile)
	symlinkPaths := collectEntriesByType(plan, plannedEntryTypeSymlink)
	hardlinkPaths := collectEntriesByType(plan, plannedEntryTypeHardlink)
	specialPaths := collectEntriesByType(plan, plannedEntryTypeSpecial)

	require.NotEmpty(t, dirPaths, "dirs sub-tree must have planned dirs at low effort")
	require.NotEmpty(t, filePaths, "files sub-tree must have planned files at low effort")

	if enabled[coverageCapabilitySymlinks] {
		require.NotEmpty(t, symlinkPaths, "symlinks sub-tree must have planned symlinks at low effort")
	}

	if enabled[coverageCapabilityHardlinks] {
		require.NotEmpty(t, hardlinkPaths, "hardlinks sub-tree must have planned hardlinks at low effort")
	}

	if enabled[coverageCapabilitySpecialFIFO] || enabled[coverageCapabilitySpecialSocket] ||
		enabled[coverageCapabilitySpecialCharDevice] || enabled[coverageCapabilitySpecialBlockDevice] {
		require.NotEmpty(t, specialPaths, "special sub-tree must have planned entries at low effort")
	}
}

func collectEntriesByType(plan runPlan, typeID plannedEntryType) []string {
	out := make([]string, 0)
	for _, e := range plan.entries {
		if e.typeID == typeID {
			out = append(out, e.path)
		}
	}
	return out
}

func TestCoverageGracefullySkipsWhenLinuxOnlyDisabled(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "no-linux")
	opts := DeterministicCoverageOptions{
		Effort:            DeterministicCoverageEffortLow,
		IncludeLinuxOnly:  false,
		IncludePrivileged: false,
	}

	plan, caps, err := enumerateCoveragePlan(base, opts)
	require.NoError(t, err)

	enabled := coverageCapabilitySet(caps)
	require.False(t, enabled[coverageCapabilitySpecialFIFO], "FIFO should be skipped when Linux-only is disabled")
	require.False(t, enabled[coverageCapabilityXAttrUser], "xattr-user should be skipped when Linux-only is disabled")
	require.False(t, enabled[coverageCapabilityACL], "ACL should be skipped when Linux-only is disabled")

	for _, e := range plan.entries {
		require.NotEqual(t, plannedEntryTypeSpecial, e.typeID, "no special entries when Linux-only disabled")
		require.False(t, e.metadata.hasXAttrs, "no xattr entries when Linux-only disabled")
		require.False(t, e.metadata.hasACL, "no ACL entries when Linux-only disabled")
	}
}

func TestCoverageBuildBuiltInScenarioIgnoresSeed(t *testing.T) {
	t.Parallel()

	names := []string{
		ScenarioNameDeterministicCoverageLow,
		ScenarioNameDeterministicCoverageMedium,
		ScenarioNameDeterministicCoverageHigh,
		ScenarioNameDeterministicCoverageXHigh,
	}

	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			leftSpec, leftErr := BuildBuiltInScenario(name, 1)
			rightSpec, rightErr := BuildBuiltInScenario(name, 9999)
			require.NoError(t, leftErr)
			require.NoError(t, rightErr)

			require.Equal(t, leftSpec.Descriptor.Name, rightSpec.Descriptor.Name)
			require.Equal(t, len(leftSpec.Options), len(rightSpec.Options))
		})
	}
}

func TestCoverageInvalidEffortRejected(t *testing.T) {
	t.Parallel()

	_, err := BuildDeterministicCoverageSpec(t.TempDir(), DeterministicCoverageOptions{Effort: 0})
	require.ErrorIs(t, err, ErrDeterministicCoverageEffortInvalid)

	_, _, err = RunDeterministicCoverage(t.TempDir(), DeterministicCoverageOptions{Effort: 200})
	require.ErrorIs(t, err, ErrDeterministicCoverageEffortInvalid)
}

func TestCoverageEmptyBasePathRejected(t *testing.T) {
	t.Parallel()

	_, err := BuildDeterministicCoverageSpec("", DeterministicCoverageOptions{Effort: DeterministicCoverageEffortLow})
	require.ErrorIs(t, err, ErrDeterministicCoverageBasePathEmpty)

	_, _, err = RunDeterministicCoverage("", DeterministicCoverageOptions{Effort: DeterministicCoverageEffortLow})
	require.ErrorIs(t, err, ErrDeterministicCoverageBasePathEmpty)
}

func TestCoveragePlanFitsWithinDefaultEntryLimit(t *testing.T) {
	t.Parallel()

	for _, effort := range []DeterministicCoverageEffort{
		DeterministicCoverageEffortLow,
		DeterministicCoverageEffortMedium,
		DeterministicCoverageEffortHigh,
		DeterministicCoverageEffortXHigh,
	} {
		effort := effort
		t.Run(effort.String(), func(t *testing.T) {
			spec, err := BuildDeterministicCoverageSpec(t.TempDir(), DeterministicCoverageOptions{Effort: effort})
			require.NoError(t, err)
			require.LessOrEqual(t, spec.PlannedEntries, defaultPlanEntryLimit, "effort=%s entries=%d exceeds default limit", effort, spec.PlannedEntries)
		})
	}
}

func TestCoverageDeterministicNamesStable(t *testing.T) {
	t.Parallel()

	for _, class := range coverageNameClassAll {
		class := class
		t.Run(class.String(), func(t *testing.T) {
			left := coverageDeterministicName(class, coverageNameDefaultLen, 0xDEAD_BEEF)
			right := coverageDeterministicName(class, coverageNameDefaultLen, 0xDEAD_BEEF)
			require.Equal(t, left, right)
			require.NotContains(t, left, "\x00", "names must never contain NUL")
			require.NotContains(t, left, "/", "names must never contain slash")
		})
	}
}
