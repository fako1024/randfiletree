package randfiletree

import (
	"io/fs"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
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

	symlinkProbGen BooleanGenerator

	rndSrc *rand.Rand
}

// New instantiates a new generator
func New(basePath string) *Generator {
	return &Generator{
		basePath:       basePath,
		dirNameGen:     StringGeneratorAlphabet(FileNameAlphabetBasic),
		dirNameLenGen:  NumberGeneratorRandomFlat(1, 64),
		dirModeGen:     FileModeGeneratorConstant(0755),
		nFilesInDirGen: NumberGeneratorRandomFlat(1, 10),
		nDirsInDirGen:  NumberGeneratorRandomFlat(0, 10),
		fileNameGen:    StringGeneratorAlphabet(FileNameAlphabetBasic),
		fileNameLenGen: NumberGeneratorRandomFlat(1, 64),
		fileModeGen:    FileModeGeneratorConstant(0644),
		dataGen:        DataGeneratorRandom(NumberGeneratorRandomFlat(0, 1024)),
		pathDepthGen:   NumberGeneratorConstant(5),
		symlinkProbGen: BooleanGeneratorProbabilityFlat(0.1),

		/* #nosec G404 */
		rndSrc: rand.New(rand.NewSource(defaultSeed)),
	}
}

// Run generates a new tree (or adds to an existing one) according to the defined rules
func (g *Generator) Run() error {
	return g.writeDir(g.basePath, 0)
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
			if err := g.writeFileInDir(path); err != nil {
				return err
			}
		}

	}

	return nil
}

func (g *Generator) writeFileInDir(dir string) error {
	path := filepath.Join(dir, g.fileNameGen(g.rndSrc, g.fileNameLenGen(g.rndSrc)))
	mode := g.fileModeGen(g.rndSrc)
	data, err := g.dataGen(g.rndSrc)
	if err != nil {
		return err
	}

	defer func() {
		g.lastPath = path
	}()

	return ioutil.WriteFile(path, data, fs.FileMode(mode))
}

func (g *Generator) writeSymlinkInDir(dir, target string) error {
	path := filepath.Join(dir, g.fileNameGen(g.rndSrc, g.fileNameLenGen(g.rndSrc)))
	return os.Symlink(target, path)
}
