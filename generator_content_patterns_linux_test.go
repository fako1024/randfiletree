//go:build linux

package randfiletree

import (
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestRunContentPatternConfigurationIncomplete(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := newSpecialConfiguredGenerator(t, base, 1,
		WithContentPattern(ContentPatternSparseHoles),
	)

	err := g.Run()
	require.Error(t, err)
	require.ErrorContains(t, err, ErrContentPatternConfigurationIncomplete.Error())
}

func TestRunContentPatternRepeatedBlocksRejectsEmptyBlock(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := newSpecialConfiguredGenerator(t, base, 1,
		WithContentPattern(ContentPatternRepeatedBlocks),
		WithContentLogicalSize(256),
		WithDataGenerator(DataGeneratorFixed(nil)),
	)

	err := g.Run()
	require.ErrorIs(t, err, ErrContentPatternRepeatedBlockEmpty)
}

func TestRunContentPatternSparseParityAndSize(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := newSpecialConfiguredGenerator(t, base, 1,
		WithSeed(33),
		WithContentPattern(ContentPatternSparseHoles),
		WithContentLogicalSize(4*1024*1024),
	)

	require.NoError(t, g.Run())

	path := filepath.Join(base, "n00")

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.EqualValues(t, 4*1024*1024, info.Size())

	var stat unix.Stat_t
	require.NoError(t, unix.Lstat(path, &stat))
	require.Less(t, stat.Blocks*512, stat.Size)
}

func TestRunContentPatternPartialRangeOverwriteDeterministic(t *testing.T) {
	t.Parallel()

	left := filepath.Join(t.TempDir(), "left")
	right := filepath.Join(t.TempDir(), "right")

	configure := func(base string) *Generator {
		g := newSpecialConfiguredGenerator(t, base, 1,
			WithSeed(77),
			WithContentPattern(ContentPatternPartialRangeOverwrite),
			WithContentLogicalSize(1*1024*1024),
		)

		return g
	}

	require.NoError(t, configure(left).Run())
	require.NoError(t, configure(right).Run())

	leftData, err := os.ReadFile(filepath.Join(left, "n00"))
	require.NoError(t, err)
	rightData, err := os.ReadFile(filepath.Join(right, "n00"))
	require.NoError(t, err)

	require.Equal(t, leftData, rightData)
}

func TestRunContentPatternLargeSparseMemoryBounded(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := newSpecialConfiguredGenerator(t, base, 1,
		WithSeed(101),
		WithContentPatternGenerator(func(r *rand.Rand) ContentPattern {
			return ContentPatternSparseHoles
		}),
		WithContentLogicalSize(128*1024*1024+1),
	)

	require.NoError(t, g.Run())

	path := filepath.Join(base, "n00")
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.EqualValues(t, 128*1024*1024+1, info.Size())

	var stat unix.Stat_t
	require.NoError(t, unix.Lstat(path, &stat))
	require.Less(t, stat.Blocks*512, stat.Size)
}

func TestRunContentPatternSparseCapabilityAwareSkip(t *testing.T) {
	t.Parallel()

	base := filepath.Join(t.TempDir(), "tree")
	g := newSpecialConfiguredGenerator(t, base, 1,
		WithContentPattern(ContentPatternSparseHoles),
		WithContentLogicalSize(4096),
	)

	err := g.Run()
	if err == nil {
		return
	}

	if errors.Is(err, os.ErrPermission) {
		t.Skipf("sparse file write unavailable in this test environment: %v", err)
	}

	require.NoError(t, err)
}
