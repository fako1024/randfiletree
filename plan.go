package randfiletree

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
)

const (
	maxPlanPathCollisionRetries = 128
	defaultSymlinkCycleLength   = 2
)

var ErrPlanPathCollisionExhausted = errors.New("planned path collision retries exhausted")

type plannedEntryType uint8

const (
	plannedEntryTypeDir plannedEntryType = iota + 1
	plannedEntryTypeFile
	plannedEntryTypeSymlink
	plannedEntryTypeHardlink
	plannedEntryTypeSpecial
)

type plannedEntry struct {
	typeID plannedEntryType
	path   string

	mode uint32
	data []byte

	contentPattern plannedFileContent

	linkTarget string

	specialFileType SpecialFileType

	specialDeviceMajor int
	specialDeviceMinor int

	metadata metadataConfig
}

type runPlan struct {
	entries        []plannedEntry
	hardlinkGroups []plannedHardlinkGroup
}

type plannedHardlinkGroup struct {
	origin string
	paths  []string
}

type planState struct {
	rnd      *rand.Rand
	used     map[string]struct{}
	lastPath string

	filePaths []string

	symlinkPaths []string

	hardlinkGroups      []*plannedHardlinkGroup
	hardlinkGroupByPath map[string]*plannedHardlinkGroup
}

func (g *Generator) planRun() (runPlan, error) {
	state := planState{
		rnd: g.rndSrc,
		used: map[string]struct{}{
			g.basePath: {},
		},
		hardlinkGroupByPath: make(map[string]*plannedHardlinkGroup),
	}

	plan := runPlan{entries: make([]plannedEntry, 0, 16)}
	if err := g.planDir(g.basePath, 0, &state, &plan); err != nil {
		return runPlan{}, err
	}

	plan.hardlinkGroups = state.materializeHardlinkGroups()

	return plan, nil
}

