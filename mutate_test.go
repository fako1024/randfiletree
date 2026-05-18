package randfiletree

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/fako1024/randfiletree/diff"
	"github.com/stretchr/testify/require"
)

func TestNormalizeOperationAcceptsAllKinds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		op   Operation
	}{
		{
			name: "CreateFile",
			op: Operation{
				Kind: OperationKindCreateFile,
				Path: "/file.txt",
				Mode: 0o600,
				Data: []byte("data"),
			},
		},
		{
			name: "CreateDir",
			op: Operation{
				Kind: OperationKindCreateDir,
				Path: "/dir",
				Mode: 0o750,
			},
		},
		{
			name: "CreateSymlink",
			op: Operation{
				Kind:       OperationKindCreateSymlink,
				Path:       "/link",
				LinkTarget: "target.txt",
			},
		},
		{
			name: "CreateHardlink",
			op: Operation{
				Kind:       OperationKindCreateHardlink,
				Path:       "/link",
				SourcePath: "/source.txt",
			},
		},
		{
			name: "Delete",
			op: Operation{
				Kind: OperationKindDelete,
				Path: "/gone",
			},
		},
		{
			name: "Rename",
			op: Operation{
				Kind:        OperationKindRename,
				Path:        "/before",
				Destination: "/after",
			},
		},
		{
			name: "Chmod",
			op: Operation{
				Kind: OperationKindChmod,
				Path: "/file.txt",
				Mode: 0o640,
			},
		},
		{
			name: "Chown",
			op: Operation{
				Kind: OperationKindChown,
				Path: "/file.txt",
				UID:  1000,
				GID:  1000,
			},
		},
		{
			name: "Truncate",
			op: Operation{
				Kind: OperationKindTruncate,
				Path: "/file.txt",
				Size: 4,
			},
		},
		{
			name: "Append",
			op: Operation{
				Kind: OperationKindAppend,
				Path: "/file.txt",
				Data: []byte("tail"),
			},
		},
		{
			name: "OverwriteRange",
			op: Operation{
				Kind:   OperationKindOverwriteRange,
				Path:   "/file.txt",
				Offset: 1,
				Data:   []byte("xx"),
			},
		},
		{
			name: "SetXAttr",
			op: Operation{
				Kind:       OperationKindSetXAttr,
				Path:       "/file.txt",
				XAttrName:  "user.test",
				XAttrValue: []byte("value"),
			},
		},
		{
			name: "RemoveXAttr",
			op: Operation{
				Kind:      OperationKindRemoveXAttr,
				Path:      "/file.txt",
				XAttrName: "user.test",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			normalized, err := normalizeOperation(tt.op)
			require.NoError(t, err)
			require.Equal(t, tt.op.Kind, normalized.Kind)
		})
	}
}

func TestExportAndParseOperationSpecRoundTrip(t *testing.T) {
	t.Parallel()

	ops := []Operation{
		{
			Kind: OperationKindCreateFile,
			Path: "/alpha.txt",
			Mode: 0o600,
			Data: []byte("alpha"),
		},
		{
			Kind: OperationKindAppend,
			Path: "/alpha.txt",
			Data: []byte("-beta"),
		},
	}

	specA, err := ExportOperationSpec(ops)
	require.NoError(t, err)
	specB, err := ExportOperationSpec(ops)
	require.NoError(t, err)
	require.Equal(t, specA, specB)

	parsed, err := ParseOperationSpec(specA)
	require.NoError(t, err)
	require.Equal(t, ops, parsed)
}

func TestApplyOperationsTableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setup       func(t *testing.T, base string)
		op          Operation
		verify      func(t *testing.T, base string)
		errContains string
		errIs       error
	}{
		{
			name: "CreateFile",
			op: Operation{
				Kind: OperationKindCreateFile,
				Path: "/new.txt",
				Mode: 0o600,
				Data: []byte("created"),
			},
			verify: func(t *testing.T, base string) {
				data, err := os.ReadFile(filepath.Join(base, "new.txt"))
				require.NoError(t, err)
				require.Equal(t, "created", string(data))
			},
		},
		{
			name: "CreateDir",
			op: Operation{
				Kind: OperationKindCreateDir,
				Path: "/nested",
				Mode: 0o750,
			},
			verify: func(t *testing.T, base string) {
				info, err := os.Stat(filepath.Join(base, "nested"))
				require.NoError(t, err)
				require.True(t, info.IsDir())
			},
		},
		{
			name: "CreateSymlink",
			setup: func(t *testing.T, base string) {
				requireMutationSymlinkSupport(t)
				require.NoError(t, os.WriteFile(filepath.Join(base, "target.txt"), []byte("target"), 0o600))
			},
			op: Operation{
				Kind:       OperationKindCreateSymlink,
				Path:       "/link.txt",
				LinkTarget: "target.txt",
			},
			verify: func(t *testing.T, base string) {
				target, err := os.Readlink(filepath.Join(base, "link.txt"))
				require.NoError(t, err)
				require.Equal(t, "target.txt", target)
			},
		},
		{
			name: "CreateHardlink",
			setup: func(t *testing.T, base string) {
				requireMutationHardlinkSupport(t)
				require.NoError(t, os.WriteFile(filepath.Join(base, "source.txt"), []byte("src"), 0o600))
			},
			op: Operation{
				Kind:       OperationKindCreateHardlink,
				Path:       "/linked.txt",
				SourcePath: "/source.txt",
			},
			verify: func(t *testing.T, base string) {
				sourceInfo, err := os.Stat(filepath.Join(base, "source.txt"))
				require.NoError(t, err)
				linkInfo, err := os.Stat(filepath.Join(base, "linked.txt"))
				require.NoError(t, err)
				require.True(t, os.SameFile(sourceInfo, linkInfo))
			},
		},
		{
			name: "Delete",
			setup: func(t *testing.T, base string) {
				require.NoError(t, os.WriteFile(filepath.Join(base, "delete.txt"), []byte("x"), 0o600))
			},
			op: Operation{
				Kind: OperationKindDelete,
				Path: "/delete.txt",
			},
			verify: func(t *testing.T, base string) {
				_, err := os.Stat(filepath.Join(base, "delete.txt"))
				require.ErrorIs(t, err, os.ErrNotExist)
			},
		},
		{
			name: "Rename",
			setup: func(t *testing.T, base string) {
				require.NoError(t, os.WriteFile(filepath.Join(base, "before.txt"), []byte("renamed"), 0o600))
			},
			op: Operation{
				Kind:        OperationKindRename,
				Path:        "/before.txt",
				Destination: "/after.txt",
			},
			verify: func(t *testing.T, base string) {
				_, err := os.Stat(filepath.Join(base, "before.txt"))
				require.ErrorIs(t, err, os.ErrNotExist)
				data, err := os.ReadFile(filepath.Join(base, "after.txt"))
				require.NoError(t, err)
				require.Equal(t, "renamed", string(data))
			},
		},
		{
			name: "Chmod",
			setup: func(t *testing.T, base string) {
				require.NoError(t, os.WriteFile(filepath.Join(base, "chmod.txt"), []byte("x"), 0o600))
			},
			op: Operation{
				Kind: OperationKindChmod,
				Path: "/chmod.txt",
				Mode: 0o640,
			},
			verify: func(t *testing.T, base string) {
				info, err := os.Stat(filepath.Join(base, "chmod.txt"))
				require.NoError(t, err)
				if runtime.GOOS == "windows" {
					require.NotZero(t, info.Mode().Perm()&0o200)
					return
				}

				require.Equal(t, os.FileMode(0o640), info.Mode().Perm())
			},
		},
		{
			name: "Chown",
			setup: func(t *testing.T, base string) {
				requireMutationChownSupport(t)
				require.NoError(t, os.WriteFile(filepath.Join(base, "chown.txt"), []byte("x"), 0o600))
			},
			op: Operation{
				Kind: OperationKindChown,
				Path: "/chown.txt",
				UID:  os.Getuid(),
				GID:  os.Getgid(),
			},
		},
		{
			name: "Truncate",
			setup: func(t *testing.T, base string) {
				require.NoError(t, os.WriteFile(filepath.Join(base, "truncate.txt"), []byte("truncate"), 0o600))
			},
			op: Operation{
				Kind: OperationKindTruncate,
				Path: "/truncate.txt",
				Size: 3,
			},
			verify: func(t *testing.T, base string) {
				info, err := os.Stat(filepath.Join(base, "truncate.txt"))
				require.NoError(t, err)
				require.EqualValues(t, 3, info.Size())
			},
		},
		{
			name: "Append",
			setup: func(t *testing.T, base string) {
				require.NoError(t, os.WriteFile(filepath.Join(base, "append.txt"), []byte("ab"), 0o600))
			},
			op: Operation{
				Kind: OperationKindAppend,
				Path: "/append.txt",
				Data: []byte("cd"),
			},
			verify: func(t *testing.T, base string) {
				data, err := os.ReadFile(filepath.Join(base, "append.txt"))
				require.NoError(t, err)
				require.Equal(t, "abcd", string(data))
			},
		},
		{
			name: "OverwriteRange",
			setup: func(t *testing.T, base string) {
				require.NoError(t, os.WriteFile(filepath.Join(base, "overwrite.txt"), []byte("abcdef"), 0o600))
			},
			op: Operation{
				Kind:   OperationKindOverwriteRange,
				Path:   "/overwrite.txt",
				Offset: 2,
				Data:   []byte("ZZ"),
			},
			verify: func(t *testing.T, base string) {
				data, err := os.ReadFile(filepath.Join(base, "overwrite.txt"))
				require.NoError(t, err)
				require.Equal(t, "abZZef", string(data))
			},
		},
		{
			name: "SetXAttr",
			setup: func(t *testing.T, base string) {
				requireMutationXAttrSupport(t)
				require.NoError(t, os.WriteFile(filepath.Join(base, "xattr.txt"), []byte("x"), 0o600))
			},
			op: Operation{
				Kind:       OperationKindSetXAttr,
				Path:       "/xattr.txt",
				XAttrName:  "user.test",
				XAttrValue: []byte("value"),
			},
			verify: func(t *testing.T, base string) {
				value, err := getPathXAttr(filepath.Join(base, "xattr.txt"), "user.test")
				require.NoError(t, err)
				require.Equal(t, []byte("value"), value)
			},
		},
		{
			name: "RemoveXAttr",
			setup: func(t *testing.T, base string) {
				requireMutationXAttrSupport(t)
				require.NoError(t, os.WriteFile(filepath.Join(base, "xattr.txt"), []byte("x"), 0o600))
				require.NoError(t, setPathXAttr(filepath.Join(base, "xattr.txt"), "user.test", []byte("value")))
			},
			op: Operation{
				Kind:      OperationKindRemoveXAttr,
				Path:      "/xattr.txt",
				XAttrName: "user.test",
			},
			verify: func(t *testing.T, base string) {
				names, err := listPathXAttrNames(filepath.Join(base, "xattr.txt"))
				require.NoError(t, err)
				require.NotContains(t, names, "user.test")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			base := t.TempDir()
			if tt.setup != nil {
				tt.setup(t, base)
			}

			err := ApplyOperations(base, []Operation{tt.op})
			if tt.errContains != "" || tt.errIs != nil {
				require.Error(t, err)
				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
				}
				if tt.errIs != nil {
					require.ErrorIs(t, err, tt.errIs)
				}

				var applyErr *OperationApplyError
				require.True(t, errors.As(err, &applyErr))
				require.Equal(t, 0, applyErr.Index)
				_, parseErr := ParseOperationSpec(applyErr.Spec)
				require.NoError(t, parseErr)

				return
			}

			require.NoError(t, err)
			if tt.verify != nil {
				tt.verify(t, base)
			}
		})
	}
}

