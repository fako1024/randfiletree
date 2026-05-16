package randfiletree

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigureAppliesOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		option Option
		assert func(t *testing.T, g *Generator)
	}{
		{
			name:   "WithRunMode",
			option: WithRunMode(RunModeStrict),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, RunModeStrict, g.runMode)
			},
		},
		{
			name:   "WithSeed",
			option: WithSeed(123),
			assert: func(t *testing.T, g *Generator) {
				reference := rand.New(rand.NewSource(123)) // #nosec G404
				require.Equal(t, reference.Int(), g.rndSrc.Int())
			},
		},
		{
			name: "WithDirNameGenerator",
			option: WithDirNameGenerator(func(r *rand.Rand, length int) string {
				return "dir"
			}),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, "dir", g.dirNameGen(g.rndSrc, 5))
			},
		},
		{
			name:   "WithDirNameLengthGenerator",
			option: WithDirNameLengthGenerator(NumberGeneratorConstant(7)),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, 7, g.dirNameLenGen(g.rndSrc))
			},
		},
		{
			name:   "WithDirModeGenerator",
			option: WithDirModeGenerator(FileModeGeneratorConstant(0o700)),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, uint32(0o700), g.dirModeGen(g.rndSrc))
			},
		},
		{
			name:   "WithFilesPerDirectoryGenerator",
			option: WithFilesPerDirectoryGenerator(NumberGeneratorConstant(2)),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, 2, g.nFilesInDirGen(g.rndSrc))
			},
		},
		{
			name:   "WithDirectoriesPerDirectoryGenerator",
			option: WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(3)),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, 3, g.nDirsInDirGen(g.rndSrc))
			},
		},
		{
			name: "WithFileNameGenerator",
			option: WithFileNameGenerator(func(r *rand.Rand, length int) string {
				return "file"
			}),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, "file", g.fileNameGen(g.rndSrc, 5))
			},
		},
		{
			name:   "WithFileNameLengthGenerator",
			option: WithFileNameLengthGenerator(NumberGeneratorConstant(9)),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, 9, g.fileNameLenGen(g.rndSrc))
			},
		},
		{
			name:   "WithFileModeGenerator",
			option: WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, uint32(0o600), g.fileModeGen(g.rndSrc))
			},
		},
		{
			name:   "WithDataGenerator",
			option: WithDataGenerator(DataGeneratorFixedString("payload")),
			assert: func(t *testing.T, g *Generator) {
				data, err := g.dataGen(g.rndSrc)
				require.NoError(t, err)
				require.Equal(t, "payload", string(data))
			},
		},
		{
			name:   "WithPathDepthGenerator",
			option: WithPathDepthGenerator(NumberGeneratorConstant(4)),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, 4, g.pathDepthGen(g.rndSrc))
			},
		},
		{
			name: "WithSymlinkGenerator",
			option: WithSymlinkGenerator(func(r *rand.Rand) bool {
				return true
			}),
			assert: func(t *testing.T, g *Generator) {
				require.True(t, g.symlinkProbGen(g.rndSrc))
			},
		},
		{
			name: "WithRelativeSymlinkGenerator",
			option: WithRelativeSymlinkGenerator(func(r *rand.Rand) bool {
				return false
			}),
			assert: func(t *testing.T, g *Generator) {
				require.False(t, g.symlinkRelProbGen(g.rndSrc))
			},
		},
		{
			name: "WithHardlinkGenerator",
			option: WithHardlinkGenerator(func(r *rand.Rand) bool {
				return true
			}),
			assert: func(t *testing.T, g *Generator) {
				require.True(t, g.hardlinkProbGen(g.rndSrc))
			},
		},
		{
			name: "WithSymlinkStrategyGenerator",
			option: WithSymlinkStrategyGenerator(func(r *rand.Rand) SymlinkStrategy {
				return SymlinkStrategyDangling
			}),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, SymlinkStrategyDangling, g.symlinkStrategyGen(g.rndSrc))
			},
		},
		{
			name:   "WithSymlinkProbability",
			option: WithSymlinkProbability(1),
			assert: func(t *testing.T, g *Generator) {
				for i := 0; i < 16; i++ {
					require.True(t, g.symlinkProbGen(g.rndSrc))
				}
			},
		},
		{
			name:   "WithRelativeSymlinkProbability",
			option: WithRelativeSymlinkProbability(0),
			assert: func(t *testing.T, g *Generator) {
				for i := 0; i < 16; i++ {
					require.False(t, g.symlinkRelProbGen(g.rndSrc))
				}
			},
		},
		{
			name:   "WithHardlinkProbability",
			option: WithHardlinkProbability(1),
			assert: func(t *testing.T, g *Generator) {
				for i := 0; i < 16; i++ {
					require.True(t, g.hardlinkProbGen(g.rndSrc))
				}
			},
		},
		{
			name: "WithSymlinkStrategyProbabilities",
			option: WithSymlinkStrategyProbabilities(map[SymlinkStrategy]float64{
				SymlinkStrategyCycle: 1,
			}),
			assert: func(t *testing.T, g *Generator) {
				for i := 0; i < 16; i++ {
					require.Equal(t, SymlinkStrategyCycle, g.symlinkStrategyGen(g.rndSrc))
				}
			},
		},
		{
			name:   "WithDirNameLengthRange",
			option: WithDirNameLengthRange(5, 6),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, 5, g.dirNameLenGen(g.rndSrc))
			},
		},
		{
			name:   "WithFileNameLengthRange",
			option: WithFileNameLengthRange(4, 5),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, 4, g.fileNameLenGen(g.rndSrc))
			},
		},
		{
			name:   "WithFilesPerDirectoryRange",
			option: WithFilesPerDirectoryRange(7, 8),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, 7, g.nFilesInDirGen(g.rndSrc))
			},
		},
		{
			name:   "WithDirectoriesPerDirectoryRange",
			option: WithDirectoriesPerDirectoryRange(9, 10),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, 9, g.nDirsInDirGen(g.rndSrc))
			},
		},
		{
			name:   "WithDataLengthRange",
			option: WithDataLengthRange(11, 12),
			assert: func(t *testing.T, g *Generator) {
				data, err := g.dataGen(g.rndSrc)
				require.NoError(t, err)
				require.Len(t, data, 11)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := New(t.TempDir())
			require.NoError(t, g.Configure(tt.option))
			tt.assert(t, g)
		})
	}
}

