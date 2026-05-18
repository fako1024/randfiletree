package randfiletree

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultOptions(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "randfiletree")

	g := New(path)
	require.NoError(t, g.RemoveAll())
	require.NoError(t, g.Run())
	_, err := os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)

	require.NoError(t, g.Run())
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestRunRejectsIncompleteConfigurationWithoutPanic(t *testing.T) {
	t.Parallel()

	g := New(t.TempDir())
	require.NoError(t, g.Configure(WithPathDepthGenerator(NumberGeneratorConstant(1))))

	var err error
	require.NotPanics(t, func() {
		err = g.Run()
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "generator configuration incomplete")
	require.ErrorContains(t, err, "directory name generator")
}

func TestPlanRunDeterministicForSameSeedAndOptions(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")

	newConfigured := func() *Generator {
		g := New(base)
		require.NoError(t, g.Configure(
			WithSeed(42),
			WithRunMode(RunModeStrict),
			WithDirNameGenerator(StringGeneratorAlphabet(FileNameAlphabetBasic)),
			WithDirNameLengthGenerator(NumberGeneratorConstant(8)),
			WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
			WithFilesPerDirectoryGenerator(NumberGeneratorConstant(2)),
			WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(1)),
			WithFileNameGenerator(StringGeneratorAlphabet(FileNameAlphabetBasic)),
			WithFileNameLengthGenerator(NumberGeneratorConstant(8)),
			WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
			WithDataGenerator(DataGeneratorRandomFixedLen(16)),
			WithPathDepthGenerator(NumberGeneratorConstant(2)),
			WithSymlinkProbability(0),
			WithRelativeSymlinkProbability(0),
			WithHardlinkProbability(0),
		))

		return g
	}

	gA := newConfigured()
	gB := newConfigured()

	planA, err := gA.planRun()
	require.NoError(t, err)

	planB, err := gB.planRun()
	require.NoError(t, err)

	require.Equal(t, planA, planB)
}