func (g *Generator) planDir(path string, depth int, state *planState, plan *runPlan) error {
	depth++
	if depth > g.pathDepthGen(state.rnd) {
		return nil
	}

	plan.entries = append(plan.entries, plannedEntry{
		typeID:   plannedEntryTypeDir,
		path:     path,
		mode:     g.dirModeGen(state.rnd),
		metadata: metadataConfig{},
	})
	entryIndex := len(plan.entries) - 1

	metadata, err := g.resolveMetadata(state.rnd)
	if err != nil {
		return err
	}

	plan.entries[entryIndex].metadata = metadata

	nDirs := g.nDirsInDirGen(state.rnd)
	for i := 0; i < nDirs; i++ {
		dirPath, err := g.planUniquePath(path, state, g.dirNameGen, g.dirNameLenGen, "directory")
		if err != nil {
			return err
		}

		if err := g.planDir(dirPath, depth, state, plan); err != nil {
			return err
		}
	}

	nFiles := g.nFilesInDirGen(state.rnd)
	for i := 0; i < nFiles; i++ {
		plannedSymlink, err := g.tryPlanSymlink(path, state, plan)
		if err != nil {
			return err
		}
		if plannedSymlink {
			continue
		}

		plannedHardlink, err := g.tryPlanHardlink(path, state, plan)
		if err != nil {
			return err
		}
		if plannedHardlink {
			continue
		}

		plannedSpecial, err := g.tryPlanSpecialFile(path, state, plan)
		if err != nil {
			return err
		}
		if plannedSpecial {
			continue
		}

		if err := g.planFile(path, state, plan); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) tryPlanSymlink(dir string, state *planState, plan *runPlan) (bool, error) {
	if !g.symlinkProbGen(state.rnd) {
		return false, nil
	}

	if g.hasExplicitSymlinkStrategy() {
		strategy := g.nextSymlinkStrategy(state.rnd)
		if err := validateSymlinkStrategy(strategy); err != nil {
			return false, fmt.Errorf("invalid configured symlink strategy: %w", err)
		}

		return g.planSymlinkStrategy(dir, strategy, state, plan)
	}

	if state.lastPath == "" {
		return false, nil
	}

	if err := g.planSymlink(dir, state.lastPath, state, plan); err != nil {
		return false, err
	}

	return true, nil
}

func (g *Generator) planSymlinkStrategy(
	dir string,
	strategy SymlinkStrategy,
	state *planState,
	plan *runPlan,
) (bool, error) {
	switch strategy {
	case SymlinkStrategyAbsolute:
		targetPath, ok := state.pickFilePath(state.rnd)
		if !ok {
			return false, nil
		}

		if err := g.planSymlink(dir, targetPath, state, plan); err != nil {
			return false, err
		}

		return true, nil

	case SymlinkStrategyRelative:
		targetPath, ok := state.pickFilePath(state.rnd)
		if !ok {
			return false, nil
		}

		relTarget, err := filepath.Rel(dir, targetPath)
		if err != nil {
			return false, fmt.Errorf("failed to derive relative symlink target for `%s`: %w", targetPath, err)
		}

		if err := g.planSymlink(dir, relTarget, state, plan); err != nil {
			return false, err
		}

		return true, nil

	case SymlinkStrategyDangling:
		danglingTarget, err := g.planUniquePath(dir, state, g.fileNameGen, g.fileNameLenGen, "dangling symlink target")
		if err != nil {
			return false, err
		}

		if err := g.planSymlink(dir, danglingTarget, state, plan); err != nil {
			return false, err
		}

		return true, nil

	case SymlinkStrategySelfReferential:
		symlinkPath, err := g.planUniquePath(dir, state, g.fileNameGen, g.fileNameLenGen, "self-referential symlink")
		if err != nil {
			return false, err
		}

		g.appendSymlink(plan, state, symlinkPath, filepath.Base(symlinkPath))

		return true, nil

	case SymlinkStrategyChained:
		chainedTarget, ok := state.pickSymlinkPath(state.rnd)
		if !ok {
			return false, nil
		}

		if err := g.planSymlink(dir, chainedTarget, state, plan); err != nil {
			return false, err
		}

		return true, nil

	case SymlinkStrategyCycle:
		if err := g.planSymlinkCycle(dir, defaultSymlinkCycleLength, state, plan); err != nil {
			return false, err
		}

		return true, nil

	default:
		return false, fmt.Errorf("unsupported symlink strategy %d", strategy)
	}
}

func (g *Generator) planSymlinkCycle(dir string, length int, state *planState, plan *runPlan) error {
	if length < 2 {
		return fmt.Errorf("symlink cycle length must be >= 2, got %d", length)
	}

	cyclePaths := make([]string, 0, length)
	for i := 0; i < length; i++ {
		symlinkPath, err := g.planUniquePath(dir, state, g.fileNameGen, g.fileNameLenGen, "symlink cycle")
		if err != nil {
			return err
		}

		cyclePaths = append(cyclePaths, symlinkPath)
	}

	for i := range cyclePaths {
		path := cyclePaths[i]
		next := cyclePaths[(i+1)%len(cyclePaths)]

		relTarget, err := filepath.Rel(filepath.Dir(path), next)
		if err != nil {
			return fmt.Errorf("failed to derive cycle symlink target for `%s`: %w", path, err)
		}

		g.appendSymlink(plan, state, path, relTarget)
	}

	return nil
}

func (g *Generator) tryPlanHardlink(dir string, state *planState, plan *runPlan) (bool, error) {
	if !g.shouldPlanHardlink(state.rnd) {
		return false, nil
	}

	targetPath, ok := state.pickFilePath(state.rnd)
	if !ok {
		return false, nil
	}

	hardlinkPath, err := g.planUniquePath(dir, state, g.fileNameGen, g.fileNameLenGen, "hardlink")
	if err != nil {
		return false, err
	}

	plan.entries = append(plan.entries, plannedEntry{
		typeID:     plannedEntryTypeHardlink,
		path:       hardlinkPath,
		linkTarget: targetPath,
	})

	state.registerFilePath(hardlinkPath)
	state.lastPath = hardlinkPath
	state.registerHardlink(targetPath, hardlinkPath)

	return true, nil
}

func (g *Generator) tryPlanSpecialFile(dir string, state *planState, plan *runPlan) (bool, error) {
	if !g.shouldPlanSpecialFile(state.rnd) {
		return false, nil
	}

	fileType := g.nextSpecialFileType(state.rnd)
	if err := validateSpecialFileType(fileType); err != nil {
		return false, fmt.Errorf("invalid configured special file type: %w", err)
	}

	path, err := g.planUniquePath(dir, state, g.fileNameGen, g.fileNameLenGen, "special file")
	if err != nil {
		return false, err
	}

	entry := plannedEntry{
		typeID:          plannedEntryTypeSpecial,
		path:            path,
		mode:            g.fileModeGen(state.rnd),
		specialFileType: fileType,
	}

	if isSpecialDeviceType(fileType) {
		if g.specialDeviceMajorGen == nil || g.specialDeviceMinorGen == nil {
			return false, ErrSpecialDeviceNumbersRequired
		}

		entry.specialDeviceMajor = g.specialDeviceMajorGen(state.rnd)
		entry.specialDeviceMinor = g.specialDeviceMinorGen(state.rnd)

		if entry.specialDeviceMajor < 0 {
			return false, fmt.Errorf("special device major must be >= 0, got %d", entry.specialDeviceMajor)
		}
		if entry.specialDeviceMinor < 0 {
			return false, fmt.Errorf("special device minor must be >= 0, got %d", entry.specialDeviceMinor)
		}
	}

	plan.entries = append(plan.entries, entry)
	state.lastPath = path

	return true, nil
}

func (g *Generator) planFile(dir string, state *planState, plan *runPlan) error {
	filePath, err := g.planUniquePath(dir, state, g.fileNameGen, g.fileNameLenGen, "file")
	if err != nil {
		return err
	}

	entry := plannedEntry{
		typeID: plannedEntryTypeFile,
		path:   filePath,
		mode:   g.fileModeGen(state.rnd),
	}

	if g.hasContentPatternConfiguration() {
		contentPattern, err := g.planFileContentPattern(state.rnd)
		if err != nil {
			return fmt.Errorf("failed to generate file content pattern for `%s`: %w", filePath, err)
		}

		entry.contentPattern = contentPattern
	} else {
		data, err := g.dataGen(state.rnd)
		if err != nil {
			return fmt.Errorf("failed to generate file data for `%s`: %w", filePath, err)
		}

		entry.data = data
	}

	metadata, err := g.resolveMetadata(state.rnd)
	if err != nil {
		return err
	}
	entry.metadata = metadata

	plan.entries = append(plan.entries, entry)
	state.registerFilePath(filePath)
	state.lastPath = filePath

	if g.hasExplicitSymlinkStrategy() {
		return nil
	}
	if g.symlinkRelProbGen == nil || !g.symlinkRelProbGen(state.rnd) {
		return nil
	}

	relTarget, err := filepath.Rel(dir, filePath)
	if err != nil {
		return fmt.Errorf("failed to derive relative symlink target for `%s`: %w", filePath, err)
	}

	if err := g.planSymlink(dir, relTarget, state, plan); err != nil {
		return err
	}

	return nil
}

func (g *Generator) planSymlink(dir, target string, state *planState, plan *runPlan) error {
	symlinkPath, err := g.planUniquePath(dir, state, g.fileNameGen, g.fileNameLenGen, "symlink")
	if err != nil {
		return err
	}

	g.appendSymlink(plan, state, symlinkPath, target)

	return nil
}

func (g *Generator) appendSymlink(plan *runPlan, state *planState, path, target string) {
	plan.entries = append(plan.entries, plannedEntry{
		typeID:     plannedEntryTypeSymlink,
		path:       path,
		linkTarget: target,
	})
	state.registerSymlinkPath(path)
}

func (g *Generator) planUniquePath(
	dir string,
	state *planState,
	nameGen FileNameGenerator,
	lenGen NumberGenerator,
	entryKind string,
) (string, error) {
	for attempt := 1; attempt <= maxPlanPathCollisionRetries; attempt++ {
		path := filepath.Join(dir, nameGen(state.rnd, lenGen(state.rnd)))
		if _, exists := state.used[path]; exists {
			continue
		}

		state.used[path] = struct{}{}

		return path, nil
	}

	return "", fmt.Errorf(
		"%w for %s in directory `%s` after %d attempts",
		ErrPlanPathCollisionExhausted,
		entryKind,
		dir,
		maxPlanPathCollisionRetries,
	)
}

func (state *planState) registerFilePath(path string) {
	state.filePaths = append(state.filePaths, path)
}

func (state *planState) registerSymlinkPath(path string) {
	state.symlinkPaths = append(state.symlinkPaths, path)
}

func (state *planState) pickFilePath(r *rand.Rand) (string, bool) {
	if len(state.filePaths) == 0 {
		return "", false
	}

	return state.filePaths[r.Intn(len(state.filePaths))], true
}

func (state *planState) pickSymlinkPath(r *rand.Rand) (string, bool) {
	if len(state.symlinkPaths) == 0 {
		return "", false
	}

	return state.symlinkPaths[r.Intn(len(state.symlinkPaths))], true
}

func (state *planState) registerHardlink(targetPath, hardlinkPath string) {
	group := state.hardlinkGroupByPath[targetPath]
	if group == nil {
		group = &plannedHardlinkGroup{
			origin: targetPath,
			paths:  []string{targetPath},
		}
		state.hardlinkGroups = append(state.hardlinkGroups, group)
		state.hardlinkGroupByPath[targetPath] = group
	}

	group.paths = append(group.paths, hardlinkPath)
	state.hardlinkGroupByPath[hardlinkPath] = group
}

func (state *planState) materializeHardlinkGroups() []plannedHardlinkGroup {
	if len(state.hardlinkGroups) == 0 {
		return nil
	}

	groups := make([]plannedHardlinkGroup, 0, len(state.hardlinkGroups))
	for _, group := range state.hardlinkGroups {
		paths := append([]string(nil), group.paths...)
		sort.Strings(paths)

		groups = append(groups, plannedHardlinkGroup{
			origin: group.origin,
			paths:  paths,
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].origin < groups[j].origin
	})

	return groups
}

func (g *Generator) applyRunPlan(plan runPlan) error {
	switch g.runMode {
	case RunModeAppend:
		// no pre-step required
	case RunModeStrict:
		// no pre-step required
	case RunModeReplace:
		if err := os.RemoveAll(g.basePath); err != nil {
			return fmt.Errorf("failed to clear base path `%s`: %w", g.basePath, err)
		}
	default:
		return validateRunMode(g.runMode)
	}

	createdDirs := make(map[string]plannedEntry)

	for _, entry := range plan.entries {
		created, err := g.applyPlannedEntry(entry)
		if err != nil {
			return err
		}

		if created && entry.typeID == plannedEntryTypeDir {
			createdDirs[entry.path] = entry
		}
	}

	for i := len(plan.entries) - 1; i >= 0; i-- {
		entry := plan.entries[i]
		if entry.typeID != plannedEntryTypeDir {
			continue
		}
		if _, ok := createdDirs[entry.path]; !ok {
			continue
		}

		if err := applyMetadata(entry.path, entry.mode, entry.metadata); err != nil {
			return fmt.Errorf("failed to finalize metadata for planned directory `%s`: %w", entry.path, err)
		}
	}

	return nil
}

func (g *Generator) applyPlannedEntry(entry plannedEntry) (bool, error) {
	info, err := os.Lstat(entry.path)
	if err == nil {
		if g.runMode == RunModeStrict {
			if entry.typeID == plannedEntryTypeDir && entry.path == g.basePath && info.IsDir() {
				return false, nil
			}

			return false, fmt.Errorf("planned path already exists in strict mode: `%s`", entry.path)
		}

		switch entry.typeID {
		case plannedEntryTypeDir:
			if !info.IsDir() {
				return false, fmt.Errorf("planned directory path `%s` already exists as non-directory", entry.path)
			}
		case plannedEntryTypeFile:
			if !info.Mode().IsRegular() {
				return false, fmt.Errorf("planned file path `%s` already exists as non-regular file", entry.path)
			}
		case plannedEntryTypeSymlink:
			if info.Mode()&os.ModeSymlink == 0 {
				return false, fmt.Errorf("planned symlink path `%s` already exists as non-symlink", entry.path)
			}
		case plannedEntryTypeHardlink:
			if !info.Mode().IsRegular() {
				return false, fmt.Errorf("planned hardlink path `%s` already exists as non-regular file", entry.path)
			}
		case plannedEntryTypeSpecial:
			if !matchesSpecialFileType(info.Mode(), entry.specialFileType) {
				return false, fmt.Errorf(
					"planned special file path `%s` already exists as incompatible type (expected %s)",
					entry.path,
					entry.specialFileType,
				)
			}
		default:
			return false, fmt.Errorf("unknown planned entry type %d for path `%s`", entry.typeID, entry.path)
		}

		return false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("failed to inspect planned path `%s`: %w", entry.path, err)
	}

	switch entry.typeID {
	case plannedEntryTypeDir:
		if err := os.MkdirAll(entry.path, fs.FileMode(entry.mode&0o777)); err != nil {
			return false, fmt.Errorf("failed to create planned directory `%s`: %w", entry.path, err)
		}
	case plannedEntryTypeFile:
		if entry.contentPattern.pattern != 0 {
			if err := writePlannedFileContent(entry.path, entry.mode, entry.contentPattern); err != nil {
				return false, err
			}
		} else {
			if err := os.WriteFile(entry.path, entry.data, fs.FileMode(entry.mode&0o777)); err != nil {
				return false, fmt.Errorf("failed to create planned file `%s`: %w", entry.path, err)
			}
		}
		if err := applyMetadata(entry.path, entry.mode, entry.metadata); err != nil {
			return false, fmt.Errorf("failed to apply metadata for planned file `%s`: %w", entry.path, err)
		}
	case plannedEntryTypeSymlink:
		if err := os.Symlink(entry.linkTarget, entry.path); err != nil {
			return false, fmt.Errorf("failed to create planned symlink `%s` -> `%s`: %w", entry.path, entry.linkTarget, err)
		}
	case plannedEntryTypeHardlink:
		if err := os.Link(entry.linkTarget, entry.path); err != nil {
			return false, fmt.Errorf("failed to create planned hardlink `%s` -> `%s`: %w", entry.path, entry.linkTarget, err)
		}
	case plannedEntryTypeSpecial:
		if err := createPlannedSpecialFile(
			entry.path,
			entry.specialFileType,
			entry.mode,
			entry.specialDeviceMajor,
			entry.specialDeviceMinor,
		); err != nil {
			return false, err
		}
	default:
		return false, fmt.Errorf("unknown planned entry type %d for path `%s`", entry.typeID, entry.path)
	}

	return true, nil
}

func matchesSpecialFileType(mode fs.FileMode, fileType SpecialFileType) bool {
	switch fileType {
	case SpecialFileTypeFIFO:
		return mode&os.ModeNamedPipe != 0
	case SpecialFileTypeSocket:
		return mode&os.ModeSocket != 0
	case SpecialFileTypeCharDevice:
		return mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0
	case SpecialFileTypeBlockDevice:
		return mode&os.ModeDevice != 0 && mode&os.ModeCharDevice == 0
	default:
		return false
	}
}
