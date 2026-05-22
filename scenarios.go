package randfiletree

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// BuiltInScenarioCapability denotes one capability expected by a built-in scenario.
type BuiltInScenarioCapability string

const (
	BuiltInScenarioCapabilityHardlinkCreation       BuiltInScenarioCapability = "hardlink-creation"
	BuiltInScenarioCapabilitySymlinkCreation        BuiltInScenarioCapability = "symlink-creation"
	BuiltInScenarioCapabilityContentPatterns        BuiltInScenarioCapability = "content-patterns"
	BuiltInScenarioCapabilityLinuxTimestampMetadata BuiltInScenarioCapability = "linux-timestamp-metadata"
	BuiltInScenarioCapabilityLinuxXAttrMetadata     BuiltInScenarioCapability = "linux-xattr-metadata"
	BuiltInScenarioCapabilityLinuxACLMetadata       BuiltInScenarioCapability = "linux-acl-metadata"
)

const (
	ScenarioNameHardlinkHeavy = "hardlink-heavy"
	ScenarioNameSymlinkCycle  = "symlink-cycle"
	ScenarioNameMetadataHeavy = "metadata-heavy"
	ScenarioNameSparseLarge   = "sparse-large"
	ScenarioNameXAttrACLHeavy = "xattr-acl-heavy"
)

// BuiltInScenarioDescriptor describes one catalog scenario entry.
type BuiltInScenarioDescriptor struct {
	Name string

	Intent string

	RequiredCapabilities []BuiltInScenarioCapability
	Prerequisites        []string
	Pitfalls             []string
}

// BuiltInScenarioSpec contains the fully configured scenario options for one deterministic seed.
type BuiltInScenarioSpec struct {
	Descriptor BuiltInScenarioDescriptor
	Seed       int64
	Options    []Option
}

type builtInScenarioTemplate struct {
	descriptor BuiltInScenarioDescriptor
	build      func(seed int64) []Option
}

var builtInScenarioOrder = []string{
	ScenarioNameHardlinkHeavy,
	ScenarioNameSymlinkCycle,
	ScenarioNameMetadataHeavy,
	ScenarioNameSparseLarge,
	ScenarioNameXAttrACLHeavy,
}

var builtInScenarioAliases = map[string]string{
	ScenarioNameHardlinkHeavy:           ScenarioNameHardlinkHeavy,
	ScenarioNameHardlinkHeavy + "-tree": ScenarioNameHardlinkHeavy,

	ScenarioNameSymlinkCycle:           ScenarioNameSymlinkCycle,
	ScenarioNameSymlinkCycle + "-tree": ScenarioNameSymlinkCycle,

	ScenarioNameMetadataHeavy:           ScenarioNameMetadataHeavy,
	ScenarioNameMetadataHeavy + "-tree": ScenarioNameMetadataHeavy,

	ScenarioNameSparseLarge:           ScenarioNameSparseLarge,
	ScenarioNameSparseLarge + "-tree": ScenarioNameSparseLarge,

	ScenarioNameXAttrACLHeavy:           ScenarioNameXAttrACLHeavy,
	ScenarioNameXAttrACLHeavy + "-tree": ScenarioNameXAttrACLHeavy,
}

