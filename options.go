package randfiletree

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"
)

// Option denotes a configuration option for a Generator.
type Option func(*Generator) error

// NewWithOptions instantiates a new generator and applies the provided options.
func NewWithOptions(basePath string, opts ...Option) (*Generator, error) {
	g := New(basePath)
	if err := g.Configure(opts...); err != nil {
		return nil, err
	}

	return g, nil
}

// Configure applies the provided options atomically.
func (g *Generator) Configure(opts ...Option) error {
	if g == nil {
		return ErrNilGenerator
	}

	next := *g
	for i, opt := range opts {
		if opt == nil {
			return fmt.Errorf("option at index %d is nil", i)
		}
		if err := opt(&next); err != nil {
			return fmt.Errorf("failed to apply option at index %d: %w", i, err)
		}
	}

	*g = next

	return nil
}

// WithSeed sets a new seed (and a new random source, for that matter).
func WithSeed(seed int64) Option {
	return func(g *Generator) error {
		g.rndSrc = rand.New(rand.NewSource(seed)) // #nosec G404
		return nil
	}
}

// WithRunMode sets the run mode used when applying generated plans.
func WithRunMode(mode RunMode) Option {
	return func(g *Generator) error {
		if err := validateRunMode(mode); err != nil {
			return err
		}

		g.runMode = mode

		return nil
	}
}

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

// WithContentPatternGenerator sets the generator used for file content patterns.
func WithContentPatternGenerator(gen ContentPatternGenerator) Option {
	return func(g *Generator) error {
		if err := validateContentPatternGenerator("content pattern generator", gen); err != nil {
			return err
		}

		g.contentPatternGen = gen

		return nil
	}
}

// WithContentPattern sets a fixed content pattern.
func WithContentPattern(pattern ContentPattern) Option {
	return func(g *Generator) error {
		if err := validateContentPattern(pattern); err != nil {
			return err
		}

		g.contentPatternGen = func(r *rand.Rand) ContentPattern {
			return pattern
		}

		return nil
	}
}

// WithContentPatternProbabilities sets weighted content pattern probabilities.
func WithContentPatternProbabilities(probabilities map[ContentPattern]float64) Option {
	return func(g *Generator) error {
		if err := validateContentPatternProbabilities(probabilities); err != nil {
			return err
		}

		copyProbabilities := make(map[ContentPattern]float64, len(probabilities))
		for pattern, probability := range probabilities {
			copyProbabilities[pattern] = probability
		}

		g.contentPatternGen = ContentPatternGeneratorProbabilityWeighted(copyProbabilities)

		return nil
	}
}

// WithContentLogicalSizeGenerator sets the generator used for logical file sizes with content patterns.
func WithContentLogicalSizeGenerator(gen NumberGenerator) Option {
	return func(g *Generator) error {
		if err := validateNumberGenerator("content logical size generator", gen); err != nil {
			return err
		}

		g.contentLogicalSizeGen = gen

		return nil
	}
}

// WithContentLogicalSize sets a fixed logical file size with content patterns.
func WithContentLogicalSize(size int) Option {
	return func(g *Generator) error {
		if size < 0 {
			return fmt.Errorf("content logical size must be >= 0, got %d", size)
		}

		g.contentLogicalSizeGen = NumberGeneratorConstant(size)

		return nil
	}
}