func TestConfigureRejectsInvalidOptionsWithoutPanic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		option      Option
		errContains string
	}{
		{
			name:        "InvalidRunMode",
			option:      WithRunMode(RunMode(255)),
			errContains: "run mode must be one of append, strict, replace",
		},
		{
			name:        "NilOption",
			option:      nil,
			errContains: "option at index 0 is nil",
		},
		{
			name:        "NilDirNameGenerator",
			option:      WithDirNameGenerator(nil),
			errContains: "directory name generator must not be nil",
		},
		{
			name:        "NilDirNameLengthGenerator",
			option:      WithDirNameLengthGenerator(nil),
			errContains: "directory name length generator must not be nil",
		},
		{
			name:        "NilDirModeGenerator",
			option:      WithDirModeGenerator(nil),
			errContains: "directory mode generator must not be nil",
		},
		{
			name:        "NilFilesPerDirectoryGenerator",
			option:      WithFilesPerDirectoryGenerator(nil),
			errContains: "files-per-directory generator must not be nil",
		},
		{
			name:        "NilDirectoriesPerDirectoryGenerator",
			option:      WithDirectoriesPerDirectoryGenerator(nil),
			errContains: "directories-per-directory generator must not be nil",
		},
		{
			name:        "NilFileNameGenerator",
			option:      WithFileNameGenerator(nil),
			errContains: "file name generator must not be nil",
		},
		{
			name:        "NilFileNameLengthGenerator",
			option:      WithFileNameLengthGenerator(nil),
			errContains: "file name length generator must not be nil",
		},
		{
			name:        "NilFileModeGenerator",
			option:      WithFileModeGenerator(nil),
			errContains: "file mode generator must not be nil",
		},
		{
			name:        "NilDataGenerator",
			option:      WithDataGenerator(nil),
			errContains: "data generator must not be nil",
		},
		{
			name:        "NilPathDepthGenerator",
			option:      WithPathDepthGenerator(nil),
			errContains: "path depth generator must not be nil",
		},
		{
			name:        "NilSymlinkGenerator",
			option:      WithSymlinkGenerator(nil),
			errContains: "symlink generator must not be nil",
		},
		{
			name:        "NilRelativeSymlinkGenerator",
			option:      WithRelativeSymlinkGenerator(nil),
			errContains: "relative symlink generator must not be nil",
		},
		{
			name:        "NilHardlinkGenerator",
			option:      WithHardlinkGenerator(nil),
			errContains: "hardlink generator must not be nil",
		},
		{
			name:        "NilSymlinkStrategyGenerator",
			option:      WithSymlinkStrategyGenerator(nil),
			errContains: "symlink strategy generator must not be nil",
		},
		{
			name:        "NegativeSymlinkProbability",
			option:      WithSymlinkProbability(-0.1),
			errContains: "symlink probability must be within [0, 1]",
		},
		{
			name:        "TooLargeSymlinkProbability",
			option:      WithSymlinkProbability(1.1),
			errContains: "symlink probability must be within [0, 1]",
		},
		{
			name:        "NaNSymlinkProbability",
			option:      WithSymlinkProbability(math.NaN()),
			errContains: "symlink probability must not be NaN",
		},
		{
			name:        "InfiniteRelativeSymlinkProbability",
			option:      WithRelativeSymlinkProbability(math.Inf(1)),
			errContains: "relative symlink probability must be finite",
		},
		{
			name:        "InfiniteHardlinkProbability",
			option:      WithHardlinkProbability(math.Inf(1)),
			errContains: "hardlink probability must be finite",
		},
		{
			name:        "EmptySymlinkStrategyProbabilities",
			option:      WithSymlinkStrategyProbabilities(map[SymlinkStrategy]float64{}),
			errContains: "symlink strategy probabilities must not be empty",
		},
		{
			name: "InvalidSymlinkStrategyProbabilities",
			option: WithSymlinkStrategyProbabilities(map[SymlinkStrategy]float64{
				SymlinkStrategy(255): 1,
			}),
			errContains: "invalid symlink strategy",
		},
		{
			name: "NegativeSymlinkStrategyProbabilities",
			option: WithSymlinkStrategyProbabilities(map[SymlinkStrategy]float64{
				SymlinkStrategyRelative: -0.1,
			}),
			errContains: "must be >= 0",
		},
		{
			name: "ZeroSymlinkStrategyProbabilities",
			option: WithSymlinkStrategyProbabilities(map[SymlinkStrategy]float64{
				SymlinkStrategyRelative: 0,
			}),
			errContains: "sum of symlink strategy probabilities must be > 0",
		},
		{
			name:        "NegativeDirNameLengthRange",
			option:      WithDirNameLengthRange(-1, 1),
			errContains: "directory name length range minimum must be >= 0",
		},
		{
			name:        "InvalidFileNameLengthRange",
			option:      WithFileNameLengthRange(4, 4),
			errContains: "file name length range maximum must be > minimum",
		},
		{
			name:        "InvalidFilesPerDirectoryRange",
			option:      WithFilesPerDirectoryRange(3, 2),
			errContains: "files-per-directory range maximum must be > minimum",
		},
		{
			name:        "InvalidDirectoriesPerDirectoryRange",
			option:      WithDirectoriesPerDirectoryRange(5, 5),
			errContains: "directories-per-directory range maximum must be > minimum",
		},
		{
			name:        "InvalidDataLengthRange",
			option:      WithDataLengthRange(2, 2),
			errContains: "data length range maximum must be > minimum",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := New(t.TempDir())
			beforeFileNameGen := functionPointer(g.fileNameGen)

			var err error
			require.NotPanics(t, func() {
				err = g.Configure(tt.option)
			})

			require.Error(t, err)
			require.ErrorContains(t, err, tt.errContains)
			require.Equal(t, beforeFileNameGen, functionPointer(g.fileNameGen))
		})
	}
}