func TestRunReturnsDeterministicCollisionError(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")

	newConfigured := func() *Generator {
		g := New(base)
		require.NoError(t, g.Configure(
			WithRunMode(RunModeAppend),
			WithSeed(1),
			WithDirNameGenerator(func(r *rand.Rand, length int) string {
				return "d"
			}),
			WithDirNameLengthGenerator(NumberGeneratorConstant(1)),
			WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
			WithFilesPerDirectoryGenerator(NumberGeneratorConstant(2)),
			WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(0)),
			WithFileNameGenerator(func(r *rand.Rand, length int) string {
				return "x"
			}),
			WithFileNameLengthGenerator(NumberGeneratorConstant(1)),
			WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
			WithDataGenerator(DataGeneratorFixedString("payload")),
			WithPathDepthGenerator(NumberGeneratorConstant(1)),
			WithSymlinkProbability(0),
			WithRelativeSymlinkProbability(0),
			WithHardlinkProbability(0),
		))

		return g
	}

	errA := newConfigured().Run()
	require.Error(t, errA)
	require.ErrorIs(t, errA, ErrPlanPathCollisionExhausted)

	errB := newConfigured().Run()
	require.Error(t, errB)
	require.ErrorIs(t, errB, ErrPlanPathCollisionExhausted)

	require.Equal(t, errA.Error(), errB.Error())
	_, err := os.Stat(base)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestRunModeAppendRecursesIntoExistingDirectory(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	existing := filepath.Join(base, "dir")
	require.NoError(t, os.MkdirAll(existing, 0o750))

	g := New(base)
	require.NoError(t, g.Configure(
		WithRunMode(RunModeAppend),
		WithDirNameGenerator(func(r *rand.Rand, length int) string {
			return "dir"
		}),
		WithDirNameLengthGenerator(NumberGeneratorConstant(3)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithFileNameGenerator(func(r *rand.Rand, length int) string {
			return "file"
		}),
		WithFileNameLengthGenerator(NumberGeneratorConstant(4)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
		WithDataGenerator(DataGeneratorFixedString("payload")),
		WithPathDepthGenerator(NumberGeneratorConstant(2)),
		WithSymlinkProbability(0),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
	))

	require.NoError(t, g.Run())

	info, err := os.Lstat(filepath.Join(existing, "file"))
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular())
}

func TestRunModeStrictFailsOnExistingPath(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	require.NoError(t, os.MkdirAll(base, 0o750))

	existingFile := filepath.Join(base, "file")
	require.NoError(t, os.WriteFile(existingFile, []byte("keep"), 0o600))

	g := New(base)
	require.NoError(t, g.Configure(
		WithRunMode(RunModeStrict),
		WithDirNameGenerator(func(r *rand.Rand, length int) string {
			return "dir"
		}),
		WithDirNameLengthGenerator(NumberGeneratorConstant(3)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(0)),
		WithFileNameGenerator(func(r *rand.Rand, length int) string {
			return "file"
		}),
		WithFileNameLengthGenerator(NumberGeneratorConstant(4)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
		WithDataGenerator(DataGeneratorFixedString("new")),
		WithPathDepthGenerator(NumberGeneratorConstant(1)),
		WithSymlinkProbability(0),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
	))

	err := g.Run()
	require.Error(t, err)
	require.ErrorContains(t, err, "strict mode")
	require.ErrorContains(t, err, existingFile)

	data, readErr := os.ReadFile(existingFile)
	require.NoError(t, readErr)
	require.Equal(t, "keep", string(data))
}

func TestRunModeReplaceClearsBasePathBeforeApply(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	legacyFile := filepath.Join(base, "legacy")
	require.NoError(t, os.MkdirAll(base, 0o750))
	require.NoError(t, os.WriteFile(legacyFile, []byte("legacy"), 0o600))

	g := New(base)
	require.NoError(t, g.Configure(
		WithRunMode(RunModeReplace),
		WithDirNameGenerator(func(r *rand.Rand, length int) string {
			return "dir"
		}),
		WithDirNameLengthGenerator(NumberGeneratorConstant(3)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(0)),
		WithFileNameGenerator(func(r *rand.Rand, length int) string {
			return "fresh"
		}),
		WithFileNameLengthGenerator(NumberGeneratorConstant(5)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
		WithDataGenerator(DataGeneratorFixedString("new")),
		WithPathDepthGenerator(NumberGeneratorConstant(1)),
		WithSymlinkProbability(0),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
	))

	require.NoError(t, g.Run())

	_, err := os.Stat(legacyFile)
	require.ErrorIs(t, err, os.ErrNotExist)

	newFileInfo, err := os.Stat(filepath.Join(base, "fresh"))
	require.NoError(t, err)
	require.True(t, newFileInfo.Mode().IsRegular())
}

func TestRunDoesNotLeakLastPathAcrossRuns(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := New(base)

	require.NoError(t, g.Configure(
		WithRunMode(RunModeAppend),
		WithDirNameGenerator(func(r *rand.Rand, length int) string {
			return "dir"
		}),
		WithDirNameLengthGenerator(NumberGeneratorConstant(3)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(0)),
		WithFileNameGenerator(func(r *rand.Rand, length int) string {
			return "first"
		}),
		WithFileNameLengthGenerator(NumberGeneratorConstant(5)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
		WithDataGenerator(DataGeneratorFixedString("first")),
		WithPathDepthGenerator(NumberGeneratorConstant(1)),
		WithSymlinkProbability(0),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
	))
	require.NoError(t, g.Run())

	require.NoError(t, g.Configure(
		WithRunMode(RunModeAppend),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(0)),
		WithFileNameGenerator(func(r *rand.Rand, length int) string {
			return "second"
		}),
		WithFileNameLengthGenerator(NumberGeneratorConstant(6)),
		WithDataGenerator(DataGeneratorFixedString("second")),
		WithSymlinkGenerator(func(r *rand.Rand) bool {
			return true
		}),
		WithRelativeSymlinkProbability(0),
		WithPathDepthGenerator(NumberGeneratorConstant(1)),
		WithHardlinkProbability(0),
	))
	require.NoError(t, g.Run())

	info, err := os.Lstat(filepath.Join(base, "second"))
	require.NoError(t, err)
	require.Zero(t, info.Mode()&os.ModeSymlink)
	require.True(t, info.Mode().IsRegular())
}

func TestWriteRelSymlinkUsesDirectoryRelativeTarget(t *testing.T) {
	t.Parallel()
	requireSymlinkSupport(t)

	base := t.TempDir()
	dir := filepath.Join(base, "nested")
	require.NoError(t, os.MkdirAll(dir, 0o750))

	target := filepath.Join(base, "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("content"), 0o600))

	g := New(base)
	require.NoError(t, g.Configure(
		WithFileNameGenerator(func(rnd *rand.Rand, length int) string {
			return "rel_link"
		}),
		WithFileNameLengthGenerator(NumberGeneratorConstant(8)),
	))

	require.NoError(t, g.writeRelSymlink(dir, target))

	linkPath := filepath.Join(dir, "rel_link")
	linkTarget, err := os.Readlink(linkPath)
	require.NoError(t, err)

	expected, err := filepath.Rel(dir, target)
	require.NoError(t, err)
	require.Equal(t, expected, linkTarget)

	resolved := filepath.Clean(filepath.Join(dir, linkTarget))
	require.Equal(t, target, resolved)
}

func TestWriteRelSymlinkRejectsEmptyTarget(t *testing.T) {
	t.Parallel()
	requireSymlinkSupport(t)

	dir := t.TempDir()
	g := New(dir)

	err := g.writeRelSymlink(dir, "")
	require.ErrorIs(t, err, ErrEmptySymlinkTarget)
}

func TestRunPlansAndAppliesHardlinks(t *testing.T) {
	t.Parallel()
	requireHardlinkSupport(t)

	base := filepath.Join(t.TempDir(), "tree")
	nameIdx := 0

	g := New(base)
	require.NoError(t, g.Configure(
		WithRunMode(RunModeAppend),
		WithSeed(11),
		WithDirNameGenerator(func(r *rand.Rand, length int) string {
			return "dir"
		}),
		WithDirNameLengthGenerator(NumberGeneratorConstant(3)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(4)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(0)),
		WithFileNameGenerator(func(r *rand.Rand, length int) string {
			name := fmt.Sprintf("n%02d", nameIdx)
			nameIdx++
			return name
		}),
		WithFileNameLengthGenerator(NumberGeneratorConstant(3)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
		WithDataGenerator(DataGeneratorFixedString("payload")),
		WithPathDepthGenerator(NumberGeneratorConstant(1)),
		WithSymlinkProbability(0),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(1),
	))

	plan, err := g.planRun()
	require.NoError(t, err)

	nFiles := 0
	nHardlinks := 0
	for _, entry := range plan.entries {
		switch entry.typeID {
		case plannedEntryTypeFile:
			nFiles++
		case plannedEntryTypeHardlink:
			nHardlinks++
		}
	}
	require.Equal(t, 1, nFiles)
	require.Equal(t, 3, nHardlinks)
	require.Len(t, plan.hardlinkGroups, 1)
	require.Len(t, plan.hardlinkGroups[0].paths, 4)

	require.NoError(t, g.Run())

	entries, err := os.ReadDir(base)
	require.NoError(t, err)

	regularFiles := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			continue
		}
		regularFiles = append(regularFiles, filepath.Join(base, entry.Name()))
	}
	sort.Strings(regularFiles)
	require.Len(t, regularFiles, 4)

	firstInfo, err := os.Stat(regularFiles[0])
	require.NoError(t, err)
	secondInfo, err := os.Stat(regularFiles[1])
	require.NoError(t, err)
	require.True(t, os.SameFile(firstInfo, secondInfo))

	require.NoError(t, os.WriteFile(regularFiles[0], []byte("updated"), 0o600))
	updatedData, err := os.ReadFile(regularFiles[1])
	require.NoError(t, err)
	require.Equal(t, "updated", string(updatedData))
}

func TestRunSymlinkStrategyAbsolute(t *testing.T) {
	t.Parallel()
	requireSymlinkSupport(t)

	base := filepath.Join(t.TempDir(), "tree")
	nameIdx := 0

	g := New(base)
	require.NoError(t, g.Configure(
		WithRunMode(RunModeAppend),
		WithSeed(7),
		WithDirNameGenerator(func(r *rand.Rand, length int) string {
			return "dir"
		}),
		WithDirNameLengthGenerator(NumberGeneratorConstant(3)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(2)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(0)),
		WithFileNameGenerator(func(r *rand.Rand, length int) string {
			name := fmt.Sprintf("n%02d", nameIdx)
			nameIdx++
			return name
		}),
		WithFileNameLengthGenerator(NumberGeneratorConstant(3)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
		WithDataGenerator(DataGeneratorFixedString("payload")),
		WithPathDepthGenerator(NumberGeneratorConstant(1)),
		WithSymlinkProbability(1),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
		WithSymlinkStrategyGenerator(func(r *rand.Rand) SymlinkStrategy {
			return SymlinkStrategyAbsolute
		}),
	))

	require.NoError(t, g.Run())

	target, err := os.Readlink(filepath.Join(base, "n01"))
	require.NoError(t, err)
	require.Equal(t, filepath.Join(base, "n00"), target)
}

func TestRunSymlinkStrategyRelative(t *testing.T) {
	t.Parallel()
	requireSymlinkSupport(t)

	base := filepath.Join(t.TempDir(), "tree")
	nameIdx := 0

	g := New(base)
	require.NoError(t, g.Configure(
		WithRunMode(RunModeAppend),
		WithSeed(7),
		WithDirNameGenerator(func(r *rand.Rand, length int) string {
			return "dir"
		}),
		WithDirNameLengthGenerator(NumberGeneratorConstant(3)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(2)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(0)),
		WithFileNameGenerator(func(r *rand.Rand, length int) string {
			name := fmt.Sprintf("n%02d", nameIdx)
			nameIdx++
			return name
		}),
		WithFileNameLengthGenerator(NumberGeneratorConstant(3)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
		WithDataGenerator(DataGeneratorFixedString("payload")),
		WithPathDepthGenerator(NumberGeneratorConstant(1)),
		WithSymlinkProbability(1),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
		WithSymlinkStrategyGenerator(func(r *rand.Rand) SymlinkStrategy {
			return SymlinkStrategyRelative
		}),
	))

	require.NoError(t, g.Run())

	target, err := os.Readlink(filepath.Join(base, "n01"))
	require.NoError(t, err)
	require.Equal(t, "n00", target)
}

func TestRunSymlinkStrategyDangling(t *testing.T) {
	t.Parallel()
	requireSymlinkSupport(t)

	base := filepath.Join(t.TempDir(), "tree")
	nameIdx := 0

	g := New(base)
	require.NoError(t, g.Configure(
		WithRunMode(RunModeAppend),
		WithSeed(7),
		WithDirNameGenerator(func(r *rand.Rand, length int) string {
			return "dir"
		}),
		WithDirNameLengthGenerator(NumberGeneratorConstant(3)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(0)),
		WithFileNameGenerator(func(r *rand.Rand, length int) string {
			name := fmt.Sprintf("n%02d", nameIdx)
			nameIdx++
			return name
		}),
		WithFileNameLengthGenerator(NumberGeneratorConstant(3)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
		WithDataGenerator(DataGeneratorFixedString("payload")),
		WithPathDepthGenerator(NumberGeneratorConstant(1)),
		WithSymlinkProbability(1),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
		WithSymlinkStrategyGenerator(func(r *rand.Rand) SymlinkStrategy {
			return SymlinkStrategyDangling
		}),
	))

	require.NoError(t, g.Run())

	linkPath := filepath.Join(base, "n01")
	target, err := os.Readlink(linkPath)
	require.NoError(t, err)

	_, err = os.Stat(target)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestRunSymlinkStrategySelfReferential(t *testing.T) {
	t.Parallel()
	requireSymlinkSupport(t)

	base := filepath.Join(t.TempDir(), "tree")
	nameIdx := 0

	g := New(base)
	require.NoError(t, g.Configure(
		WithRunMode(RunModeAppend),
		WithSeed(7),
		WithDirNameGenerator(func(r *rand.Rand, length int) string {
			return "dir"
		}),
		WithDirNameLengthGenerator(NumberGeneratorConstant(3)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(0)),
		WithFileNameGenerator(func(r *rand.Rand, length int) string {
			name := fmt.Sprintf("n%02d", nameIdx)
			nameIdx++
			return name
		}),
		WithFileNameLengthGenerator(NumberGeneratorConstant(3)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
		WithDataGenerator(DataGeneratorFixedString("payload")),
		WithPathDepthGenerator(NumberGeneratorConstant(1)),
		WithSymlinkProbability(1),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
		WithSymlinkStrategyGenerator(func(r *rand.Rand) SymlinkStrategy {
			return SymlinkStrategySelfReferential
		}),
	))

	require.NoError(t, g.Run())

	target, err := os.Readlink(filepath.Join(base, "n00"))
	require.NoError(t, err)
	require.Equal(t, "n00", target)
}

func TestRunSymlinkStrategyCycleAndChained(t *testing.T) {
	t.Parallel()
	requireSymlinkSupport(t)

	base := filepath.Join(t.TempDir(), "tree")
	nameIdx := 0
	strategyCall := 0

	g := New(base)
	require.NoError(t, g.Configure(
		WithRunMode(RunModeAppend),
		WithSeed(9),
		WithDirNameGenerator(func(r *rand.Rand, length int) string {
			return "dir"
		}),
		WithDirNameLengthGenerator(NumberGeneratorConstant(3)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(2)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(0)),
		WithFileNameGenerator(func(r *rand.Rand, length int) string {
			name := fmt.Sprintf("n%02d", nameIdx)
			nameIdx++
			return name
		}),
		WithFileNameLengthGenerator(NumberGeneratorConstant(3)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
		WithDataGenerator(DataGeneratorFixedString("payload")),
		WithPathDepthGenerator(NumberGeneratorConstant(1)),
		WithSymlinkProbability(1),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
		WithSymlinkStrategyGenerator(func(r *rand.Rand) SymlinkStrategy {
			strategyCall++
			if strategyCall == 1 {
				return SymlinkStrategyCycle
			}

			return SymlinkStrategyChained
		}),
	))

	require.NoError(t, g.Run())

	targetN00, err := os.Readlink(filepath.Join(base, "n00"))
	require.NoError(t, err)
	require.Equal(t, "n01", targetN00)

	targetN01, err := os.Readlink(filepath.Join(base, "n01"))
	require.NoError(t, err)
	require.Equal(t, "n00", targetN01)

	targetN02, err := os.Readlink(filepath.Join(base, "n02"))
	require.NoError(t, err)
	require.Contains(t, []string{filepath.Join(base, "n00"), filepath.Join(base, "n01")}, targetN02)

	nVisited := 0
	require.NoError(t, g.Walk(func(path string, info os.FileInfo, err error) error {
		require.NoError(t, err)
		nVisited++
		return nil
	}))
	require.Equal(t, 4, nVisited)
}

func requireSymlinkSupport(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")
	link := filepath.Join(tmpDir, "link.txt")

	require.NoError(t, os.WriteFile(target, []byte("data"), 0o600))
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported in this test environment: %s", err)
	}
}

func requireHardlinkSupport(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")
	link := filepath.Join(tmpDir, "link.txt")

	require.NoError(t, os.WriteFile(target, []byte("data"), 0o600))
	if err := os.Link(target, link); err != nil {
		t.Skipf("hardlink not supported in this test environment: %s", err)
	}
}

func TestRunRejectsXAttrNamespaceWithoutOptIn(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := New(base)
	require.NoError(t, g.Configure(
		WithRunMode(RunModeAppend),
		WithSeed(1),
		WithDirNameGenerator(func(r *rand.Rand, length int) string { return "dir" }),
		WithDirNameLengthGenerator(NumberGeneratorConstant(3)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(0)),
		WithFileNameGenerator(func(r *rand.Rand, length int) string { return "file" }),
		WithFileNameLengthGenerator(NumberGeneratorConstant(4)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
		WithDataGenerator(DataGeneratorFixedString("payload")),
		WithPathDepthGenerator(NumberGeneratorConstant(1)),
		WithSymlinkProbability(0),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
		WithXAttr("trusted.test", []byte("value")),
	))

	err := g.Run()
	require.Error(t, err)
	require.ErrorContains(t, err, ErrXAttrNamespaceNotAllowed.Error())
	_, statErr := os.Stat(base)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestRunRejectsIncompleteSpecialDeviceConfiguration(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := New(base)
	require.NoError(t, g.Configure(
		WithRunMode(RunModeAppend),
		WithSeed(1),
		WithDirNameGenerator(func(r *rand.Rand, length int) string { return "dir" }),
		WithDirNameLengthGenerator(NumberGeneratorConstant(3)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(0)),
		WithFileNameGenerator(func(r *rand.Rand, length int) string { return "file" }),
		WithFileNameLengthGenerator(NumberGeneratorConstant(4)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o600)),
		WithDataGenerator(DataGeneratorFixedString("payload")),
		WithPathDepthGenerator(NumberGeneratorConstant(1)),
		WithSymlinkProbability(0),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
		WithSpecialFileProbability(1),
		WithSpecialFileTypeGenerator(func(r *rand.Rand) SpecialFileType {
			return SpecialFileTypeCharDevice
		}),
		WithSpecialDeviceNumberGenerators(NumberGeneratorConstant(1), NumberGeneratorConstant(2)),
		func(next *Generator) error {
			next.specialDeviceMinorGen = nil
			return nil
		},
	))

	err := g.Run()
	require.Error(t, err)
	require.ErrorContains(t, err, ErrSpecialDeviceConfigurationIncomplete.Error())
}