// WithContentLogicalSizeRange sets a random flat logical file size generator in the range [min, max).
func WithContentLogicalSizeRange(min, max int) Option {
	return func(g *Generator) error {
		if err := validateContentLogicalSizeRange("content logical size range", min, max); err != nil {
			return err
		}

		g.contentLogicalSizeGen = NumberGeneratorRandomFlat(min, max)

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

// WithOwnershipGenerators sets explicit uid/gid generators used for created files and directories.
func WithOwnershipGenerators(uidGen, gidGen NumberGenerator) Option {
	return func(g *Generator) error {
		if err := validateNumberGenerator("ownership uid generator", uidGen); err != nil {
			return err
		}
		if err := validateNumberGenerator("ownership gid generator", gidGen); err != nil {
			return err
		}

		g.ownershipUIDGen = uidGen
		g.ownershipGIDGen = gidGen

		return nil
	}
}

// WithOwnership sets a fixed uid/gid used for created files and directories.
func WithOwnership(uid, gid int) Option {
	return func(g *Generator) error {
		if uid < 0 {
			return fmt.Errorf("ownership uid must be >= 0, got %d", uid)
		}
		if gid < 0 {
			return fmt.Errorf("ownership gid must be >= 0, got %d", gid)
		}

		g.ownershipUIDGen = NumberGeneratorConstant(uid)
		g.ownershipGIDGen = NumberGeneratorConstant(gid)

		return nil
	}
}

// WithTimestampGenerators sets explicit atime/mtime generators used with nanosecond precision on Linux.
func WithTimestampGenerators(atimeGen, mtimeGen TimestampGenerator) Option {
	return func(g *Generator) error {
		if err := validateTimestampGenerator("atime generator", atimeGen); err != nil {
			return err
		}
		if err := validateTimestampGenerator("mtime generator", mtimeGen); err != nil {
			return err
		}

		g.atimeGen = atimeGen
		g.mtimeGen = mtimeGen

		return nil
	}
}

// WithTimestamps sets fixed atime/mtime used with nanosecond precision on Linux.
func WithTimestamps(atime, mtime time.Time) Option {
	return func(g *Generator) error {
		g.atimeGen = TimestampGeneratorConstant(atime)
		g.mtimeGen = TimestampGeneratorConstant(mtime)

		return nil
	}
}

// WithXAttrValueGenerator adds or replaces a single xattr value generator.
func WithXAttrValueGenerator(name string, gen DataGenerator) Option {
	return func(g *Generator) error {
		if err := validateDataGenerator("xattr value generator", gen); err != nil {
			return err
		}

		normalizedName, err := validateXAttrName(name)
		if err != nil {
			return err
		}

		next := make([]xattrValueGeneratorConfig, 0, len(g.xattrValueGens)+1)
		replaced := false
		for _, cfg := range g.xattrValueGens {
			if cfg.name == normalizedName {
				next = append(next, xattrValueGeneratorConfig{name: normalizedName, valueGen: gen})
				replaced = true
				continue
			}

			next = append(next, cfg)
		}

		if !replaced {
			next = append(next, xattrValueGeneratorConfig{name: normalizedName, valueGen: gen})
		}

		sort.Slice(next, func(i, j int) bool {
			return next[i].name < next[j].name
		})

		g.xattrValueGens = next

		return nil
	}
}

// WithXAttr sets a fixed xattr value.
func WithXAttr(name string, value []byte) Option {
	return WithXAttrValueGenerator(name, DataGeneratorFixed(cloneBytes(value)))
}

// WithXAttrsFixed sets a complete fixed xattr map.
func WithXAttrsFixed(xattrs map[string][]byte) Option {
	return func(g *Generator) error {
		if len(xattrs) == 0 {
			g.xattrValueGens = nil
			return nil
		}

		names := make([]string, 0, len(xattrs))
		for name := range xattrs {
			normalizedName, err := validateXAttrName(name)
			if err != nil {
				return err
			}

			names = append(names, normalizedName)
		}

		sort.Strings(names)

		next := make([]xattrValueGeneratorConfig, 0, len(names))
		for _, name := range names {
			value := cloneBytes(xattrs[name])
			next = append(next, xattrValueGeneratorConfig{
				name:     name,
				valueGen: DataGeneratorFixed(value),
			})
		}

		g.xattrValueGens = next

		return nil
	}
}

// WithTrustedXAttrNamespace sets whether trusted.* xattrs are allowed.
func WithTrustedXAttrNamespace(enabled bool) Option {
	return func(g *Generator) error {
		g.xattrAllowTrustedNamespace = enabled
		return nil
	}
}

// WithSecurityXAttrNamespace sets whether security.* xattrs are allowed.
func WithSecurityXAttrNamespace(enabled bool) Option {
	return func(g *Generator) error {
		g.xattrAllowSecurityNamespace = enabled
		return nil
	}
}

// WithACLGenerator sets the ACL entry generator.
func WithACLGenerator(gen ACLGenerator) Option {
	return func(g *Generator) error {
		if gen == nil {
			return fmt.Errorf("ACL generator must not be nil")
		}

		g.aclEntriesGen = gen

		return nil
	}
}

// WithACL sets fixed ACL entries.
func WithACL(entries ...string) Option {
	fixed := append([]string(nil), entries...)
	return WithACLGenerator(ACLGeneratorFixed(fixed))
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

// WithHardlinkGenerator sets the generator used to decide whether to create hardlinks.
func WithHardlinkGenerator(gen BooleanGenerator) Option {
	return func(g *Generator) error {
		if err := validateBooleanGenerator("hardlink generator", gen); err != nil {
			return err
		}
		g.hardlinkProbGen = gen
		return nil
	}
}

// WithSpecialFileGenerator sets the generator used to decide whether to create special files.
func WithSpecialFileGenerator(gen BooleanGenerator) Option {
	return func(g *Generator) error {
		if err := validateBooleanGenerator("special file generator", gen); err != nil {
			return err
		}

		g.specialFileProbGen = gen

		return nil
	}
}

// WithSpecialFileTypeGenerator sets the generator used to choose special file types.
func WithSpecialFileTypeGenerator(gen SpecialFileTypeGenerator) Option {
	return func(g *Generator) error {
		if err := validateSpecialFileTypeGenerator("special file type generator", gen); err != nil {
			return err
		}

		g.specialFileTypeGen = gen

		return nil
	}
}

// WithSpecialFileTypeProbabilities sets weighted special file type probabilities.
func WithSpecialFileTypeProbabilities(probabilities map[SpecialFileType]float64) Option {
	return func(g *Generator) error {
		if err := validateSpecialFileTypeProbabilities(probabilities); err != nil {
			return err
		}

		copyProbabilities := make(map[SpecialFileType]float64, len(probabilities))
		for fileType, probability := range probabilities {
			copyProbabilities[fileType] = probability
		}

		g.specialFileTypeGen = SpecialFileTypeGeneratorProbabilityWeighted(copyProbabilities)

		return nil
	}
}

// WithSpecialDeviceNumberGenerators sets generators for special device major/minor numbers.
func WithSpecialDeviceNumberGenerators(majorGen, minorGen NumberGenerator) Option {
	return func(g *Generator) error {
		if err := validateNumberGenerator("special device major generator", majorGen); err != nil {
			return err
		}
		if err := validateNumberGenerator("special device minor generator", minorGen); err != nil {
			return err
		}

		g.specialDeviceMajorGen = majorGen
		g.specialDeviceMinorGen = minorGen

		return nil
	}
}

// WithSpecialDeviceNumbers sets fixed special device major/minor numbers.
func WithSpecialDeviceNumbers(major, minor int) Option {
	return func(g *Generator) error {
		if major < 0 {
			return fmt.Errorf("special device major must be >= 0, got %d", major)
		}
		if minor < 0 {
			return fmt.Errorf("special device minor must be >= 0, got %d", minor)
		}

		g.specialDeviceMajorGen = NumberGeneratorConstant(major)
		g.specialDeviceMinorGen = NumberGeneratorConstant(minor)

		return nil
	}
}

// WithSymlinkStrategyGenerator sets the generator used to choose symlink target strategies.
func WithSymlinkStrategyGenerator(gen SymlinkStrategyGenerator) Option {
	return func(g *Generator) error {
		if err := validateSymlinkStrategyGenerator("symlink strategy generator", gen); err != nil {
			return err
		}
		g.symlinkStrategyGen = gen
		return nil
	}
}

// WithSymlinkStrategyProbabilities sets weighted symlink strategy probabilities.
func WithSymlinkStrategyProbabilities(probabilities map[SymlinkStrategy]float64) Option {
	return func(g *Generator) error {
		if err := validateSymlinkStrategyProbabilities(probabilities); err != nil {
			return err
		}

		copyProbabilities := make(map[SymlinkStrategy]float64, len(probabilities))
		for strategy, probability := range probabilities {
			copyProbabilities[strategy] = probability
		}

		g.symlinkStrategyGen = SymlinkStrategyGeneratorProbabilityWeighted(copyProbabilities)

		return nil
	}
}

// WithByteFileNameGenerator sets the byte-level generator used for file names.
func WithByteFileNameGenerator(gen ByteNameGenerator) Option {
	return func(g *Generator) error {
		if err := validateByteNameGenerator("byte file name generator", gen); err != nil {
			return err
		}
		g.byteFileNameGen = gen
		return nil
	}
}

// WithByteDirNameGenerator sets the byte-level generator used for directory names.
func WithByteDirNameGenerator(gen ByteNameGenerator) Option {
	return func(g *Generator) error {
		if err := validateByteNameGenerator("byte directory name generator", gen); err != nil {
			return err
		}
		g.byteDirNameGen = gen
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

// WithHardlinkProbability sets a flat probability for hardlink creation in the range [0, 1].
func WithHardlinkProbability(probability float64) Option {
	return func(g *Generator) error {
		if err := validateProbability("hardlink probability", probability); err != nil {
			return err
		}

		g.hardlinkProbGen = BooleanGeneratorProbabilityFlat(probability)

		return nil
	}
}

// WithSpecialFileProbability sets a flat probability for special file creation in the range [0, 1].
func WithSpecialFileProbability(probability float64) Option {
	return func(g *Generator) error {
		if err := validateProbability("special file probability", probability); err != nil {
			return err
		}

		g.specialFileProbGen = BooleanGeneratorProbabilityFlat(probability)

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

/////////////////////////////

func validateProbability(name string, probability float64) error {
	if math.IsNaN(probability) {
		return fmt.Errorf("%s must not be NaN", name)
	}
	if math.IsInf(probability, 0) {
		return fmt.Errorf("%s must be finite", name)
	}
	if probability < 0 || probability > 1 {
		return fmt.Errorf("%s must be within [0, 1], got %v", name, probability)
	}

	return nil
}

func validateIntRange(name string, min, max int) error {
	if min < 0 {
		return fmt.Errorf("%s minimum must be >= 0, got %d", name, min)
	}
	if max <= min {
		return fmt.Errorf("%s maximum must be > minimum, got min=%d max=%d", name, min, max)
	}

	return nil
}

func validateFileNameGenerator(name string, gen FileNameGenerator) error {
	if gen == nil {
		return fmt.Errorf("%s must not be nil", name)
	}

	return nil
}

func validateByteNameGenerator(name string, gen ByteNameGenerator) error {
	if gen == nil {
		return fmt.Errorf("%s must not be nil", name)
	}

	return nil
}

func validateNumberGenerator(name string, gen NumberGenerator) error {
	if gen == nil {
		return fmt.Errorf("%s must not be nil", name)
	}

	return nil
}

func validateFileModeGenerator(name string, gen FileModeGenerator) error {
	if gen == nil {
		return fmt.Errorf("%s must not be nil", name)
	}

	return nil
}

func validateDataGenerator(name string, gen DataGenerator) error {
	if gen == nil {
		return fmt.Errorf("%s must not be nil", name)
	}

	return nil
}

func validateBooleanGenerator(name string, gen BooleanGenerator) error {
	if gen == nil {
		return fmt.Errorf("%s must not be nil", name)
	}

	return nil
}

func validateSymlinkStrategyGenerator(name string, gen SymlinkStrategyGenerator) error {
	if gen == nil {
		return fmt.Errorf("%s must not be nil", name)
	}

	return nil
}

func validateContentPatternGenerator(name string, gen ContentPatternGenerator) error {
	if gen == nil {
		return fmt.Errorf("%s must not be nil", name)
	}

	return nil
}

func validateTimestampGenerator(name string, gen TimestampGenerator) error {
	if gen == nil {
		return fmt.Errorf("%s must not be nil", name)
	}

	return nil
}

func validateSpecialFileTypeGenerator(name string, gen SpecialFileTypeGenerator) error {
	if gen == nil {
		return fmt.Errorf("%s must not be nil", name)
	}

	return nil
}

func validateSymlinkStrategyProbabilities(probabilities map[SymlinkStrategy]float64) error {
	if len(probabilities) == 0 {
		return ErrSymlinkStrategyProbabilitiesEmpty
	}

	total := 0.0
	for strategy, probability := range probabilities {
		if err := validateSymlinkStrategy(strategy); err != nil {
			return err
		}

		if math.IsNaN(probability) {
			return fmt.Errorf("symlink strategy probability for %s must not be NaN", strategy)
		}
		if math.IsInf(probability, 0) {
			return fmt.Errorf("symlink strategy probability for %s must be finite", strategy)
		}
		if probability < 0 {
			return fmt.Errorf("symlink strategy probability for %s must be >= 0, got %v", strategy, probability)
		}

		total += probability
	}

	if total <= 0 {
		return ErrSymlinkStrategyProbabilitiesNonPositive
	}

	return nil
}

func validateSpecialFileTypeProbabilities(probabilities map[SpecialFileType]float64) error {
	if len(probabilities) == 0 {
		return ErrSpecialFileTypeProbabilitiesEmpty
	}

	total := 0.0
	for fileType, probability := range probabilities {
		if err := validateSpecialFileType(fileType); err != nil {
			return err
		}

		if math.IsNaN(probability) {
			return fmt.Errorf("special file type probability for %s must not be NaN", fileType)
		}
		if math.IsInf(probability, 0) {
			return fmt.Errorf("special file type probability for %s must be finite", fileType)
		}
		if probability < 0 {
			return fmt.Errorf("special file type probability for %s must be >= 0, got %v", fileType, probability)
		}

		total += probability
	}

	if total <= 0 {
		return ErrSpecialFileTypeProbabilitiesNonPositive
	}

	return nil
}

func validateContentPatternProbabilities(probabilities map[ContentPattern]float64) error {
	if len(probabilities) == 0 {
		return ErrContentPatternProbabilitiesEmpty
	}

	total := 0.0
	for pattern, probability := range probabilities {
		if err := validateContentPattern(pattern); err != nil {
			return err
		}

		if math.IsNaN(probability) {
			return fmt.Errorf("content pattern probability for %s must not be NaN", pattern)
		}
		if math.IsInf(probability, 0) {
			return fmt.Errorf("content pattern probability for %s must be finite", pattern)
		}
		if probability < 0 {
			return fmt.Errorf("content pattern probability for %s must be >= 0, got %v", pattern, probability)
		}

		total += probability
	}

	if total <= 0 {
		return ErrContentPatternProbabilitiesNonPositive
	}

	return nil
}

func validateContentLogicalSizeRange(name string, min, max int) error {
	if min < 0 {
		return fmt.Errorf("%s minimum must be >= 0, got %d", name, min)
	}
	if max < 0 {
		return fmt.Errorf("%s maximum must be >= 0, got %d", name, max)
	}
	if max <= min {
		return fmt.Errorf("%s maximum must be > minimum, got min=%d max=%d", name, min, max)
	}

	return nil
}
