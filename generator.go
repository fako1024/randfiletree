package randfiletree

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultSeed           = 1
	defaultPlanEntryLimit = 100000
)

// Generator denotes a filetree generator.
//
// A Generator is not safe for concurrent use. The embedded random source
// (rndSrc) and the plan/apply state mutate in place; callers that want to
// run multiple generators in parallel must instantiate one per goroutine.
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

	contentPatternGen     ContentPatternGenerator
	contentLogicalSizeGen NumberGenerator

	ownershipUIDGen NumberGenerator
	ownershipGIDGen NumberGenerator

	atimeGen TimestampGenerator
	mtimeGen TimestampGenerator

	xattrValueGens              []xattrValueGeneratorConfig
	xattrAllowTrustedNamespace  bool
	xattrAllowSecurityNamespace bool

	aclEntriesGen ACLGenerator

	symlinkProbGen     BooleanGenerator
	symlinkRelProbGen  BooleanGenerator
	hardlinkProbGen    BooleanGenerator
	specialFileProbGen BooleanGenerator

	symlinkStrategyGen SymlinkStrategyGenerator
	specialFileTypeGen SpecialFileTypeGenerator

	specialDeviceMajorGen NumberGenerator
	specialDeviceMinorGen NumberGenerator

	byteFileNameGen ByteNameGenerator
	byteDirNameGen  ByteNameGenerator

	seed int64

	rndSrc  *rand.Rand
	runMode RunMode

	planEntryLimit int
}

// New instantiates a new generator
func New(basePath string) *Generator {
	return &Generator{
		basePath: basePath,

		/* #nosec G404 */
		seed:           defaultSeed,
		rndSrc:         rand.New(rand.NewSource(defaultSeed)),
		runMode:        RunModeAppend,
		planEntryLimit: defaultPlanEntryLimit,
	}
}

// Run generates a new tree (or adds to an existing one) according to the defined rules
func (g *Generator) Run() error {
	return g.RunWithOptions(RunOptions{})
}

// RunOptions defines optional deterministic execution behavior for Run.
type RunOptions struct {
	// FaultProfile injects deterministic failures at matching execution points.
	FaultProfile FaultProfile
}

func (o RunOptions) validate() error {
	if err := o.FaultProfile.validate(); err != nil {
		return fmt.Errorf("invalid run fault profile: %w", err)
	}

	return nil
}

// RunWithOptions generates a new tree according to the defined rules and execution options.
func (g *Generator) RunWithOptions(opts RunOptions) error {
	_, err := g.RunWithMetrics(opts)

	return err
}

// RunMetrics denotes deterministic planning/apply summary metrics for one generator run.
type RunMetrics struct {
	Nodes      int
	Retries    int
	Collisions int

	HardlinkGroups       int
	AppliedEntries       int
	FinalizedDirectories int

	PlanningElapsed time.Duration
	ApplyElapsed    time.Duration
	Elapsed         time.Duration
}

// RunWithMetrics generates a new tree and returns execution metrics for diagnostics.
func (g *Generator) RunWithMetrics(opts RunOptions) (RunMetrics, error) {
	if g == nil {
		return RunMetrics{}, ErrNilGenerator
	}

	if err := opts.validate(); err != nil {
		return RunMetrics{}, err
	}

	runStart := time.Now()

	if g.hasNoConfiguration() {
		return RunMetrics{Elapsed: time.Since(runStart)}, nil
	}

	if err := g.validateRunConfiguration(); err != nil {
		return RunMetrics{Elapsed: time.Since(runStart)}, err
	}

	planStart := time.Now()
	plan, err := g.planRun()
	metrics := RunMetrics{
		Nodes:          len(plan.entries),
		Retries:        plan.metrics.pathRetries,
		Collisions:     plan.metrics.pathCollisions,
		HardlinkGroups: len(plan.hardlinkGroups),
	}
	metrics.PlanningElapsed = time.Since(planStart)
	if err != nil {
		metrics.Elapsed = time.Since(runStart)

		return metrics, err
	}

	execCtx, err := newExecutionContext(opts.FaultProfile)
	if err != nil {
		metrics.Elapsed = time.Since(runStart)

		return metrics, err
	}

	applyStats, err := g.applyRunPlan(plan, execCtx)
	metrics.ApplyElapsed = applyStats.elapsed
	metrics.AppliedEntries = applyStats.appliedEntries
	metrics.FinalizedDirectories = applyStats.finalizedDirectories
	metrics.Elapsed = time.Since(runStart)
	if err != nil {
		return metrics, err
	}

	return metrics, nil
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
	return g.ApplyOperationsWithOptions(ops, OperationApplyOptions{})
}

