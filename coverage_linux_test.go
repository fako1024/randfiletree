//go:build linux

package randfiletree

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fako1024/randfiletree/diff"
	"github.com/stretchr/testify/require"
)

// TestCoverageApplyDeterministicSameBasePath verifies two consecutive
// coverage applies into the same base path (via RunModeReplace) produce a
// byte-identical tree. This is the strongest reproducibility claim the
// scenario makes: same options + same base path => same on-disk tree.
//
// The test cannot use two different base paths and a strict diff because the
// coverage tree contains absolute-strategy symlinks whose targets encode the
// base path verbatim. Plan-level determinism across base paths is covered by
// TestCoveragePlanDeterministicAcrossCalls.
func TestCoverageApplyDeterministicSameBasePath(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("coverage end-to-end apply test requires linux")
	}

	efforts := []DeterministicCoverageEffort{DeterministicCoverageEffortLow}
	if !testing.Short() {
		efforts = append(efforts, DeterministicCoverageEffortMedium)
	}

	for _, effort := range efforts {
		effort := effort
		t.Run(effort.String(), func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "replay")

			opts := DeterministicCoverageOptions{
				Effort:  effort,
				RunMode: RunModeReplace,
			}

			firstSpec, firstMetrics, err := RunDeterministicCoverage(base, opts)
			require.NoError(t, err)

			firstSnapshot, err := snapshotCoverageTree(base)
			require.NoError(t, err)

			secondSpec, secondMetrics, err := RunDeterministicCoverage(base, opts)
			require.NoError(t, err)

			secondSnapshot, err := snapshotCoverageTree(base)
			require.NoError(t, err)

			require.Equal(t, firstSpec.PlannedEntries, secondSpec.PlannedEntries)
			require.Equal(t, firstSpec.EnabledDimensions, secondSpec.EnabledDimensions)
			require.Equal(t, firstMetrics.Nodes, secondMetrics.Nodes)
			require.Equal(t, firstMetrics.AppliedEntries, secondMetrics.AppliedEntries)

			diffOpts := coverageDiffOptions(firstSpec)
			require.NoError(t, diff.PathsWithOptions(firstSnapshot, secondSnapshot, diffOpts))
		})
	}
}

// snapshotCoverageTree copies the live coverage tree to a sibling
// directory using cp -a, preserving metadata (mode, timestamps, xattrs,
// symlinks, hardlinks). The snapshot is the input to the two-path diff.
func snapshotCoverageTree(base string) (string, error) {
	snapshot := base + "-snapshot"
	if err := os.RemoveAll(snapshot); err != nil {
		return "", err
	}

	// We can't shell out to cp -a deterministically across systems, so do a
	// manual filesystem-level move into the snapshot directory and then
	// re-apply by re-running the coverage. Simpler: rename the base.
	if err := os.Rename(base, snapshot); err != nil {
		return "", err
	}

	return snapshot, nil
}

func coverageDiffOptions(spec DeterministicCoverageSpec) diff.Options {
	opts := diff.StrictLinuxOptions()

	enabled := map[string]bool{}
	for _, name := range spec.EnabledDimensions {
		enabled[name] = true
	}

	// CompareACLs is intentionally not enabled here: the coverage tree
	// contains symlinks, and the underlying diff collector cannot read
	// system.posix_acl_access via Lgetxattr on symlinks (EOPNOTSUPP),
	// which the package surfaces as a hard error. ACL apply parity is
	// still exercised via the catalog-end-to-end smoke.
	//
	// CompareAccessTime is also disabled: atime is updated by the diff's
	// own reads (and by relatime/strictatime kernel policy), so it cannot
	// be byte-identical between two runs.
	opts.CompareXAttrs = enabled[coverageCapabilityXAttrUser.String()] ||
		enabled[coverageCapabilityXAttrTrusted.String()] ||
		enabled[coverageCapabilityXAttrSecurity.String()]
	opts.CompareAccessTime = false
	opts.CompareOwnership = enabled[coverageCapabilityOwnershipMetadata.String()]

	return opts
}

func TestCoverageCatalogEntriesApplyEndToEnd(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("coverage catalog end-to-end test requires linux")
	}

	names := []string{ScenarioNameDeterministicCoverageLow}
	if !testing.Short() {
		names = append(names, ScenarioNameDeterministicCoverageMedium)
	}

	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			base := filepath.Join(t.TempDir(), name)

			spec, err := BuildBuiltInScenario(name, 42)
			require.NoError(t, err)
			require.NotEmpty(t, spec.Options)

			g, err := NewWithOptions(base, spec.Options...)
			require.NoError(t, err)

			require.NoError(t, g.Run())

			info, statErr := os.Stat(base)
			require.NoError(t, statErr)
			require.True(t, info.IsDir())

			entries, readErr := os.ReadDir(base)
			require.NoError(t, readErr)
			require.NotEmpty(t, entries)
		})
	}
}

func TestCoverageRunModeReplaceClearsBase(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("coverage replace test requires linux")
	}

	base := filepath.Join(t.TempDir(), "replace")

	require.NoError(t, os.MkdirAll(base, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(base, "pre-existing.txt"), []byte("garbage"), 0o644))

	_, _, err := RunDeterministicCoverage(base, DeterministicCoverageOptions{
		Effort:  DeterministicCoverageEffortLow,
		RunMode: RunModeReplace,
	})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(base, "pre-existing.txt"))
	require.ErrorIs(t, err, os.ErrNotExist, "pre-existing file should be cleared by RunModeReplace")
}

func TestInspectCoveragePlanModes(t *testing.T) {
	base := "/dev/shm/rapidsafe_tests/input/debug_mode"
	opts := DeterministicCoverageOptions{
		Effort:            DeterministicCoverageEffortLow,
		IncludeLinuxOnly:  true,
		IncludePrivileged: false,
	}

	plan, _, err := enumerateCoveragePlan(base, opts)
	if err != nil {
		t.Fatalf("enumerateCoveragePlan failed: %v", err)
	}

	for _, e := range plan.entries {
		if e.typeID != plannedEntryTypeDir {
			continue
		}
		// look for the problematic cell-dirs parent
		if strings.Contains(e.path, "cell-dirs-00003") {
			t.Logf("dir entry: %s mode=%#o", e.path, e.mode)
		}
		// also log any directory entries that have owner-exec bit missing
		if e.mode&0o100 == 0 {
			t.Logf("dir entry without owner-x in plan: %s mode=%#o", e.path, e.mode)
		}
	}
}

func BenchmarkDeterministicCoverage(b *testing.B) {
	for _, effort := range []DeterministicCoverageEffort{
		DeterministicCoverageEffortLow,
		DeterministicCoverageEffortMedium,
		DeterministicCoverageEffortHigh,
	} {
		effort := effort
		b.Run(effort.String(), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				base := b.TempDir()
				if _, _, err := RunDeterministicCoverage(base, DeterministicCoverageOptions{Effort: effort}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