func TestApplyOperationsReportsReplaySpecOnFailure(t *testing.T) {
	t.Parallel()

	base := t.TempDir()

	ops := []Operation{
		{
			Kind: OperationKindCreateFile,
			Path: "/file.txt",
			Mode: 0o600,
			Data: []byte("data"),
		},
		{
			Kind:        OperationKindRename,
			Path:        "/missing.txt",
			Destination: "/after.txt",
		},
	}

	err := ApplyOperations(base, ops)
	require.Error(t, err)

	var applyErr *OperationApplyError
	require.True(t, errors.As(err, &applyErr))
	require.Equal(t, 1, applyErr.Index)
	require.Contains(t, applyErr.Error(), "replay-spec=")

	parsed, parseErr := ParseOperationSpec(applyErr.Spec)
	require.NoError(t, parseErr)
	require.Equal(t, []Operation{
		{
			Kind: OperationKindCreateFile,
			Path: "/file.txt",
			Mode: 0o600,
			Data: []byte("data"),
		},
		{
			Kind:        OperationKindRename,
			Path:        "/missing.txt",
			Destination: "/after.txt",
		},
	}, parsed)
}

func TestApplyOperationsRejectsSymlinkBasePath(t *testing.T) {
	t.Parallel()
	requireMutationSymlinkSupport(t)

	root := t.TempDir()
	baseTarget := filepath.Join(root, "base-target")
	baseLink := filepath.Join(root, "base-link")

	require.NoError(t, os.MkdirAll(baseTarget, 0o750))
	require.NoError(t, os.Symlink(baseTarget, baseLink))

	err := ApplyOperations(baseLink, []Operation{{
		Kind: OperationKindCreateFile,
		Path: "/inside.txt",
		Mode: 0o600,
		Data: []byte("data"),
	}})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrBasePathSymlink)
}