// ApplyOperationsWithOptions applies operation streams against the generator base path.
func (g *Generator) ApplyOperationsWithOptions(ops []Operation, opts OperationApplyOptions) error {
	if g == nil {
		return ErrNilGenerator
	}

	return ApplyOperationsWithOptions(g.basePath, ops, opts)
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
		g.contentPatternGen == nil &&
		g.contentLogicalSizeGen == nil &&
		g.ownershipUIDGen == nil &&
		g.ownershipGIDGen == nil &&
		g.atimeGen == nil &&
		g.mtimeGen == nil &&
		g.symlinkProbGen == nil &&
		g.symlinkRelProbGen == nil &&
		g.hardlinkProbGen == nil &&
		g.specialFileProbGen == nil &&
		g.symlinkStrategyGen == nil &&
		g.specialFileTypeGen == nil &&
		g.specialDeviceMajorGen == nil &&
		g.specialDeviceMinorGen == nil &&
		g.byteFileNameGen == nil &&
		g.byteDirNameGen == nil &&
		len(g.xattrValueGens) == 0 &&
		g.aclEntriesGen == nil
}

func (g *Generator) validateRunConfiguration() error {
	missing := make([]string, 0, 20)

	if g.rndSrc == nil {
		missing = append(missing, "random source")
	}
	if g.dirNameGen == nil && g.byteDirNameGen == nil {
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
	if g.fileNameGen == nil && g.byteFileNameGen == nil {
		missing = append(missing, "file name generator")
	}
	if g.fileNameLenGen == nil {
		missing = append(missing, "file name length generator")
	}
	if g.fileModeGen == nil {
		missing = append(missing, "file mode generator")
	}
	if g.dataGen == nil && g.contentPatternGen == nil {
		missing = append(missing, "data generator")
	}
	if (g.contentPatternGen == nil) != (g.contentLogicalSizeGen == nil) {
		missing = append(missing, ErrContentPatternConfigurationIncomplete.Error())
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
	if (g.specialFileProbGen == nil) != (g.specialFileTypeGen == nil) {
		missing = append(missing, "special file generator and special file type generator")
	}
	if (g.specialDeviceMajorGen == nil) != (g.specialDeviceMinorGen == nil) {
		missing = append(missing, ErrSpecialDeviceConfigurationIncomplete.Error())
	}
	if err := g.validateXAttrConfiguration(); err != nil {
		missing = append(missing, err.Error())
	}
	if err := validateRunMode(g.runMode); err != nil {
		missing = append(missing, err.Error())
	}
	if g.planEntryLimit <= 0 {
		missing = append(missing, ErrPlanEntryLimitInvalid.Error())
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

func (g *Generator) shouldPlanSpecialFile(r *rand.Rand) bool {
	if g.specialFileProbGen == nil {
		return false
	}

	return g.specialFileProbGen(r)
}

func (g *Generator) nextSpecialFileType(r *rand.Rand) SpecialFileType {
	if g.specialFileTypeGen == nil {
		return 0
	}

	return g.specialFileTypeGen(r)
}

func (g *Generator) hasContentPatternConfiguration() bool {
	return g.contentPatternGen != nil && g.contentLogicalSizeGen != nil
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
