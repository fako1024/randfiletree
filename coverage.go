package randfiletree

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"
)

// DeterministicCoverageEffort selects the per-cell repetition multiplier for
// the deterministic coverage scenario. Higher levels do not add new
// dimensions; they only multiply the number of entries that exercise each
// pairwise tuple.
type DeterministicCoverageEffort uint8

const (
	DeterministicCoverageEffortLow DeterministicCoverageEffort = iota + 1
	DeterministicCoverageEffortMedium
	DeterministicCoverageEffortHigh
	DeterministicCoverageEffortXHigh
)

func (e DeterministicCoverageEffort) String() string {
	switch e {
	case DeterministicCoverageEffortLow:
		return "low"
	case DeterministicCoverageEffortMedium:
		return "medium"
	case DeterministicCoverageEffortHigh:
		return "high"
	case DeterministicCoverageEffortXHigh:
		return "xhigh"
	default:
		return fmt.Sprintf("unknown(%d)", e)
	}
}

// DeterministicCoverageOptions configures a deterministic coverage run. The
// zero value is invalid (Effort defaults must be selected explicitly).
type DeterministicCoverageOptions struct {
	Effort DeterministicCoverageEffort

	// IncludeLinuxOnly enables dimensions that require Linux kernel
	// features (special files, ownership/timestamp metadata, xattr, ACL).
	// Defaults to runtime.GOOS == "linux" via
	// normalizeDeterministicCoverageOptions.
	IncludeLinuxOnly bool

	// IncludePrivileged enables dimensions that typically require root or
	// equivalent capabilities (char/block device nodes, trusted.* and
	// security.* xattrs, ownership changes to non-effective uid/gid).
	IncludePrivileged bool

	// PlanEntryLimit overrides the planning entry ceiling. 0 means use the
	// package default (100000).
	PlanEntryLimit int

	// RunMode selects how the plan is applied (Append/Strict/Replace).
	// Zero value is RunModeAppend.
	RunMode RunMode
}

// DeterministicCoverageSpec describes what a coverage run plans to do without
// applying it.
type DeterministicCoverageSpec struct {
	Effort            DeterministicCoverageEffort
	BasePath          string
	EnabledDimensions []string
	SkippedDimensions []SkippedDimension
	PlannedEntries    int
	HardlinkGroups    int
	GeneratedAt       time.Time
}

var (
	ErrDeterministicCoverageEffortInvalid = errors.New("deterministic coverage effort is invalid")
	ErrDeterministicCoverageBasePathEmpty = errors.New("deterministic coverage base path must not be empty")
)

// BuildDeterministicCoverageSpec returns the spec for the given options
// without touching the filesystem. The spec records what would be planned and
// which dimensions were skipped because of capability gates.
func BuildDeterministicCoverageSpec(basePath string, opts DeterministicCoverageOptions) (DeterministicCoverageSpec, error) {
	if basePath == "" {
		return DeterministicCoverageSpec{}, ErrDeterministicCoverageBasePathEmpty
	}
	if err := validateDeterministicCoverageEffort(opts.Effort); err != nil {
		return DeterministicCoverageSpec{}, err
	}

	opts = normalizeDeterministicCoverageOptions(opts)

	plan, caps, err := enumerateCoveragePlan(basePath, opts)
	if err != nil {
		return DeterministicCoverageSpec{}, err
	}

	return DeterministicCoverageSpec{
		Effort:            opts.Effort,
		BasePath:          basePath,
		EnabledDimensions: coverageCapabilityEnabledNames(caps),
		SkippedDimensions: coverageCapabilitySkipped(caps),
		PlannedEntries:    len(plan.entries),
		HardlinkGroups:    len(plan.hardlinkGroups),
		GeneratedAt:       coverageEpoch,
	}, nil
}