func TestApplyOperationsRejectsSymlinkParentEscape(t *testing.T) {
	t.Parallel()
	requireMutationSymlinkSupport(t)

	base := t.TempDir()
	outside := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(base, "safe"), 0o750))
	require.NoError(t, os.Symlink(outside, filepath.Join(base, "safe", "escape")))

	err := ApplyOperations(base, []Operation{{
		Kind: OperationKindCreateFile,
		Path: "/safe/escape/leak.txt",
		Mode: 0o600,
		Data: []byte("data"),
	}})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrOperationPathSymlinkParent)

	_, statErr := os.Stat(filepath.Join(outside, "leak.txt"))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestMutationSentinelErrors(t *testing.T) {
	t.Parallel()

	_, err := ParseOperationSpec("  ")
	require.ErrorIs(t, err, ErrOperationSpecEmpty)

	_, err = GenerateOperations("", DefaultOperationGenerationOptions())
	require.ErrorIs(t, err, ErrBasePathEmpty)

	err = ApplyOperations("", []Operation{{
		Kind: OperationKindCreateFile,
		Path: "/ignored.txt",
		Mode: 0o600,
		Data: []byte("data"),
	}})
	require.ErrorIs(t, err, ErrBasePathEmpty)
}

func TestGenerateOperationsDeterministicForSameSeed(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(base, "dir"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(base, "dir", "seed.txt"), []byte("seed-data"), 0o600))

	opts := DefaultOperationGenerationOptions()
	opts.Seed = 42
	opts.Count = 20
	opts.MaxDataSize = 16
	opts.AllowedKinds = []OperationKind{
		OperationKindCreateFile,
		OperationKindCreateDir,
		OperationKindDelete,
		OperationKindRename,
		OperationKindChmod,
		OperationKindTruncate,
		OperationKindAppend,
		OperationKindOverwriteRange,
	}

	opsA, err := GenerateOperations(base, opts)
	require.NoError(t, err)
	opsB, err := GenerateOperations(base, opts)
	require.NoError(t, err)

	require.Equal(t, opsA, opsB)
}

