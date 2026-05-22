package randfiletree

import (
	"math/rand"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/fako1024/randfiletree/diff"
	"github.com/stretchr/testify/require"
)

func TestBuildScenarioManifestDeterministicForSameInput(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := newManifestConfiguredGenerator(t, base)

	manifestA, err := g.BuildScenarioManifest(nil)
	require.NoError(t, err)

	manifestB, err := g.BuildScenarioManifest(nil)
	require.NoError(t, err)

	require.Equal(t, manifestA, manifestB)
	require.NotEmpty(t, manifestA.Entries)
	require.Equal(t, "/", manifestA.Entries[0].Path)
	require.Equal(t, ScenarioManifestEntryTypeDir, manifestA.Entries[0].Type)
}

func TestBuildScenarioManifestIncludesCapabilities(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := newManifestConfiguredGenerator(t, base)

	manifest, err := g.BuildScenarioManifest([]Operation{{
		Kind:      OperationKindSetXAttr,
		Path:      "/file",
		XAttrName: "user.test",
	}, {
		Kind: OperationKindChown,
		Path: "/file",
		UID:  1,
		GID:  1,
	}})
	require.NoError(t, err)

	require.Contains(t, manifest.RequiredCapabilities, ScenarioManifestCapabilityOperationXAttr)
	require.Contains(t, manifest.RequiredCapabilities, ScenarioManifestCapabilityOperationChown)
}

func TestApplyScenarioManifestParity(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")

	g := newManifestConfiguredGenerator(t, left)

	ops := []Operation{
		{
			Kind: OperationKindAppend,
			Path: "/file",
			Data: []byte("-tail"),
		},
		{
			Kind: OperationKindCreateFile,
			Path: "/extra.txt",
			Mode: 0o600,
			Data: []byte("extra"),
		},
	}

	manifest, err := g.BuildScenarioManifest(ops)
	require.NoError(t, err)

	require.NoError(t, g.Run())
	require.NoError(t, ApplyOperations(left, ops))

	require.NoError(t, ApplyScenarioManifest(right, manifest))

	fixedTime := time.Unix(1_700_000_100, 0)
	require.NoError(t, normalizeTreeMTime(left, fixedTime))
	require.NoError(t, normalizeTreeMTime(right, fixedTime))

	require.NoError(t, diff.PathsWithOptions(left, right, diff.DefaultOptions()))
}

func TestApplyScenarioManifestNilGeneratorReceiver(t *testing.T) {
	t.Parallel()

	var g *Generator
	err := g.ApplyScenarioManifest(ScenarioManifest{})
	require.ErrorIs(t, err, ErrNilGenerator)
}

func TestApplyScenarioManifestFailsForUnsupportedCapabilities(t *testing.T) {
	t.Parallel()

	manifest := ScenarioManifest{
		Version: scenarioManifestVersion,
		Generator: ScenarioManifestGenerator{
			Seed:           1,
			RunMode:        RunModeReplace,
			PlanEntryLimit: 32,
		},
		Entries: []ScenarioManifestEntry{
			{
				Type: ScenarioManifestEntryTypeDir,
				Path: "/",
				Mode: 0o750,
			},
		},
		RequiredCapabilities: []ScenarioManifestCapability{
			ScenarioManifestCapabilityLinuxSpecialFiles,
		},
	}

	sealed, err := sealScenarioManifestIntegrity(manifest)
	require.NoError(t, err)

	err = ApplyScenarioManifest(t.TempDir(), sealed)
	if runtime.GOOS == "linux" {
		require.NoError(t, err)
		return
	}

	require.Error(t, err)
	require.ErrorIs(t, err, ErrScenarioManifestCapabilityUnsupported)
}

func TestNormalizeScenarioManifestRejectsMissingCapabilitiesForPayload(t *testing.T) {
	t.Parallel()

	manifest := ScenarioManifest{
		Version: scenarioManifestVersion,
		Generator: ScenarioManifestGenerator{
			Seed:           1,
			RunMode:        RunModeAppend,
			PlanEntryLimit: 32,
		},
		Entries: []ScenarioManifestEntry{
			{
				Type: ScenarioManifestEntryTypeDir,
				Path: "/",
				Mode: 0o750,
			},
			{
				Type: ScenarioManifestEntryTypeFile,
				Path: "/file",
				Mode: 0o600,
				Metadata: &ScenarioManifestMetadata{
					XAttrs: []ScenarioManifestXAttr{{Name: "user.test", Value: []byte("v")}},
				},
			},
		},
		RequiredCapabilities: nil,
	}

	_, err := normalizeScenarioManifest(manifest)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrScenarioManifestCapabilitiesIncomplete)
}