var builtInScenarioTemplates = map[string]builtInScenarioTemplate{
	ScenarioNameHardlinkHeavy: {
		descriptor: BuiltInScenarioDescriptor{
			Name:   ScenarioNameHardlinkHeavy,
			Intent: "Create inode-sharing pressure with dense hardlink groups.",
			RequiredCapabilities: []BuiltInScenarioCapability{
				BuiltInScenarioCapabilityHardlinkCreation,
			},
			Prerequisites: []string{
				"Filesystem and runtime must allow hardlink creation.",
			},
			Pitfalls: []string{
				"Mutating one linked path mutates all paths in the same inode group.",
				"Path-level dedup assumptions can hide data sharing bugs.",
			},
		},
		build: buildHardlinkHeavyScenarioOptions,
	},
	ScenarioNameSymlinkCycle: {
		descriptor: BuiltInScenarioDescriptor{
			Name:   ScenarioNameSymlinkCycle,
			Intent: "Exercise cycle-aware and dangling symlink traversal behavior.",
			RequiredCapabilities: []BuiltInScenarioCapability{
				BuiltInScenarioCapabilitySymlinkCreation,
			},
			Prerequisites: []string{
				"Filesystem and runtime must allow symlink creation.",
			},
			Pitfalls: []string{
				"Recursive scanners can loop forever without cycle detection.",
				"Absolute-vs-relative target rewriting can break restore parity.",
			},
		},
		build: buildSymlinkCycleScenarioOptions,
	},
	ScenarioNameMetadataHeavy: {
		descriptor: BuiltInScenarioDescriptor{
			Name:   ScenarioNameMetadataHeavy,
			Intent: "Stress mode-bit and nanosecond timestamp metadata handling.",
			RequiredCapabilities: []BuiltInScenarioCapability{
				BuiltInScenarioCapabilityLinuxTimestampMetadata,
			},
			Prerequisites: []string{
				"Linux is required for nanosecond timestamp metadata controls.",
			},
			Pitfalls: []string{
				"Non-Linux platforms fail with explicit metadata unsupported errors.",
				"Mode-bit parity checks should distinguish permission bits from content data.",
			},
		},
		build: buildMetadataHeavyScenarioOptions,
	},
	ScenarioNameSparseLarge: {
		descriptor: BuiltInScenarioDescriptor{
			Name:   ScenarioNameSparseLarge,
			Intent: "Stress sparse allocation and large logical file replay behavior.",
			RequiredCapabilities: []BuiltInScenarioCapability{
				BuiltInScenarioCapabilityContentPatterns,
			},
			Prerequisites: []string{
				"Environment should have enough free space for the configured logical sizes.",
			},
			Pitfalls: []string{
				"Logical size and allocated blocks are intentionally not equivalent.",
				"Block-allocation behavior differs across filesystems and mount settings.",
			},
		},
		build: buildSparseLargeScenarioOptions,
	},
	ScenarioNameXAttrACLHeavy: {
		descriptor: BuiltInScenarioDescriptor{
			Name:   ScenarioNameXAttrACLHeavy,
			Intent: "Stress xattr/ACL metadata parity for backup and restore validation.",
			RequiredCapabilities: []BuiltInScenarioCapability{
				BuiltInScenarioCapabilityLinuxXAttrMetadata,
				BuiltInScenarioCapabilityLinuxACLMetadata,
			},
			Prerequisites: []string{
				"Linux filesystem must support user.* xattrs and POSIX ACL xattrs.",
			},
			Pitfalls: []string{
				"Some filesystems disable ACLs or xattrs by default and return unsupported errors.",
				"ACL semantics can vary between local and network-backed filesystems.",
			},
		},
		build: buildXAttrACLHeavyScenarioOptions,
	},
}

// BuiltInScenarioCatalog returns all built-in scenarios in deterministic order.
func BuiltInScenarioCatalog() []BuiltInScenarioDescriptor {
	catalog := make([]BuiltInScenarioDescriptor, 0, len(builtInScenarioOrder))
	for _, name := range builtInScenarioOrder {
		template := builtInScenarioTemplates[name]
		catalog = append(catalog, cloneBuiltInScenarioDescriptor(template.descriptor))
	}

	return catalog
}

// BuildBuiltInScenario resolves a named built-in scenario for the provided deterministic seed.
func BuildBuiltInScenario(name string, seed int64) (BuiltInScenarioSpec, error) {
	normalizedName := normalizeBuiltInScenarioName(name)
	if normalizedName == "" {
		return BuiltInScenarioSpec{}, ErrScenarioNameEmpty
	}

	canonicalName, exists := builtInScenarioAliases[normalizedName]
	if !exists {
		return BuiltInScenarioSpec{}, fmt.Errorf("%w: %s", ErrScenarioUnknown, normalizedName)
	}

	template, exists := builtInScenarioTemplates[canonicalName]
	if !exists {
		return BuiltInScenarioSpec{}, fmt.Errorf("%w: %s", ErrScenarioUnknown, canonicalName)
	}

	options := template.build(seed)

	return BuiltInScenarioSpec{
		Descriptor: cloneBuiltInScenarioDescriptor(template.descriptor),
		Seed:       seed,
		Options:    append([]Option(nil), options...),
	}, nil
}

func normalizeBuiltInScenarioName(name string) string {
	normalized := strings.TrimSpace(strings.ToLower(name))
	normalized = strings.NewReplacer("_", "-", " ", "-", "/", "-").Replace(normalized)

	for strings.Contains(normalized, "--") {
		normalized = strings.ReplaceAll(normalized, "--", "-")
	}

	return strings.Trim(normalized, "-")
}