func TestConfigureAppliesOptionsAtomically(t *testing.T) {
	t.Parallel()

	g := New(t.TempDir())
	beforeFileNameGen := functionPointer(g.fileNameGen)
	beforeRandomSource := g.rndSrc

	err := g.Configure(
		WithFileNameGenerator(func(r *rand.Rand, length int) string {
			return "updated"
		}),
		WithSeed(42),
		func(generator *Generator) error {
			return fmt.Errorf("forced failure")
		},
	)

	require.Error(t, err)
	require.ErrorContains(t, err, "forced failure")
	require.Equal(t, beforeFileNameGen, functionPointer(g.fileNameGen))
	require.Equal(t, beforeRandomSource, g.rndSrc)
}

func TestConfigureRejectsNilGenerator(t *testing.T) {
	t.Parallel()

	var g *Generator
	err := g.Configure(WithSeed(1))
	require.Error(t, err)
	require.ErrorContains(t, err, "nil generator")
}

func TestNewWithOptions(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()

		path := t.TempDir()
		g, err := NewWithOptions(
			path,
			WithDirNameGenerator(StringGeneratorAlphabet(FileNameAlphabetBasic)),
			WithDirNameLengthGenerator(NumberGeneratorConstant(8)),
			WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
			WithPathDepthGenerator(NumberGeneratorConstant(1)),
			WithFilesPerDirectoryGenerator(NumberGeneratorConstant(0)),
			WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(0)),
			WithFileNameGenerator(StringGeneratorAlphabet(FileNameAlphabetBasic)),
			WithFileNameLengthGenerator(NumberGeneratorConstant(8)),
			WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
			WithDataGenerator(DataGeneratorFixedString("")),
			WithSymlinkGenerator(func(r *rand.Rand) bool { return false }),
			WithRelativeSymlinkGenerator(func(r *rand.Rand) bool { return false }),
			WithHardlinkGenerator(func(r *rand.Rand) bool { return false }),
		)
		require.NoError(t, err)
		require.NotNil(t, g)
		require.NoError(t, g.Run())
	})

	t.Run("Failure", func(t *testing.T) {
		t.Parallel()

		g, err := NewWithOptions(t.TempDir(), WithFilesPerDirectoryRange(2, 2))
		require.Nil(t, g)
		require.Error(t, err)
		require.ErrorContains(t, err, "files-per-directory range maximum must be > minimum")
	})
}

func functionPointer(fn interface{}) uintptr {
	if fn == nil {
		return 0
	}

	return reflect.ValueOf(fn).Pointer()
}
