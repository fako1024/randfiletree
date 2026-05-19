//go:build linux

package randfiletree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunWithByteFileNames(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
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

	require.NoError(t, g.Run())

	var foundFiles int
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			foundFiles++
		}
		return nil
	})
	require.NoError(t, err)
	require.Greater(t, foundFiles, 0, "expected files to be created")
}

func TestRunWithByteDirNames(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := New(base)
	require.NoError(t, g.Configure(
		WithSeed(42),
		WithRunMode(RunModeStrict),
		WithByteDirNameGenerator(ByteNamePresetLeadingDots),
		WithDirNameLengthGenerator(NumberGeneratorConstant(8)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(2)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithFileNameGenerator(StringGeneratorAlphabet(FileNameAlphabetBasic)),
		WithFileNameLengthGenerator(NumberGeneratorConstant(10)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
		WithDataGenerator(DataGeneratorRandomFixedLen(16)),
		WithPathDepthGenerator(NumberGeneratorConstant(2)),
		WithSymlinkProbability(0),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
	))

	require.NoError(t, g.Run())

	var foundDirs int
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && path != base {
			foundDirs++
		}
		return nil
	})
	require.NoError(t, err)
	require.Greater(t, foundDirs, 0, "expected directories to be created")
}

func TestRunWithInvalidUTF8Names(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := New(base)
	require.NoError(t, g.Configure(
		WithSeed(42),
		WithRunMode(RunModeStrict),
		WithDirNameGenerator(StringGeneratorAlphabet(FileNameAlphabetBasic)),
		WithDirNameLengthGenerator(NumberGeneratorConstant(8)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(2)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithByteFileNameGenerator(ByteNamePresetInvalidUTF8),
		WithFileNameLengthGenerator(NumberGeneratorConstant(15)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
		WithDataGenerator(DataGeneratorRandomFixedLen(16)),
		WithPathDepthGenerator(NumberGeneratorConstant(2)),
		WithSymlinkProbability(0),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
	))

	require.NoError(t, g.Run())

	var foundFiles int
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			foundFiles++
		}
		return nil
	})
	require.NoError(t, err)
	require.Greater(t, foundFiles, 0, "expected files with invalid UTF-8 names to be created")
}

func TestRunWithControlCharNames(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := New(base)
	require.NoError(t, g.Configure(
		WithSeed(42),
		WithRunMode(RunModeStrict),
		WithDirNameGenerator(StringGeneratorAlphabet(FileNameAlphabetBasic)),
		WithDirNameLengthGenerator(NumberGeneratorConstant(8)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(2)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithByteFileNameGenerator(ByteNamePresetControlChars),
		WithFileNameLengthGenerator(NumberGeneratorConstant(12)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
		WithDataGenerator(DataGeneratorRandomFixedLen(16)),
		WithPathDepthGenerator(NumberGeneratorConstant(2)),
		WithSymlinkProbability(0),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
	))

	require.NoError(t, g.Run())

	var foundFiles int
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			foundFiles++
		}
		return nil
	})
	require.NoError(t, err)
	require.Greater(t, foundFiles, 0, "expected files with control char names to be created")
}
