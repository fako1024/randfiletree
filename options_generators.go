package randfiletree

// WithDirNameGenerator sets the generator used for directory names.
func WithDirNameGenerator(gen FileNameGenerator) Option {
	return func(g *Generator) error {
		if err := validateFileNameGenerator("directory name generator", gen); err != nil {
			return err
		}
		g.dirNameGen = gen
		return nil
	}
}

// WithDirNameLengthGenerator sets the generator used for directory name lengths.
func WithDirNameLengthGenerator(gen FileNameLenGenerator) Option {
	return func(g *Generator) error {
		if err := validateNumberGenerator("directory name length generator", gen); err != nil {
			return err
		}
		g.dirNameLenGen = gen
		return nil
	}
}

// WithDirModeGenerator sets the generator used for directory modes.
func WithDirModeGenerator(gen FileModeGenerator) Option {
	return func(g *Generator) error {
		if err := validateFileModeGenerator("directory mode generator", gen); err != nil {
			return err
		}
		g.dirModeGen = gen
		return nil
	}
}

// WithFilesPerDirectoryGenerator sets the generator used for number of files in each directory.
func WithFilesPerDirectoryGenerator(gen NumberGenerator) Option {
	return func(g *Generator) error {
		if err := validateNumberGenerator("files-per-directory generator", gen); err != nil {
			return err
		}
		g.nFilesInDirGen = gen
		return nil
	}
}

// WithDirectoriesPerDirectoryGenerator sets the generator used for number of subdirectories in each directory.
func WithDirectoriesPerDirectoryGenerator(gen NumberGenerator) Option {
	return func(g *Generator) error {
		if err := validateNumberGenerator("directories-per-directory generator", gen); err != nil {
			return err
		}
		g.nDirsInDirGen = gen
		return nil
	}
}

// WithFileNameGenerator sets the generator used for file names.
func WithFileNameGenerator(gen FileNameGenerator) Option {
	return func(g *Generator) error {
		if err := validateFileNameGenerator("file name generator", gen); err != nil {
			return err
		}
		g.fileNameGen = gen
		return nil
	}
}

// WithFileNameLengthGenerator sets the generator used for file name lengths.
func WithFileNameLengthGenerator(gen FileNameLenGenerator) Option {
	return func(g *Generator) error {
		if err := validateNumberGenerator("file name length generator", gen); err != nil {
			return err
		}
		g.fileNameLenGen = gen
		return nil
	}
}

// WithFileModeGenerator sets the generator used for file modes.
func WithFileModeGenerator(gen FileModeGenerator) Option {
	return func(g *Generator) error {
		if err := validateFileModeGenerator("file mode generator", gen); err != nil {
			return err
		}
		g.fileModeGen = gen
		return nil
	}
}

// WithDataGenerator sets the generator used for file content.
func WithDataGenerator(gen DataGenerator) Option {
	return func(g *Generator) error {
		if err := validateDataGenerator("data generator", gen); err != nil {
			return err
		}
		g.dataGen = gen
		return nil
	}
}

// WithPathDepthGenerator sets the generator used for path depth.
func WithPathDepthGenerator(gen NumberGenerator) Option {
	return func(g *Generator) error {
		if err := validateNumberGenerator("path depth generator", gen); err != nil {
			return err
		}
		g.pathDepthGen = gen
		return nil
	}
}

// WithSymlinkGenerator sets the generator used to decide whether to create symlinks.
func WithSymlinkGenerator(gen BooleanGenerator) Option {
	return func(g *Generator) error {
		if err := validateBooleanGenerator("symlink generator", gen); err != nil {
			return err
		}
		g.symlinkProbGen = gen
		return nil
	}
}

// WithRelativeSymlinkGenerator sets the generator used to decide whether to create relative symlinks.
func WithRelativeSymlinkGenerator(gen BooleanGenerator) Option {
	return func(g *Generator) error {
		if err := validateBooleanGenerator("relative symlink generator", gen); err != nil {
			return err
		}
		g.symlinkRelProbGen = gen
		return nil
	}
}

// WithSymlinkProbability sets a flat probability for symlink creation in the range [0, 1].
func WithSymlinkProbability(probability float64) Option {
	return func(g *Generator) error {
		if err := validateProbability("symlink probability", probability); err != nil {
			return err
		}

		g.symlinkProbGen = BooleanGeneratorProbabilityFlat(probability)

		return nil
	}
}

// WithRelativeSymlinkProbability sets a flat probability for relative symlink creation in the range [0, 1].
func WithRelativeSymlinkProbability(probability float64) Option {
	return func(g *Generator) error {
		if err := validateProbability("relative symlink probability", probability); err != nil {
			return err
		}

		g.symlinkRelProbGen = BooleanGeneratorProbabilityFlat(probability)

		return nil
	}
}

// WithDirNameLengthRange sets a random flat directory name length generator in the range [min, max).
func WithDirNameLengthRange(min, max int) Option {
	return func(g *Generator) error {
		if err := validateIntRange("directory name length range", min, max); err != nil {
			return err
		}

		g.dirNameLenGen = NumberGeneratorRandomFlat(min, max)

		return nil
	}
}

// WithFileNameLengthRange sets a random flat file name length generator in the range [min, max).
func WithFileNameLengthRange(min, max int) Option {
	return func(g *Generator) error {
		if err := validateIntRange("file name length range", min, max); err != nil {
			return err
		}

		g.fileNameLenGen = NumberGeneratorRandomFlat(min, max)

		return nil
	}
}

// WithFilesPerDirectoryRange sets a random flat files-per-directory generator in the range [min, max).
func WithFilesPerDirectoryRange(min, max int) Option {
	return func(g *Generator) error {
		if err := validateIntRange("files-per-directory range", min, max); err != nil {
			return err
		}

		g.nFilesInDirGen = NumberGeneratorRandomFlat(min, max)

		return nil
	}
}

// WithDirectoriesPerDirectoryRange sets a random flat directories-per-directory generator in the range [min, max).
func WithDirectoriesPerDirectoryRange(min, max int) Option {
	return func(g *Generator) error {
		if err := validateIntRange("directories-per-directory range", min, max); err != nil {
			return err
		}

		g.nDirsInDirGen = NumberGeneratorRandomFlat(min, max)

		return nil
	}
}

// WithDataLengthRange sets a randomized data generator with lengths in the range [min, max).
func WithDataLengthRange(min, max int) Option {
	return func(g *Generator) error {
		if err := validateIntRange("data length range", min, max); err != nil {
			return err
		}

		g.dataGen = DataGeneratorRandom(NumberGeneratorRandomFlat(min, max))

		return nil
	}
}
