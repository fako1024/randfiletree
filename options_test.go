package randfiletree

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"
	"time"

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
			name:   "WithContentPattern",
			option: WithContentPattern(ContentPatternSparseHoles),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, ContentPatternSparseHoles, g.contentPatternGen(g.rndSrc))
			},
		},
		{
			name: "WithContentPatternGenerator",
			option: WithContentPatternGenerator(func(r *rand.Rand) ContentPattern {
				return ContentPatternPartialRangeOverwrite
			}),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, ContentPatternPartialRangeOverwrite, g.contentPatternGen(g.rndSrc))
			},
		},
		{
			name: "WithContentPatternProbabilities",
			option: WithContentPatternProbabilities(map[ContentPattern]float64{
				ContentPatternRepeatedBlocks: 1,
			}),
			assert: func(t *testing.T, g *Generator) {
				for i := 0; i < 16; i++ {
					require.Equal(t, ContentPatternRepeatedBlocks, g.contentPatternGen(g.rndSrc))
				}
			},
		},
		{
			name:   "WithContentLogicalSize",
			option: WithContentLogicalSize(512),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, 512, g.contentLogicalSizeGen(g.rndSrc))
			},
		},
		{
			name:   "WithContentLogicalSizeGenerator",
			option: WithContentLogicalSizeGenerator(NumberGeneratorConstant(1024)),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, 1024, g.contentLogicalSizeGen(g.rndSrc))
			},
		},
		{
			name:   "WithContentLogicalSizeRange",
			option: WithContentLogicalSizeRange(2048, 2049),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, 2048, g.contentLogicalSizeGen(g.rndSrc))
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
			name:   "WithOwnershipGenerators",
			option: WithOwnershipGenerators(NumberGeneratorConstant(101), NumberGeneratorConstant(202)),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, 101, g.ownershipUIDGen(g.rndSrc))
				require.Equal(t, 202, g.ownershipGIDGen(g.rndSrc))
			},
		},
		{
			name:   "WithOwnership",
			option: WithOwnership(303, 404),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, 303, g.ownershipUIDGen(g.rndSrc))
				require.Equal(t, 404, g.ownershipGIDGen(g.rndSrc))
			},
		},
		{
			name: "WithTimestampGenerators",
			option: WithTimestampGenerators(
				TimestampGeneratorConstant(time.Unix(1_700_000_000, 123)),
				TimestampGeneratorConstant(time.Unix(1_700_000_001, 456)),
			),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, time.Unix(1_700_000_000, 123), g.atimeGen(g.rndSrc))
				require.Equal(t, time.Unix(1_700_000_001, 456), g.mtimeGen(g.rndSrc))
			},
		},
		{
			name: "WithTimestamps",
			option: WithTimestamps(
				time.Unix(1_700_000_010, 111),
				time.Unix(1_700_000_020, 222),
			),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, time.Unix(1_700_000_010, 111), g.atimeGen(g.rndSrc))
				require.Equal(t, time.Unix(1_700_000_020, 222), g.mtimeGen(g.rndSrc))
			},
		},
		{
			name:   "WithXAttr",
			option: WithXAttr("user.test", []byte("value")),
			assert: func(t *testing.T, g *Generator) {
				metadata, err := g.resolveMetadata(g.rndSrc)
				require.NoError(t, err)
				require.True(t, metadata.hasXAttrs)
				require.Equal(t, []byte("value"), metadata.xattrs["user.test"])
			},
		},
		{
			name: "WithXAttrsFixed",
			option: WithXAttrsFixed(map[string][]byte{
				"user.b": []byte("B"),
				"user.a": []byte("A"),
			}),
			assert: func(t *testing.T, g *Generator) {
				names := make([]string, 0, len(g.xattrValueGens))
				for _, cfg := range g.xattrValueGens {
					names = append(names, cfg.name)
				}
				require.Equal(t, []string{"user.a", "user.b"}, names)

				metadata, err := g.resolveMetadata(g.rndSrc)
				require.NoError(t, err)
				require.Equal(t, []byte("A"), metadata.xattrs["user.a"])
				require.Equal(t, []byte("B"), metadata.xattrs["user.b"])
			},
		},
		{
			name:   "WithTrustedXAttrNamespace",
			option: WithTrustedXAttrNamespace(true),
			assert: func(t *testing.T, g *Generator) {
				require.True(t, g.xattrAllowTrustedNamespace)
			},
		},
		{
			name:   "WithSecurityXAttrNamespace",
			option: WithSecurityXAttrNamespace(true),
			assert: func(t *testing.T, g *Generator) {
				require.True(t, g.xattrAllowSecurityNamespace)
			},
		},
		{
			name:   "WithACL",
			option: WithACL("g::r--", "u::rw-", "u::rw-"),
			assert: func(t *testing.T, g *Generator) {
				metadata, err := g.resolveMetadata(g.rndSrc)
				require.NoError(t, err)
				require.True(t, metadata.hasACL)
				require.Equal(t, []string{"g::r--", "u::rw-"}, metadata.aclEntries)
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
			name: "WithSpecialFileGenerator",
			option: WithSpecialFileGenerator(func(r *rand.Rand) bool {
				return true
			}),
			assert: func(t *testing.T, g *Generator) {
				require.True(t, g.specialFileProbGen(g.rndSrc))
			},
		},
		{
			name: "WithSpecialFileTypeGenerator",
			option: WithSpecialFileTypeGenerator(func(r *rand.Rand) SpecialFileType {
				return SpecialFileTypeSocket
			}),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, SpecialFileTypeSocket, g.specialFileTypeGen(g.rndSrc))
			},
		},
		{
			name: "WithSpecialFileTypeProbabilities",
			option: WithSpecialFileTypeProbabilities(map[SpecialFileType]float64{
				SpecialFileTypeFIFO: 1,
			}),
			assert: func(t *testing.T, g *Generator) {
				for i := 0; i < 16; i++ {
					require.Equal(t, SpecialFileTypeFIFO, g.specialFileTypeGen(g.rndSrc))
				}
			},
		},
		{
			name:   "WithSpecialDeviceNumberGenerators",
			option: WithSpecialDeviceNumberGenerators(NumberGeneratorConstant(7), NumberGeneratorConstant(9)),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, 7, g.specialDeviceMajorGen(g.rndSrc))
				require.Equal(t, 9, g.specialDeviceMinorGen(g.rndSrc))
			},
		},
		{
			name:   "WithSpecialDeviceNumbers",
			option: WithSpecialDeviceNumbers(11, 13),
			assert: func(t *testing.T, g *Generator) {
				require.Equal(t, 11, g.specialDeviceMajorGen(g.rndSrc))
				require.Equal(t, 13, g.specialDeviceMinorGen(g.rndSrc))
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
			name:   "WithSpecialFileProbability",
			option: WithSpecialFileProbability(1),
			assert: func(t *testing.T, g *Generator) {
				for i := 0; i < 16; i++ {
					require.True(t, g.specialFileProbGen(g.rndSrc))
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
			name:        "InvalidContentPattern",
			option:      WithContentPattern(ContentPattern(255)),
			errContains: "invalid content pattern",
		},
		{
			name:        "NilContentPatternGenerator",
			option:      WithContentPatternGenerator(nil),
			errContains: "content pattern generator must not be nil",
		},
		{
			name:        "EmptyContentPatternProbabilities",
			option:      WithContentPatternProbabilities(map[ContentPattern]float64{}),
			errContains: "content pattern probabilities must not be empty",
		},
		{
			name: "InvalidContentPatternProbabilities",
			option: WithContentPatternProbabilities(map[ContentPattern]float64{
				ContentPattern(255): 1,
			}),
			errContains: "invalid content pattern",
		},
		{
			name: "NegativeContentPatternProbabilities",
			option: WithContentPatternProbabilities(map[ContentPattern]float64{
				ContentPatternDenseRandom: -0.1,
			}),
			errContains: "must be >= 0",
		},
		{
			name: "ZeroContentPatternProbabilities",
			option: WithContentPatternProbabilities(map[ContentPattern]float64{
				ContentPatternDenseRandom: 0,
			}),
			errContains: "sum of content pattern probabilities must be > 0",
		},
		{
			name: "NaNContentPatternProbabilities",
			option: WithContentPatternProbabilities(map[ContentPattern]float64{
				ContentPatternDenseRandom: math.NaN(),
			}),
			errContains: "must not be NaN",
		},
		{
			name: "InfiniteContentPatternProbabilities",
			option: WithContentPatternProbabilities(map[ContentPattern]float64{
				ContentPatternDenseRandom: math.Inf(1),
			}),
			errContains: "must be finite",
		},
		{
			name:        "NilContentLogicalSizeGenerator",
			option:      WithContentLogicalSizeGenerator(nil),
			errContains: "content logical size generator must not be nil",
		},
		{
			name:        "NegativeContentLogicalSize",
			option:      WithContentLogicalSize(-1),
			errContains: "content logical size must be >= 0",
		},
		{
			name:        "InvalidContentLogicalSizeRange",
			option:      WithContentLogicalSizeRange(2, 2),
			errContains: "content logical size range maximum must be > minimum",
		},
		{
			name:        "NilPathDepthGenerator",
			option:      WithPathDepthGenerator(nil),
			errContains: "path depth generator must not be nil",
		},
		{
			name:        "NilOwnershipUIDGenerator",
			option:      WithOwnershipGenerators(nil, NumberGeneratorConstant(1)),
			errContains: "ownership uid generator must not be nil",
		},
		{
			name:        "NilOwnershipGIDGenerator",
			option:      WithOwnershipGenerators(NumberGeneratorConstant(1), nil),
			errContains: "ownership gid generator must not be nil",
		},
		{
			name:        "NegativeOwnershipUID",
			option:      WithOwnership(-1, 1),
			errContains: "ownership uid must be >= 0",
		},
		{
			name:        "NegativeOwnershipGID",
			option:      WithOwnership(1, -1),
			errContains: "ownership gid must be >= 0",
		},
		{
			name:        "NilAtimeGenerator",
			option:      WithTimestampGenerators(nil, TimestampGeneratorConstant(time.Unix(1, 1))),
			errContains: "atime generator must not be nil",
		},
		{
			name:        "NilMtimeGenerator",
			option:      WithTimestampGenerators(TimestampGeneratorConstant(time.Unix(1, 1)), nil),
			errContains: "mtime generator must not be nil",
		},
		{
			name:        "InvalidXAttrName",
			option:      WithXAttrValueGenerator("invalid", DataGeneratorFixedString("x")),
			errContains: "xattr name must include namespace prefix",
		},
		{
			name:        "NilXAttrValueGenerator",
			option:      WithXAttrValueGenerator("user.test", nil),
			errContains: "xattr value generator must not be nil",
		},
		{
			name:        "NilACLGenerator",
			option:      WithACLGenerator(nil),
			errContains: "ACL generator must not be nil",
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
			name:        "NilSpecialFileGenerator",
			option:      WithSpecialFileGenerator(nil),
			errContains: "special file generator must not be nil",
		},
		{
			name:        "NilSpecialFileTypeGenerator",
			option:      WithSpecialFileTypeGenerator(nil),
			errContains: "special file type generator must not be nil",
		},
		{
			name:        "NilSpecialDeviceMajorGenerator",
			option:      WithSpecialDeviceNumberGenerators(nil, NumberGeneratorConstant(1)),
			errContains: "special device major generator must not be nil",
		},
		{
			name:        "NilSpecialDeviceMinorGenerator",
			option:      WithSpecialDeviceNumberGenerators(NumberGeneratorConstant(1), nil),
			errContains: "special device minor generator must not be nil",
		},
		{
			name:        "NegativeSpecialDeviceMajor",
			option:      WithSpecialDeviceNumbers(-1, 1),
			errContains: "special device major must be >= 0",
		},
		{
			name:        "NegativeSpecialDeviceMinor",
			option:      WithSpecialDeviceNumbers(1, -1),
			errContains: "special device minor must be >= 0",
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
			name:        "TooLargeSpecialFileProbability",
			option:      WithSpecialFileProbability(1.1),
			errContains: "special file probability must be within [0, 1]",
		},
		{
			name:        "NaNSpecialFileProbability",
			option:      WithSpecialFileProbability(math.NaN()),
			errContains: "special file probability must not be NaN",
		},
		{
			name:        "EmptySpecialFileTypeProbabilities",
			option:      WithSpecialFileTypeProbabilities(map[SpecialFileType]float64{}),
			errContains: "special file type probabilities must not be empty",
		},
		{
			name: "InvalidSpecialFileTypeProbabilities",
			option: WithSpecialFileTypeProbabilities(map[SpecialFileType]float64{
				SpecialFileType(255): 1,
			}),
			errContains: "invalid special file type",
		},
		{
			name: "NegativeSpecialFileTypeProbabilities",
			option: WithSpecialFileTypeProbabilities(map[SpecialFileType]float64{
				SpecialFileTypeFIFO: -0.1,
			}),
			errContains: "must be >= 0",
		},
		{
			name: "ZeroSpecialFileTypeProbabilities",
			option: WithSpecialFileTypeProbabilities(map[SpecialFileType]float64{
				SpecialFileTypeFIFO: 0,
			}),
			errContains: "sum of special file type probabilities must be > 0",
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
	require.ErrorIs(t, err, ErrNilGenerator)
}

func TestValidateSymlinkStrategyProbabilitiesSentinelErrors(t *testing.T) {
	t.Parallel()

	err := validateSymlinkStrategyProbabilities(map[SymlinkStrategy]float64{})
	require.ErrorIs(t, err, ErrSymlinkStrategyProbabilitiesEmpty)

	err = validateSymlinkStrategyProbabilities(map[SymlinkStrategy]float64{
		SymlinkStrategyRelative: 0,
	})
	require.ErrorIs(t, err, ErrSymlinkStrategyProbabilitiesNonPositive)
}

func TestValidateSpecialFileTypeProbabilitiesSentinelErrors(t *testing.T) {
	t.Parallel()

	err := validateSpecialFileTypeProbabilities(map[SpecialFileType]float64{})
	require.ErrorIs(t, err, ErrSpecialFileTypeProbabilitiesEmpty)

	err = validateSpecialFileTypeProbabilities(map[SpecialFileType]float64{
		SpecialFileTypeFIFO: 0,
	})
	require.ErrorIs(t, err, ErrSpecialFileTypeProbabilitiesNonPositive)
}

func TestValidateContentPatternProbabilitiesSentinelErrors(t *testing.T) {
	t.Parallel()

	err := validateContentPatternProbabilities(map[ContentPattern]float64{})
	require.ErrorIs(t, err, ErrContentPatternProbabilitiesEmpty)

	err = validateContentPatternProbabilities(map[ContentPattern]float64{
		ContentPatternDenseRandom: 0,
	})
	require.ErrorIs(t, err, ErrContentPatternProbabilitiesNonPositive)
}

func TestValidateXAttrNameAndACLEntries(t *testing.T) {
	t.Parallel()

	t.Run("XAttrNameValid", func(t *testing.T) {
		t.Parallel()

		name, err := validateXAttrName("user.test")
		require.NoError(t, err)
		require.Equal(t, "user.test", name)
	})

	t.Run("XAttrNameInvalid", func(t *testing.T) {
		t.Parallel()

		_, err := validateXAttrName("invalid")
		require.ErrorIs(t, err, ErrXAttrNameMissingNamespace)

		_, err = validateXAttrName("user.\x00name")
		require.ErrorIs(t, err, ErrXAttrNameContainsNUL)
	})

	t.Run("NormalizeACLEntries", func(t *testing.T) {
		t.Parallel()

		normalized, err := normalizeACLEntries([]string{"u::rw-", "g::r--", "u::rw-"})
		require.NoError(t, err)
		require.Equal(t, []string{"g::r--", "u::rw-"}, normalized)

		_, err = normalizeACLEntries([]string{""})
		require.ErrorIs(t, err, ErrACLEntryEmpty)

		_, err = normalizeACLEntries([]string{"u::rw-,g::r--"})
		require.ErrorIs(t, err, ErrACLEntryContainsComma)
	})
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