func TestGenerateAndApplyOperationsIntegrationWithDiff(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	left := filepath.Join(base, "left")
	right := filepath.Join(base, "right")

	require.NoError(t, os.MkdirAll(filepath.Join(left, "sub"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(right, "sub"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(left, "sub", "seed.txt"), []byte("seed-content"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(right, "sub", "seed.txt"), []byte("seed-content"), 0o600))

	opts := DefaultOperationGenerationOptions()
	opts.Seed = 7
	opts.Count = 24
	opts.MaxDataSize = 12
	opts.AllowedKinds = []OperationKind{
		OperationKindCreateFile,
		OperationKindCreateDir,
		OperationKindDelete,
		OperationKindRename,
		OperationKindChmod,
		OperationKindTruncate,
		OperationKindAppend,
		OperationKindOverwriteRange,
	}

	ops, err := GenerateOperations(left, opts)
	require.NoError(t, err)

	require.NoError(t, ApplyOperations(left, ops))
	require.NoError(t, ApplyOperations(right, ops))

	fixedTime := time.Unix(1_700_000_000, 0)
	require.NoError(t, normalizeTreeMTime(left, fixedTime))
	require.NoError(t, normalizeTreeMTime(right, fixedTime))

	require.NoError(t, diff.PathsWithOptions(left, right, diff.DefaultOptions()))
}

func TestGeneratorMutationIntegration(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	require.NoError(t, os.MkdirAll(base, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(base, "seed.txt"), []byte("seed"), 0o600))

	g := New(base)

	opts := DefaultOperationGenerationOptions()
	opts.Seed = 5
	opts.Count = 4
	opts.MaxDataSize = 8
	opts.AllowedKinds = []OperationKind{
		OperationKindCreateFile,
		OperationKindAppend,
	}

	ops, err := g.GenerateOperations(opts)
	require.NoError(t, err)
	require.Len(t, ops, 4)
	require.NoError(t, g.ApplyOperations(ops))
}

func TestGeneratorMutationNilReceiver(t *testing.T) {
	t.Parallel()

	var g *Generator
	_, err := g.GenerateOperations(DefaultOperationGenerationOptions())
	require.ErrorIs(t, err, ErrNilGenerator)

	err = g.ApplyOperations(nil)
	require.ErrorIs(t, err, ErrNilGenerator)
}

func requireMutationSymlinkSupport(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")
	link := filepath.Join(tmpDir, "link.txt")

	require.NoError(t, os.WriteFile(target, []byte("data"), 0o600))
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported in this test environment: %s", err)
	}
}

func normalizeTreeMTime(basePath string, ts time.Time) error {
	return filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		return os.Chtimes(path, ts, ts)
	})
}

func requireMutationHardlinkSupport(t *testing.T) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("hardlink checks are not stable in this windows test environment")
	}

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")
	link := filepath.Join(tmpDir, "link.txt")

	require.NoError(t, os.WriteFile(target, []byte("data"), 0o600))
	if err := os.Link(target, link); err != nil {
		t.Skipf("hardlink not supported in this test environment: %s", err)
	}
}

func requireMutationChownSupport(t *testing.T) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("chown is not supported on windows")
	}

	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("data"), 0o600))

	if err := os.Lchown(target, os.Getuid(), os.Getgid()); err != nil {
		t.Skipf("chown not supported in this test environment: %s", err)
	}
}

func requireMutationXAttrSupport(t *testing.T) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("xattr operations are not supported on windows")
	}

	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "target.txt")
	require.NoError(t, os.WriteFile(target, []byte("data"), 0o600))

	if err := setPathXAttr(target, "user.probe", []byte("v")); err != nil {
		if errors.Is(err, ErrXAttrMetadataUnsupported) || errors.Is(err, ErrXAttrUnsupported) {
			t.Skipf("xattr not supported in this test environment: %s", err)
		}
		t.Skipf("xattr probe failed in this test environment: %s", err)
	}

	if err := removePathXAttr(target, "user.probe"); err != nil {
		if errors.Is(err, ErrXAttrMetadataUnsupported) || errors.Is(err, ErrXAttrUnsupported) {
			t.Skipf("xattr remove not supported in this test environment: %s", err)
		}
		t.Skipf("xattr remove probe failed in this test environment: %s", err)
	}
}
