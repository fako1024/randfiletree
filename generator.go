package randfiletree

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultSeed = 1
)

// Generator denotes a filetree generator
type Generator struct {
	basePath string

	dirNameGen    FileNameGenerator
	dirNameLenGen FileNameLenGenerator
	dirModeGen    FileModeGenerator

	nFilesInDirGen NumberGenerator
	nDirsInDirGen  NumberGenerator

	fileNameGen    FileNameGenerator
	fileNameLenGen FileNameLenGenerator
	fileModeGen    FileModeGenerator

	dataGen      DataGenerator
	pathDepthGen NumberGenerator

	ownershipUIDGen NumberGenerator
	ownershipGIDGen NumberGenerator

	atimeGen TimestampGenerator
	mtimeGen TimestampGenerator

	symlinkProbGen    BooleanGenerator
	symlinkRelProbGen BooleanGenerator
	hardlinkProbGen   BooleanGenerator

	symlinkStrategyGen SymlinkStrategyGenerator

	rndSrc  *rand.Rand
	runMode RunMode
}

// New instantiates a new generator
func New(basePath string) *Generator {
	return &Generator{
		basePath: basePath,

		/* #nosec G404 */
		rndSrc:  rand.New(rand.NewSource(defaultSeed)),
		runMode: RunModeAppend,
	}
}

// Run generates a new tree (or adds to an existing one) according to the defined rules
func (g *Generator) Run() error {
	if g == nil {
		return ErrNilGenerator
	}

	if g.hasNoConfiguration() {
		return nil
	}

	if err := g.validateRunConfiguration(); err != nil {
		return err
	}

	plan, err := g.planRun()
	if err != nil {
		return err
	}

	if err := g.applyRunPlan(plan); err != nil {
		return err
	}

	return nil
}

// GenerateOperations creates a deterministic operation stream against the generator base path.
func (g *Generator) GenerateOperations(opts OperationGenerationOptions) ([]Operation, error) {
	if g == nil {
		return nil, ErrNilGenerator
	}

	return GenerateOperations(g.basePath, opts)
}

// ApplyOperations applies operation streams against the generator base path.
func (g *Generator) ApplyOperations(ops []Operation) error {
	if g == nil {
		return ErrNilGenerator
	}

	return ApplyOperations(g.basePath, ops)
}

func (g *Generator) hasNoConfiguration() bool {
	return g.dirNameGen == nil &&
		g.dirNameLenGen == nil &&
		g.dirModeGen == nil &&
		g.nFilesInDirGen == nil &&
		g.nDirsInDirGen == nil &&
		g.fileNameGen == nil &&
		g.fileNameLenGen == nil &&
		g.fileModeGen == nil &&
		g.dataGen == nil &&
		g.pathDepthGen == nil &&
		g.ownershipUIDGen == nil &&
		g.ownershipGIDGen == nil &&
		g.atimeGen == nil &&
		g.mtimeGen == nil &&
		g.symlinkProbGen == nil &&
		g.symlinkRelProbGen == nil &&
		g.hardlinkProbGen == nil &&
		g.symlinkStrategyGen == nil
}

func (g *Generator) validateRunConfiguration() error {
	missing := make([]string, 0, 17)

	if g.rndSrc == nil {
		missing = append(missing, "random source")
	}
	if g.dirNameGen == nil {
		missing = append(missing, "directory name generator")
	}
	if g.dirNameLenGen == nil {
		missing = append(missing, "directory name length generator")
	}
	if g.dirModeGen == nil {
		missing = append(missing, "directory mode generator")
	}
	if g.nFilesInDirGen == nil {
		missing = append(missing, "files-per-directory generator")
	}
	if g.nDirsInDirGen == nil {
		missing = append(missing, "directories-per-directory generator")
	}
	if g.fileNameGen == nil {
		missing = append(missing, "file name generator")
	}
	if g.fileNameLenGen == nil {
		missing = append(missing, "file name length generator")
	}
	if g.fileModeGen == nil {
		missing = append(missing, "file mode generator")
	}
	if g.dataGen == nil {
		missing = append(missing, "data generator")
	}
	if g.pathDepthGen == nil {
		missing = append(missing, "path depth generator")
	}
	if (g.ownershipUIDGen == nil) != (g.ownershipGIDGen == nil) {
		missing = append(missing, ErrOwnershipMetadataConfigurationIncomplete.Error())
	}
	if (g.atimeGen == nil) != (g.mtimeGen == nil) {
		missing = append(missing, ErrTimestampMetadataConfigurationIncomplete.Error())
	}
	if g.symlinkProbGen == nil {
		missing = append(missing, "symlink generator")
	}
	if g.symlinkRelProbGen == nil && g.symlinkStrategyGen == nil {
		missing = append(missing, "relative symlink generator or symlink strategy generator")
	}
	if err := validateRunMode(g.runMode); err != nil {
		missing = append(missing, err.Error())
	}

	if len(missing) > 0 {
		return fmt.Errorf("generator configuration incomplete, missing: %s", strings.Join(missing, ", "))
	}

	return nil
}

func (g *Generator) shouldPlanHardlink(r *rand.Rand) bool {
	if g.hardlinkProbGen == nil {
		return false
	}

	return g.hardlinkProbGen(r)
}

func (g *Generator) hasExplicitSymlinkStrategy() bool {
	return g.symlinkStrategyGen != nil
}

func (g *Generator) nextSymlinkStrategy(r *rand.Rand) SymlinkStrategy {
	if g.symlinkStrategyGen != nil {
		return g.symlinkStrategyGen(r)
	}

	if g.symlinkRelProbGen != nil && g.symlinkRelProbGen(r) {
		return SymlinkStrategyRelative
	}

	return SymlinkStrategyAbsolute
}

// RemoveAll removes (and recreates) the directory
func (g *Generator) RemoveAll() error {
	return os.RemoveAll(g.basePath)
}

// Walk performs a recursive walk through the provided directory (wrapping filepath.Walk())
func (g *Generator) Walk(fn filepath.WalkFunc) error {
	return filepath.Walk(g.basePath, fn)
}

func (g *Generator) writeRelSymlink(dir, target string) error {
	if target == "" {
		return ErrEmptySymlinkTarget
	}

	path := filepath.Join(dir, g.fileNameGen(g.rndSrc, g.fileNameLenGen(g.rndSrc)))
	relTarget, err := filepath.Rel(dir, target)
	if err != nil {
		return fmt.Errorf("failed to derive relative symlink target for `%s`: %w", target, err)
	}

	// Check if the link already exists
	if _, err := os.Lstat(path); err == nil {
		return nil
	}

	return os.Symlink(relTarget, path)
}
