package randfiletree

import (
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const stressRuntimeCeiling = 30 * time.Second

type performanceScale struct {
	name        string
	seed        int64
	pathDepth   int
	dirsPerDir  int
	filesPerDir int
	dataLen     int
}

var benchmarkScales = []performanceScale{
	{
		name:        "small",
		seed:        11,
		pathDepth:   3,
		dirsPerDir:  2,
		filesPerDir: 8,
		dataLen:     32,
	},
	{
		name:        "medium",
		seed:        13,
		pathDepth:   4,
		dirsPerDir:  3,
		filesPerDir: 12,
		dataLen:     64,
	},
	{
		name:        "large",
		seed:        17,
		pathDepth:   5,
		dirsPerDir:  3,
		filesPerDir: 16,
		dataLen:     128,
	},
}

func BenchmarkGeneratorRunScales(b *testing.B) {
	for _, scale := range benchmarkScales {
		scale := scale
		b.Run(scale.name, func(b *testing.B) {
			basePath := filepath.Join(b.TempDir(), "tree")
			g := New(basePath)

			if err := configurePerformanceGenerator(g, scale); err != nil {
				b.Fatalf("failed to configure benchmark generator: %v", err)
			}

			b.ReportAllocs()

			var totalNodes float64
			var totalRetries float64
			var totalCollisions float64
			var totalPlanNanos float64
			var totalApplyNanos float64
			var totalRunNanos float64

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := g.Configure(WithSeed(scale.seed)); err != nil {
					b.Fatalf("failed to reset benchmark seed: %v", err)
				}

				metrics, err := g.RunWithMetrics(RunOptions{})
				if err != nil {
					b.Fatalf("generator run failed: %v", err)
				}

				totalNodes += float64(metrics.Nodes)
				totalRetries += float64(metrics.Retries)
				totalCollisions += float64(metrics.Collisions)
				totalPlanNanos += float64(metrics.PlanningElapsed.Nanoseconds())
				totalApplyNanos += float64(metrics.ApplyElapsed.Nanoseconds())
				totalRunNanos += float64(metrics.Elapsed.Nanoseconds())
			}
			b.StopTimer()

			if b.N > 0 {
				n := float64(b.N)
				b.ReportMetric(totalNodes/n, "nodes/op")
				b.ReportMetric(totalRetries/n, "retries/op")
				b.ReportMetric(totalCollisions/n, "collisions/op")
				b.ReportMetric(totalPlanNanos/n, "plan-ns/op")
				b.ReportMetric(totalApplyNanos/n, "apply-ns/op")
				b.ReportMetric(totalRunNanos/n, "run-ns/op")
			}
		})
	}
}

func TestStressLargeScaleRunWithinRuntimeCeiling(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "tree")
	g := New(basePath)

	scale := performanceScale{
		name:        "stress",
		seed:        23,
		pathDepth:   5,
		dirsPerDir:  3,
		filesPerDir: 16,
		dataLen:     64,
	}

	require.NoError(t, configurePerformanceGenerator(g, scale))

	metrics, err := g.RunWithMetrics(RunOptions{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, metrics.Nodes, 2000)
	require.Less(t, metrics.Elapsed, stressRuntimeCeiling)
	require.LessOrEqual(t, metrics.Collisions, metrics.Retries)
}

func TestStressCollisionRetriesRemainBounded(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "tree")
	g := New(basePath)
	require.NoError(t, g.Configure(
		WithRunMode(RunModeAppend),
		WithSeed(31),
		WithDirNameGenerator(func(r *rand.Rand, length int) string {
			return "dir"
		}),
		WithDirNameLengthGenerator(NumberGeneratorConstant(3)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(2)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(0)),
		WithFileNameGenerator(func(r *rand.Rand, length int) string {
			return "file"
		}),
		WithFileNameLengthGenerator(NumberGeneratorConstant(4)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
		WithDataGenerator(DataGeneratorFixedString("payload")),
		WithPathDepthGenerator(NumberGeneratorConstant(1)),
		WithSymlinkProbability(0),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
	))

	metrics, err := g.RunWithMetrics(RunOptions{})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPlanPathCollisionExhausted)
	require.Equal(t, maxPlanPathCollisionRetries, metrics.Retries)
	require.Equal(t, maxPlanPathCollisionRetries, metrics.Collisions)
	require.Less(t, metrics.Elapsed, time.Second)
}

func TestStressPlanEntryLimitBoundedAtConfiguredCeiling(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "tree")
	g := New(basePath)

	scale := performanceScale{
		name:        "bounded",
		seed:        41,
		pathDepth:   6,
		dirsPerDir:  4,
		filesPerDir: 32,
		dataLen:     32,
	}

	require.NoError(t, configurePerformanceGenerator(g, scale, WithPlanEntryLimit(200)))

	metrics, err := g.RunWithMetrics(RunOptions{})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrPlanEntryLimitExceeded)
	require.Equal(t, 200, metrics.Nodes)
	require.Zero(t, metrics.AppliedEntries)
	require.Zero(t, metrics.FinalizedDirectories)
	require.Less(t, metrics.Elapsed, time.Second)
}

func configurePerformanceGenerator(g *Generator, scale performanceScale, extra ...Option) error {
	opts := []Option{
		WithRunMode(RunModeReplace),
		WithSeed(scale.seed),
		WithDirNameGenerator(StringGeneratorAlphabet(FileNameAlphabetBasic)),
		WithDirNameLengthGenerator(NumberGeneratorConstant(8)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(scale.filesPerDir)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(scale.dirsPerDir)),
		WithFileNameGenerator(StringGeneratorAlphabet(FileNameAlphabetBasic)),
		WithFileNameLengthGenerator(NumberGeneratorConstant(8)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
		WithDataGenerator(DataGeneratorRandomFixedLen(scale.dataLen)),
		WithPathDepthGenerator(NumberGeneratorConstant(scale.pathDepth)),
		WithSymlinkProbability(0),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
	}

	if len(extra) > 0 {
		opts = append(opts, extra...)
	}

	return g.Configure(opts...)
}
