package randfiletree

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestByteNameDeterministicUnderSameSeed(t *testing.T) {
	t.Parallel()

	baseA := filepath.Join(t.TempDir(), "tree")
	baseB := filepath.Join(t.TempDir(), "tree")

	newConfigured := func(base string) *Generator {
		g := New(base)
		require.NoError(t, g.Configure(
			WithSeed(42),
			WithRunMode(RunModeStrict),
			WithDirNameGenerator(StringGeneratorAlphabet(FileNameAlphabetBasic)),
			WithDirNameLengthGenerator(NumberGeneratorConstant(8)),
			WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
			WithFilesPerDirectoryGenerator(NumberGeneratorConstant(2)),
			WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(1)),
			WithByteFileNameGenerator(ByteNamePresetLeadingDots),
			WithFileNameLengthGenerator(NumberGeneratorConstant(10)),
			WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
			WithDataGenerator(DataGeneratorRandomFixedLen(16)),
			WithPathDepthGenerator(NumberGeneratorConstant(2)),
			WithSymlinkProbability(0),
			WithRelativeSymlinkProbability(0),
			WithHardlinkProbability(0),
		))
		return g
	}

	gA := newConfigured(baseA)
	gB := newConfigured(baseB)

	planA, err := gA.planRun()
	require.NoError(t, err)

	planB, err := gB.planRun()
	require.NoError(t, err)

	for i := range planA.entries {
		relA, _ := filepath.Rel(baseA, planA.entries[i].path)
		relB, _ := filepath.Rel(baseB, planB.entries[i].path)
		planA.entries[i].path = relA
		planB.entries[i].path = relB
	}

	require.Equal(t, planA.entries, planB.entries)
}

func TestWithByteFileNameGeneratorRejectsNil(t *testing.T) {
	t.Parallel()

	g := New(t.TempDir())
	err := g.Configure(WithByteFileNameGenerator(nil))
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not be nil")
}

func TestWithByteDirNameGeneratorRejectsNil(t *testing.T) {
	t.Parallel()

	g := New(t.TempDir())
	err := g.Configure(WithByteDirNameGenerator(nil))
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not be nil")
}
