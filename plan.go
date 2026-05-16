package randfiletree

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
)

const maxPlanPathCollisionRetries = 128

var ErrPlanPathCollisionExhausted = errors.New("planned path collision retries exhausted")

type plannedEntryType uint8

const (
	plannedEntryTypeDir plannedEntryType = iota + 1
	plannedEntryTypeFile
	plannedEntryTypeSymlink
)

type plannedEntry struct {
	typeID plannedEntryType
	path   string

	mode fs.FileMode
	data []byte

	linkTarget string
}

type runPlan struct {
	entries []plannedEntry
}

type planState struct {
	rnd      *rand.Rand
	used     map[string]struct{}
	lastPath string
}

func (g *Generator) planRun() (runPlan, error) {
	state := planState{
		rnd: g.rndSrc,
		used: map[string]struct{}{
			g.basePath: {},
		},
	}

	plan := runPlan{entries: make([]plannedEntry, 0, 16)}
	if err := g.planDir(g.basePath, 0, &state, &plan); err != nil {
		return runPlan{}, err
	}

	return plan, nil
}

func (g *Generator) planDir(path string, depth int, state *planState, plan *runPlan) error {
	depth++
	if depth > g.pathDepthGen(state.rnd) {
		return nil
	}

	plan.entries = append(plan.entries, plannedEntry{
		typeID: plannedEntryTypeDir,
		path:   path,
		mode:   fs.FileMode(g.dirModeGen(state.rnd)),
	})

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
		if state.lastPath != "" && g.symlinkProbGen(state.rnd) {
			if err := g.planSymlink(path, state.lastPath, state, plan); err != nil {
				return err
			}

			continue
		}

		filePath, err := g.planUniquePath(path, state, g.fileNameGen, g.fileNameLenGen, "file")
		if err != nil {
			return err
		}

		data, err := g.dataGen(state.rnd)
		if err != nil {
			return fmt.Errorf("failed to generate file data for `%s`: %w", filePath, err)
		}

		plan.entries = append(plan.entries, plannedEntry{
			typeID: plannedEntryTypeFile,
			path:   filePath,
			mode:   fs.FileMode(g.fileModeGen(state.rnd)),
			data:   data,
		})
		state.lastPath = filePath

		if !g.symlinkRelProbGen(state.rnd) {
			continue
		}

		relTarget, err := filepath.Rel(path, filePath)
		if err != nil {
			return fmt.Errorf("failed to derive relative symlink target for `%s`: %w", filePath, err)
		}

		if err := g.planSymlink(path, relTarget, state, plan); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) planSymlink(dir, target string, state *planState, plan *runPlan) error {
	symlinkPath, err := g.planUniquePath(dir, state, g.fileNameGen, g.fileNameLenGen, "symlink")
	if err != nil {
		return err
	}

	plan.entries = append(plan.entries, plannedEntry{
		typeID:     plannedEntryTypeSymlink,
		path:       symlinkPath,
		linkTarget: target,
	})

	return nil
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

	for _, entry := range plan.entries {
		if err := g.applyPlannedEntry(entry); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) applyPlannedEntry(entry plannedEntry) error {
	info, err := os.Lstat(entry.path)
	if err == nil {
		if g.runMode == RunModeStrict {
			if entry.typeID == plannedEntryTypeDir && entry.path == g.basePath && info.IsDir() {
				return nil
			}

			return fmt.Errorf("planned path already exists in strict mode: `%s`", entry.path)
		}

		switch entry.typeID {
		case plannedEntryTypeDir:
			if !info.IsDir() {
				return fmt.Errorf("planned directory path `%s` already exists as non-directory", entry.path)
			}
		case plannedEntryTypeFile:
			if !info.Mode().IsRegular() {
				return fmt.Errorf("planned file path `%s` already exists as non-regular file", entry.path)
			}
		case plannedEntryTypeSymlink:
			if info.Mode()&os.ModeSymlink == 0 {
				return fmt.Errorf("planned symlink path `%s` already exists as non-symlink", entry.path)
			}
		default:
			return fmt.Errorf("unknown planned entry type %d for path `%s`", entry.typeID, entry.path)
		}

		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to inspect planned path `%s`: %w", entry.path, err)
	}

	switch entry.typeID {
	case plannedEntryTypeDir:
		if err := os.MkdirAll(entry.path, entry.mode); err != nil {
			return fmt.Errorf("failed to create planned directory `%s`: %w", entry.path, err)
		}
	case plannedEntryTypeFile:
		if err := os.WriteFile(entry.path, entry.data, entry.mode); err != nil {
			return fmt.Errorf("failed to create planned file `%s`: %w", entry.path, err)
		}
	case plannedEntryTypeSymlink:
		if err := os.Symlink(entry.linkTarget, entry.path); err != nil {
			return fmt.Errorf("failed to create planned symlink `%s` -> `%s`: %w", entry.path, entry.linkTarget, err)
		}
	default:
		return fmt.Errorf("unknown planned entry type %d for path `%s`", entry.typeID, entry.path)
	}

	return nil
}