func cloneBuiltInScenarioDescriptor(descriptor BuiltInScenarioDescriptor) BuiltInScenarioDescriptor {
	clone := descriptor
	clone.RequiredCapabilities = append([]BuiltInScenarioCapability(nil), descriptor.RequiredCapabilities...)
	clone.Prerequisites = append([]string(nil), descriptor.Prerequisites...)
	clone.Pitfalls = append([]string(nil), descriptor.Pitfalls...)

	return clone
}

func builtInScenarioBaseOptions(seed int64) []Option {
	return []Option{
		WithSeed(seed),
		WithRunMode(RunModeAppend),
		WithPlanEntryLimit(defaultPlanEntryLimit),
		WithDirNameGenerator(StringGeneratorAlphabet(FileNameAlphabetBasic)),
		WithDirNameLengthRange(6, 12),
		WithDirModeGenerator(FileModeGeneratorConstant(0o750)),
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(4)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithFileNameGenerator(StringGeneratorAlphabet(FileNameAlphabetBasic)),
		WithFileNameLengthRange(8, 16),
		WithFileModeGenerator(FileModeGeneratorConstant(0o640)),
		WithDataLengthRange(128, 512),
		WithPathDepthGenerator(NumberGeneratorConstant(2)),
		WithSymlinkProbability(0),
		WithRelativeSymlinkProbability(0),
		WithHardlinkProbability(0),
	}
}

func buildHardlinkHeavyScenarioOptions(seed int64) []Option {
	options := builtInScenarioBaseOptions(seed)

	return append(options,
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(10)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithPathDepthGenerator(NumberGeneratorConstant(3)),
		WithHardlinkProbability(0.85),
	)
}

func buildSymlinkCycleScenarioOptions(seed int64) []Option {
	options := builtInScenarioBaseOptions(seed)

	return append(options,
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(6)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithPathDepthGenerator(NumberGeneratorConstant(3)),
		WithSymlinkProbability(1),
		WithHardlinkProbability(0),
		WithSymlinkStrategyProbabilities(map[SymlinkStrategy]float64{
			SymlinkStrategyCycle:    6,
			SymlinkStrategyChained:  2,
			SymlinkStrategyDangling: 1,
		}),
	)
}

func buildMetadataHeavyScenarioOptions(seed int64) []Option {
	options := builtInScenarioBaseOptions(seed)

	baseTime := time.Unix(1_700_000_000, 123_456_789)

	return append(options,
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(6)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(2)),
		WithPathDepthGenerator(NumberGeneratorConstant(2)),
		WithDirModeGenerator(FileModeGeneratorConstant(0o2750)),
		WithFileModeGenerator(FileModeGeneratorConstant(0o6750)),
		WithTimestampGenerators(
			func(r *rand.Rand) time.Time {
				return baseTime.Add(time.Duration(r.Intn(8)) * time.Second)
			},
			func(r *rand.Rand) time.Time {
				return baseTime.Add(30*time.Second + time.Duration(r.Intn(8))*time.Second)
			},
		),
	)
}

func buildSparseLargeScenarioOptions(seed int64) []Option {
	options := builtInScenarioBaseOptions(seed)

	return append(options,
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(3)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithPathDepthGenerator(NumberGeneratorConstant(2)),
		WithContentPatternProbabilities(map[ContentPattern]float64{
			ContentPatternSparseHoles:           7,
			ContentPatternPartialRangeOverwrite: 2,
			ContentPatternRepeatedBlocks:        1,
		}),
		WithContentLogicalSizeRange(1<<20, 2<<20),
		WithDataLengthRange(4<<10, 8<<10),
	)
}

func buildXAttrACLHeavyScenarioOptions(seed int64) []Option {
	options := builtInScenarioBaseOptions(seed)

	return append(options,
		WithFilesPerDirectoryGenerator(NumberGeneratorConstant(4)),
		WithDirectoriesPerDirectoryGenerator(NumberGeneratorConstant(1)),
		WithPathDepthGenerator(NumberGeneratorConstant(2)),
		WithXAttrsFixed(map[string][]byte{
			"user.backup.case":      []byte("edge"),
			"user.backup.retention": []byte("long"),
			"user.backup.source":    []byte("simulated"),
		}),
		WithACL(
			"u::rwx",
			"g::r-x",
			"o::---",
			"u:1000:r--",
			"g:1000:r--",
		),
	)
}
