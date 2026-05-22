package diff

import (
	"path/filepath"
	"testing"

	"github.com/fako1024/randfiletree"
)

func BenchmarkDiffPathsScales(b *testing.B) {
	for _, scale := range benchmarkScales {
		scale := scale
		b.Run(scale.name, func(b *testing.B) {
			treeA := filepath.Join(b.TempDir(), "a")
			treeB := filepath.Join(b.TempDir(), "b")

			if err := buildBenchmarkTree(treeA, scale); err != nil {
				b.Fatalf("failed to build benchmark tree A: %v", err)
			}
			if err := buildBenchmarkTree(treeB, scale); err != nil {
				b.Fatalf("failed to build benchmark tree B: %v", err)
			}

			nodes := approximateNodeCount(scale)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := PathsWithOptions(treeA, treeB, DefaultOptions()); err != nil {
					b.Fatalf("diff failed: %v", err)
				}
			}
			b.StopTimer()

			b.ReportMetric(float64(nodes), "nodes/op")
		})
	}
}

type benchmarkScale struct {
	name        string
	seed        int64
	pathDepth   int
	dirsPerDir  int
	filesPerDir int
	dataLen     int
}

var benchmarkScales = []benchmarkScale{
	{
		name:        "small",
		seed:        101,
		pathDepth:   3,
		dirsPerDir:  2,
		filesPerDir: 8,
		dataLen:     32,
	},
	{
		name:        "medium",
		seed:        103,
		pathDepth:   4,
		dirsPerDir:  3,
		filesPerDir: 12,
		dataLen:     64,
	},
	{
		name:        "large",
		seed:        107,
		pathDepth:   5,
		dirsPerDir:  3,
		filesPerDir: 16,
		dataLen:     128,
	},
}

func buildBenchmarkTree(basePath string, scale benchmarkScale) error {
	g := randfiletree.New(basePath)

	if err := g.Configure(
		randfiletree.WithRunMode(randfiletree.RunModeReplace),
		randfiletree.WithSeed(scale.seed),
		randfiletree.WithDirNameGenerator(randfiletree.StringGeneratorAlphabet(randfiletree.FileNameAlphabetBasic)),
		randfiletree.WithDirNameLengthGenerator(randfiletree.NumberGeneratorConstant(8)),
		randfiletree.WithDirModeGenerator(randfiletree.FileModeGeneratorConstant(0o750)),
		randfiletree.WithFilesPerDirectoryGenerator(randfiletree.NumberGeneratorConstant(scale.filesPerDir)),
		randfiletree.WithDirectoriesPerDirectoryGenerator(randfiletree.NumberGeneratorConstant(scale.dirsPerDir)),
		randfiletree.WithFileNameGenerator(randfiletree.StringGeneratorAlphabet(randfiletree.FileNameAlphabetBasic)),
		randfiletree.WithFileNameLengthGenerator(randfiletree.NumberGeneratorConstant(10)),
		randfiletree.WithFileModeGenerator(randfiletree.FileModeGeneratorConstant(0o600)),
		randfiletree.WithDataGenerator(randfiletree.DataGeneratorRandomFixedLen(scale.dataLen)),
		randfiletree.WithPathDepthGenerator(randfiletree.NumberGeneratorConstant(scale.pathDepth)),
		randfiletree.WithSymlinkProbability(0),
		randfiletree.WithRelativeSymlinkProbability(0),
		randfiletree.WithHardlinkProbability(0),
	); err != nil {
		return err
	}

	return g.Run()
}

func approximateNodeCount(scale benchmarkScale) int {
	dirCount := 0
	width := 1
	for level := 0; level < scale.pathDepth; level++ {
		dirCount += width
		width *= scale.dirsPerDir
	}

	return dirCount + (dirCount * scale.filesPerDir)
}