func TestNormalizeScenarioManifestRejectsHardlinkToNonFile(t *testing.T) {
	t.Parallel()

	manifest := ScenarioManifest{
		Version: scenarioManifestVersion,
		Generator: ScenarioManifestGenerator{
			Seed:           1,
			RunMode:        RunModeAppend,
			PlanEntryLimit: 32,
		},
		Entries: []ScenarioManifestEntry{
			{Type: ScenarioManifestEntryTypeDir, Path: "/", Mode: 0o750},
			{Type: ScenarioManifestEntryTypeDir, Path: "/dir", Mode: 0o750},
			{Type: ScenarioManifestEntryTypeHardlink, Path: "/h", SourcePath: "/dir"},
		},
		RequiredCapabilities: nil,
	}

	_, err := normalizeScenarioManifest(manifest)
	require.Error(t, err)
	require.ErrorContains(t, err, "is not a regular file path")
}

func TestNormalizeScenarioManifestRejectsMissingParentEntry(t *testing.T) {
	t.Parallel()

	manifest := ScenarioManifest{
		Version: scenarioManifestVersion,
		Generator: ScenarioManifestGenerator{
			Seed:           1,
			RunMode:        RunModeAppend,
			PlanEntryLimit: 32,
		},
		Entries: []ScenarioManifestEntry{
			{Type: ScenarioManifestEntryTypeDir, Path: "/", Mode: 0o750},
			{Type: ScenarioManifestEntryTypeFile, Path: "/missing/child", Mode: 0o600, Data: []byte("x")},
		},
	}

	_, err := normalizeScenarioManifest(manifest)
	require.Error(t, err)
	require.ErrorContains(t, err, "missing parent directory")
}

func TestParseScenarioManifestRejectsVersionTooOldAndTooNew(t *testing.T) {
	t.Parallel()

	oldManifest := ScenarioManifest{
		Version: scenarioManifestVersionMinSupported - 1,
		Generator: ScenarioManifestGenerator{
			Seed:           1,
			RunMode:        RunModeAppend,
			PlanEntryLimit: 32,
		},
		Entries: []ScenarioManifestEntry{{Type: ScenarioManifestEntryTypeDir, Path: "/", Mode: 0o750}},
	}

	oldPayloadBytes, err := jsoniter.Marshal(oldManifest)
	require.NoError(t, err)
	_, err = ParseScenarioManifestJSON(string(oldPayloadBytes))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrScenarioManifestVersionTooOld)

	newManifest := oldManifest
	newManifest.Version = scenarioManifestVersionMaxSupported + 1

	newPayloadBytes, err := jsoniter.Marshal(newManifest)
	require.NoError(t, err)
	_, err = ParseScenarioManifestJSON(string(newPayloadBytes))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrScenarioManifestVersionTooNew)
}

func TestApplyScenarioManifestPathValidation(t *testing.T) {
	t.Parallel()

	err := ApplyScenarioManifest("", ScenarioManifest{})
	require.ErrorIs(t, err, ErrBasePathEmpty)
}

func TestBuildScenarioManifestNilGenerator(t *testing.T) {
	t.Parallel()

	_, err := BuildScenarioManifest(nil, nil)
	require.ErrorIs(t, err, ErrNilGenerator)
}

// TestScenarioManifestChecksumGolden pins the canonical JSON checksum of a
// minimal manifest. Any change to the encoder configuration, field layout, or
// integrity algorithm that alters this checksum must be treated as a payload
// format change and a manifest version bump, since checksums already embedded
// in manifests on disk would no longer verify.
func TestScenarioManifestChecksumGolden(t *testing.T) {
	t.Parallel()

	manifest := ScenarioManifest{
		Version: 1,
		Generator: ScenarioManifestGenerator{
			Seed:           42,
			RunMode:        RunModeAppend,
			PlanEntryLimit: 1024,
		},
		Entries: []ScenarioManifestEntry{
			{Type: ScenarioManifestEntryTypeDir, Path: "/", Mode: 0o755},
			{Type: ScenarioManifestEntryTypeFile, Path: "/a.txt", Mode: 0o644, Data: []byte("alpha")},
		},
	}

	checksum, err := scenarioManifestChecksum(manifest)
	require.NoError(t, err)
	require.Equal(t, "e55fae3a2a3ac4a15805eb60e4c1f7c16eecc172454fb500cfcbd617f42478cc", checksum)
}

func newManifestConfiguredGenerator(t *testing.T, basePath string) *Generator {
	t.Helper()

	g := New(basePath)
	require.NoError(t, g.Configure(
		WithRunMode(RunModeReplace),
		WithSeed(17),
		WithPlanEntryLimit(128),
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
		WithDataGenerator(DataGeneratorFixedString("payload")),
		WithPathDepthGenerator(NumberGeneratorConstant(1)),
		WithSymlinkProbability(0),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
	))

	return g
}