// RunDeterministicCoverage builds the deterministic coverage plan and applies
// it to basePath, returning the resolved spec and execution metrics. The plan
// is byte-for-byte identical for identical (basePath, opts) pairs.
//
// Determinism contract: no rand.Rand, no time.Now, no map-iteration order is
// involved in plan synthesis. Two calls with the same arguments produce
// identical entries, identical contents, and identical metadata payloads.
func RunDeterministicCoverage(basePath string, opts DeterministicCoverageOptions) (DeterministicCoverageSpec, RunMetrics, error) {
	if basePath == "" {
		return DeterministicCoverageSpec{}, RunMetrics{}, ErrDeterministicCoverageBasePathEmpty
	}
	if err := validateDeterministicCoverageEffort(opts.Effort); err != nil {
		return DeterministicCoverageSpec{}, RunMetrics{}, err
	}

	opts = normalizeDeterministicCoverageOptions(opts)
	if err := validateRunMode(opts.RunMode); err != nil {
		return DeterministicCoverageSpec{}, RunMetrics{}, err
	}

	runStart := time.Now()

	plan, caps, err := enumerateCoveragePlan(basePath, opts)
	if err != nil {
		return DeterministicCoverageSpec{}, RunMetrics{Elapsed: time.Since(runStart)}, err
	}

	planLimit := opts.PlanEntryLimit
	if planLimit <= 0 {
		planLimit = defaultPlanEntryLimit
	}
	if len(plan.entries) > planLimit {
		return DeterministicCoverageSpec{}, RunMetrics{Elapsed: time.Since(runStart)}, fmt.Errorf("%w: planned=%d limit=%d", ErrPlanEntryLimitExceeded, len(plan.entries), planLimit)
	}

	g := New(basePath)
	g.runMode = opts.RunMode
	g.planEntryLimit = planLimit

	applyStats, applyErr := g.applyPrebuiltPlan(plan)

	metrics := RunMetrics{
		Nodes:                len(plan.entries),
		HardlinkGroups:       len(plan.hardlinkGroups),
		AppliedEntries:       applyStats.appliedEntries,
		FinalizedDirectories: applyStats.finalizedDirectories,
		ApplyElapsed:         applyStats.elapsed,
		Elapsed:              time.Since(runStart),
	}

	spec := DeterministicCoverageSpec{
		Effort:            opts.Effort,
		BasePath:          basePath,
		EnabledDimensions: coverageCapabilityEnabledNames(caps),
		SkippedDimensions: coverageCapabilitySkipped(caps),
		PlannedEntries:    len(plan.entries),
		HardlinkGroups:    len(plan.hardlinkGroups),
		GeneratedAt:       coverageEpoch,
	}

	if applyErr != nil {
		return spec, metrics, applyErr
	}

	return spec, metrics, nil
}

// normalizeDeterministicCoverageOptions fills in default values that depend on
// the host environment (defaults to Linux capability inclusion when on Linux,
// off otherwise) without overriding caller-supplied values.
func normalizeDeterministicCoverageOptions(opts DeterministicCoverageOptions) DeterministicCoverageOptions {
	// Force IncludeLinuxOnly to false on non-Linux hosts since the
	// underlying helpers always fail there; if the caller explicitly opted
	// in, they will receive structured unsupported errors at apply time.
	if runtime.GOOS != "linux" {
		opts.IncludeLinuxOnly = false
		opts.IncludePrivileged = false
	} else if !opts.IncludeLinuxOnly && !opts.IncludePrivileged {
		// On Linux with neither flag set the call almost certainly wants
		// Linux capabilities; treat the default as IncludeLinuxOnly=true
		// so the run exercises the full Linux-only matrix.
		opts.IncludeLinuxOnly = true
	}

	return opts
}

// coverageCurrentUID returns the effective uid of the running process, or 0
// on platforms where the call is not meaningful.
func coverageCurrentUID() int {
	if uid := os.Geteuid(); uid >= 0 {
		return uid
	}
	return 0
}

// coverageCurrentGID returns the effective gid of the running process, or 0
// on platforms where the call is not meaningful.
func coverageCurrentGID() int {
	if gid := os.Getegid(); gid >= 0 {
		return gid
	}
	return 0
}

// runCoverage executes the coverage path on behalf of (*Generator).Run when
// the WithDeterministicCoverage option has been applied.
func (g *Generator) runCoverage(runStart time.Time) (RunMetrics, error) {
	opts := normalizeDeterministicCoverageOptions(g.coverageOptions)
	if err := validateDeterministicCoverageEffort(opts.Effort); err != nil {
		return RunMetrics{Elapsed: time.Since(runStart)}, err
	}

	// Honor the generator's own runMode/planEntryLimit unless the coverage
	// options explicitly override them.
	if opts.RunMode != g.runMode && opts.RunMode == 0 {
		opts.RunMode = g.runMode
	}
	if opts.PlanEntryLimit <= 0 {
		opts.PlanEntryLimit = g.planEntryLimit
	}

	plan, _, err := enumerateCoveragePlan(g.basePath, opts)
	if err != nil {
		return RunMetrics{Elapsed: time.Since(runStart)}, err
	}

	planLimit := opts.PlanEntryLimit
	if planLimit <= 0 {
		planLimit = defaultPlanEntryLimit
	}
	if len(plan.entries) > planLimit {
		return RunMetrics{Elapsed: time.Since(runStart)}, fmt.Errorf("%w: planned=%d limit=%d", ErrPlanEntryLimitExceeded, len(plan.entries), planLimit)
	}

	previousRunMode := g.runMode
	previousLimit := g.planEntryLimit
	g.runMode = opts.RunMode
	g.planEntryLimit = planLimit
	defer func() {
		g.runMode = previousRunMode
		g.planEntryLimit = previousLimit
	}()

	applyStats, applyErr := g.applyPrebuiltPlan(plan)
	metrics := RunMetrics{
		Nodes:                len(plan.entries),
		HardlinkGroups:       len(plan.hardlinkGroups),
		AppliedEntries:       applyStats.appliedEntries,
		FinalizedDirectories: applyStats.finalizedDirectories,
		ApplyElapsed:         applyStats.elapsed,
		Elapsed:              time.Since(runStart),
	}

	if applyErr != nil {
		return metrics, applyErr
	}

	return metrics, nil
}
