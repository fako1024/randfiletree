package randfiletree

import (
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyOperationsWithOptionsFaultFailNthAndResume(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	ops := []Operation{
		{
			Kind: OperationKindCreateFile,
			Path: "/alpha.txt",
			Mode: 0o600,
			Data: []byte("alpha"),
		},
		{
			Kind: OperationKindCreateFile,
			Path: "/beta.txt",
			Mode: 0o600,
			Data: []byte("beta"),
		},
		{
			Kind: OperationKindAppend,
			Path: "/alpha.txt",
			Data: []byte("-tail"),
		},
	}

	err := ApplyOperationsWithOptions(base, ops, OperationApplyOptions{
		FaultProfile: FaultProfile{Rules: []FaultRule{{
			Nth:   2,
			Scope: FaultScopeMutation,
			Kind:  OperationKindCreateFile.String(),
		}}},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrFaultInjected)

	var applyErr *OperationApplyError
	require.True(t, errors.As(err, &applyErr))
	require.Equal(t, 1, applyErr.Index)
	require.Equal(t, "/beta.txt", applyErr.Operation.Path)

	var injectedErr *FaultInjectionError
	require.True(t, errors.As(err, &injectedErr))
	require.Equal(t, FaultScopeMutation, injectedErr.Point.Scope)
	require.Equal(t, 1, injectedErr.Point.Index)
	require.Equal(t, OperationKindCreateFile.String(), injectedErr.Point.Kind)
	require.Equal(t, "/beta.txt", injectedErr.Point.Path)

	alphaData, readErr := os.ReadFile(filepath.Join(base, "alpha.txt"))
	require.NoError(t, readErr)
	require.Equal(t, "alpha", string(alphaData))

	_, statErr := os.Stat(filepath.Join(base, "beta.txt"))
	require.ErrorIs(t, statErr, os.ErrNotExist)

	err = ApplyOperationsWithOptions(base, ops, OperationApplyOptions{StartIndex: applyErr.Index})
	require.NoError(t, err)

	alphaData, readErr = os.ReadFile(filepath.Join(base, "alpha.txt"))
	require.NoError(t, readErr)
	require.Equal(t, "alpha-tail", string(alphaData))

	betaData, readErr := os.ReadFile(filepath.Join(base, "beta.txt"))
	require.NoError(t, readErr)
	require.Equal(t, "beta", string(betaData))
}

func TestApplyOperationsWithOptionsFaultMatchesKindAndPathPattern(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	ops := []Operation{
		{
			Kind: OperationKindCreateDir,
			Path: "/nested",
			Mode: 0o750,
		},
		{
			Kind: OperationKindCreateFile,
			Path: "/nested/blocked.txt",
			Mode: 0o600,
			Data: []byte("blocked"),
		},
		{
			Kind: OperationKindCreateFile,
			Path: "/after.txt",
			Mode: 0o600,
			Data: []byte("after"),
		},
	}

	err := ApplyOperationsWithOptions(base, ops, OperationApplyOptions{
		FaultProfile: FaultProfile{Rules: []FaultRule{{
			Nth:         1,
			Scope:       FaultScopeMutation,
			Kind:        OperationKindCreateFile.String(),
			PathPattern: "/nested/*",
		}}},
	})
	require.Error(t, err)

	var applyErr *OperationApplyError
	require.True(t, errors.As(err, &applyErr))
	require.Equal(t, 1, applyErr.Index)
	require.Equal(t, "/nested/blocked.txt", applyErr.Operation.Path)

	nestedInfo, statErr := os.Stat(filepath.Join(base, "nested"))
	require.NoError(t, statErr)
	require.True(t, nestedInfo.IsDir())

	_, statErr = os.Stat(filepath.Join(base, "nested", "blocked.txt"))
	require.ErrorIs(t, statErr, os.ErrNotExist)

	_, statErr = os.Stat(filepath.Join(base, "after.txt"))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestApplyOperationsWithOptionsFaultReplayDeterministic(t *testing.T) {
	t.Parallel()

	ops := []Operation{
		{
			Kind: OperationKindCreateFile,
			Path: "/first.txt",
			Mode: 0o600,
			Data: []byte("first"),
		},
		{
			Kind: OperationKindCreateFile,
			Path: "/second.txt",
			Mode: 0o600,
			Data: []byte("second"),
		},
		{
			Kind: OperationKindAppend,
			Path: "/first.txt",
			Data: []byte("-tail"),
		},
	}

	opts := OperationApplyOptions{
		FaultProfile: FaultProfile{Rules: []FaultRule{{
			Nth:   2,
			Scope: FaultScopeMutation,
			Kind:  OperationKindCreateFile.String(),
		}}},
	}

	runOnce := func(base string) (int, FaultPoint) {
		err := ApplyOperationsWithOptions(base, ops, opts)
		require.Error(t, err)

		var applyErr *OperationApplyError
		require.True(t, errors.As(err, &applyErr))

		var injectedErr *FaultInjectionError
		require.True(t, errors.As(err, &injectedErr))

		return applyErr.Index, injectedErr.Point
	}

	indexA, pointA := runOnce(t.TempDir())
	indexB, pointB := runOnce(t.TempDir())

	require.Equal(t, indexA, indexB)
	require.Equal(t, pointA, pointB)
}

func TestApplyOperationsWithOptionsStartIndexValidation(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	ops := []Operation{{
		Kind: OperationKindCreateFile,
		Path: "/one.txt",
		Mode: 0o600,
		Data: []byte("one"),
	}}

	err := ApplyOperationsWithOptions(base, ops, OperationApplyOptions{StartIndex: -1})
	require.ErrorIs(t, err, ErrOperationStartIndexNegative)

	err = ApplyOperationsWithOptions(base, ops, OperationApplyOptions{StartIndex: len(ops) + 1})
	require.ErrorIs(t, err, ErrOperationStartIndexOutOfRange)

	err = ApplyOperationsWithOptions(base, ops, OperationApplyOptions{StartIndex: len(ops)})
	require.NoError(t, err)
}

func TestApplyOperationsWithOptionsRejectsInvalidFaultProfile(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	ops := []Operation{{
		Kind: OperationKindCreateFile,
		Path: "/one.txt",
		Mode: 0o600,
		Data: []byte("one"),
	}}

	err := ApplyOperationsWithOptions(base, ops, OperationApplyOptions{
		FaultProfile: FaultProfile{Rules: []FaultRule{{
			Nth: 0,
		}}},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "nth must be > 0")

	err = ApplyOperationsWithOptions(base, ops, OperationApplyOptions{
		FaultProfile: FaultProfile{Rules: []FaultRule{{
			Nth:         1,
			PathPattern: "[",
		}}},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid path pattern")
}

func TestRunWithOptionsFaultInjectionIncludesMetadata(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := newFaultTestGenerator(t, base)

	err := g.RunWithOptions(RunOptions{
		FaultProfile: FaultProfile{Rules: []FaultRule{{
			Nth:   1,
			Scope: FaultScopeRun,
			Kind:  "create-file",
		}}},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrFaultInjected)

	var injectedErr *FaultInjectionError
	require.True(t, errors.As(err, &injectedErr))
	require.Equal(t, FaultScopeRun, injectedErr.Point.Scope)
	require.Equal(t, "create-file", injectedErr.Point.Kind)
	require.Equal(t, 1, injectedErr.Point.Index)
	require.Equal(t, filepath.Join(base, "file"), injectedErr.Point.Path)

	_, statErr := os.Stat(filepath.Join(base, "file"))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestGeneratorExecutionOptionsNilReceiver(t *testing.T) {
	t.Parallel()

	var g *Generator

	err := g.RunWithOptions(RunOptions{})
	require.ErrorIs(t, err, ErrNilGenerator)

	err = g.ApplyOperationsWithOptions(nil, OperationApplyOptions{})
	require.ErrorIs(t, err, ErrNilGenerator)
}

func newFaultTestGenerator(t *testing.T, basePath string) *Generator {
	t.Helper()

	g := New(basePath)
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
