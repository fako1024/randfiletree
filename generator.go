package randfiletree

import (
	"fmt"
	"io/fs"
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
	lastPath string

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

	symlinkProbGen    BooleanGenerator
	symlinkRelProbGen BooleanGenerator

	rndSrc *rand.Rand
}

// New instantiates a new generator
func New(basePath string) *Generator {
	return &Generator{
		basePath: basePath,

		/* #nosec G404 */
		rndSrc: rand.New(rand.NewSource(defaultSeed)),
	}
}

// Run generates a new tree (or adds to an existing one) according to the defined rules
func (g *Generator) Run() error {
	if g == nil {
		return fmt.Errorf("nil generator")
	}

	if g.hasNoConfiguration() {
		return nil
	}

	if err := g.validateRunConfiguration(); err != nil {
		return err
	}

	return g.writeDir(g.basePath, 0)
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
		g.symlinkProbGen == nil &&
		g.symlinkRelProbGen == nil
}

func (g *Generator) validateRunConfiguration() error {
	missing := make([]string, 0, 13)

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
	if g.symlinkProbGen == nil {
		missing = append(missing, "symlink generator")
	}
	if g.symlinkRelProbGen == nil {
		missing = append(missing, "relative symlink generator")
	}

	if len(missing) > 0 {
		return fmt.Errorf("generator configuration incomplete, missing: %s", strings.Join(missing, ", "))
	}

	return nil
}

// RemoveAll removes (and recreates) the directory
func (g *Generator) RemoveAll() error {
	return os.RemoveAll(g.basePath)
}

// Walk performs a recursive walk through the provided directory (wrapping filepath.Walk())
func (g *Generator) Walk(fn filepath.WalkFunc) error {
	return filepath.Walk(g.basePath, fn)
}

func (g *Generator) writeDir(path string, depth int) error {

	// Check for depth abort criterion
	depth++
	if depth > g.pathDepthGen(g.rndSrc) {
		return nil
	}

	// Check if the directory already exists
	if _, err := os.Stat(path); depth > 1 && err == nil {
		return nil
	}

	// Create the directory
	if err := os.MkdirAll(path, fs.FileMode(g.dirModeGen(g.rndSrc))); err != nil {
		return err
	}

	// Create sub-directories, if any
	for i := 0; i < g.nDirsInDirGen(g.rndSrc); i++ {
		if err := g.writeDir(filepath.Join(path, g.dirNameGen(g.rndSrc, g.dirNameLenGen(g.rndSrc))), depth); err != nil {
			return err
		}
	}

	// Create files, if any
	for i := 0; i < g.nFilesInDirGen(g.rndSrc); i++ {
		if g.lastPath != "" && g.symlinkProbGen(g.rndSrc) {
			if err := g.writeSymlinkInDir(path, g.lastPath); err != nil {
				return err
			}
		} else {
			createdPath, created, err := g.writeFileInDir(path)
			if err != nil {
				return err
			}
			if created && g.symlinkRelProbGen(g.rndSrc) {
				if err := g.writeRelSymlink(path, createdPath); err != nil {
					return err
				}
			}
		}

	}

	return nil
}

func (g *Generator) writeFileInDir(dir string) (string, bool, error) {
	path := filepath.Join(dir, g.fileNameGen(g.rndSrc, g.fileNameLenGen(g.rndSrc)))

	// Check if the file already exists
	if _, err := os.Lstat(path); err == nil {
		return "", false, nil
	}

	mode := g.fileModeGen(g.rndSrc)
	data, err := g.dataGen(g.rndSrc)
	if err != nil {
		return "", false, err
	}

	if err := os.WriteFile(path, data, fs.FileMode(mode)); err != nil {
		return "", false, err
	}

	g.lastPath = path

	return path, true, nil
}

func (g *Generator) writeSymlinkInDir(dir, target string) error {
	path := filepath.Join(dir, g.fileNameGen(g.rndSrc, g.fileNameLenGen(g.rndSrc)))

	// Check if the link already exists
	if _, err := os.Lstat(path); err == nil {
		return nil
	}

	return os.Symlink(target, path)
}

func (g *Generator) writeRelSymlink(dir, target string) error {
	if target == "" {
		return fmt.Errorf("empty symlink target")
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
