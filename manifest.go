package randfiletree

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/rand"
	gopath "path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
)

const (
	scenarioManifestVersionMinSupported = 1
	scenarioManifestVersionMaxSupported = 1
	scenarioManifestVersion             = scenarioManifestVersionMaxSupported

	scenarioManifestChecksumAlgorithmSHA256 = "sha256"
)

// scenarioManifestCanonicalJSON is the encoder used for checksum computation.
// It is pinned to encoding/json compatibility mode so that future jsoniter
// upgrades cannot silently change the canonical byte sequence and invalidate
// checksums embedded in existing manifests on disk.
var scenarioManifestCanonicalJSON = jsoniter.ConfigCompatibleWithStandardLibrary

// ScenarioManifest denotes a portable, versioned replay specification.
type ScenarioManifest struct {
	Version int `json:"version" yaml:"version"`

	Generator ScenarioManifestGenerator `json:"generator" yaml:"generator"`

	Entries []ScenarioManifestEntry `json:"entries" yaml:"entries"`

	Operations []Operation `json:"operations,omitempty" yaml:"operations,omitempty"`

	RequiredCapabilities []ScenarioManifestCapability `json:"requiredCapabilities,omitempty" yaml:"requiredCapabilities,omitempty"`

	Integrity ScenarioManifestIntegrity `json:"integrity" yaml:"integrity"`
}

// ScenarioManifestGenerator captures deterministic generator metadata.
type ScenarioManifestGenerator struct {
	Seed           int64   `json:"seed" yaml:"seed"`
	RunMode        RunMode `json:"runMode" yaml:"runMode"`
	PlanEntryLimit int     `json:"planEntryLimit" yaml:"planEntryLimit"`

	DeterministicSettings []ScenarioManifestSetting `json:"deterministicSettings,omitempty" yaml:"deterministicSettings,omitempty"`
}

// ScenarioManifestSetting captures future deterministic, non-random controls.
type ScenarioManifestSetting struct {
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value" yaml:"value"`
}

// ScenarioManifestIntegrity denotes payload integrity metadata.
type ScenarioManifestIntegrity struct {
	Algorithm string `json:"algorithm" yaml:"algorithm"`
	Checksum  string `json:"checksum" yaml:"checksum"`
}

// ScenarioManifestCapability denotes an explicit replay capability requirement.
type ScenarioManifestCapability string

const (
	ScenarioManifestCapabilityLinuxSpecialFiles      ScenarioManifestCapability = "linux-special-files"
	ScenarioManifestCapabilityLinuxOwnershipMetadata ScenarioManifestCapability = "linux-ownership-metadata"
	ScenarioManifestCapabilityLinuxTimestampMetadata ScenarioManifestCapability = "linux-timestamp-metadata"
	ScenarioManifestCapabilityLinuxXAttrMetadata     ScenarioManifestCapability = "linux-xattr-metadata"
	ScenarioManifestCapabilityLinuxACLMetadata       ScenarioManifestCapability = "linux-acl-metadata"
	ScenarioManifestCapabilityOperationXAttr         ScenarioManifestCapability = "operation-xattr"
	ScenarioManifestCapabilityOperationChown         ScenarioManifestCapability = "operation-chown"
)

// ScenarioManifestEntryType denotes one planned entry category.
type ScenarioManifestEntryType string

const (
	ScenarioManifestEntryTypeDir      ScenarioManifestEntryType = "dir"
	ScenarioManifestEntryTypeFile     ScenarioManifestEntryType = "file"
	ScenarioManifestEntryTypeSymlink  ScenarioManifestEntryType = "symlink"
	ScenarioManifestEntryTypeHardlink ScenarioManifestEntryType = "hardlink"
	ScenarioManifestEntryTypeSpecial  ScenarioManifestEntryType = "special"
)

// ScenarioManifestLinkTargetType denotes how a symlink target should be interpreted.
type ScenarioManifestLinkTargetType string

const (
	ScenarioManifestLinkTargetTypeLiteral      ScenarioManifestLinkTargetType = "literal"
	ScenarioManifestLinkTargetTypeManifestPath ScenarioManifestLinkTargetType = "manifest-path"
)

// ScenarioManifestEntry denotes one planned replay node.
type ScenarioManifestEntry struct {
	Type ScenarioManifestEntryType `json:"type" yaml:"type"`
	Path string                    `json:"path" yaml:"path"`

	Mode uint32 `json:"mode,omitempty" yaml:"mode,omitempty"`

	Data []byte `json:"data,omitempty" yaml:"data,omitempty"`

	Content *ScenarioManifestFileContent `json:"content,omitempty" yaml:"content,omitempty"`

	LinkTarget     string                         `json:"linkTarget,omitempty" yaml:"linkTarget,omitempty"`
	LinkTargetType ScenarioManifestLinkTargetType `json:"linkTargetType,omitempty" yaml:"linkTargetType,omitempty"`

	SourcePath string `json:"sourcePath,omitempty" yaml:"sourcePath,omitempty"`

	Special *ScenarioManifestSpecialFile `json:"special,omitempty" yaml:"special,omitempty"`

	Metadata *ScenarioManifestMetadata `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ScenarioManifestSpecialFile denotes planned special-file metadata.
type ScenarioManifestSpecialFile struct {
	Type        SpecialFileType `json:"type" yaml:"type"`
	DeviceMajor int             `json:"deviceMajor,omitempty" yaml:"deviceMajor,omitempty"`
	DeviceMinor int             `json:"deviceMinor,omitempty" yaml:"deviceMinor,omitempty"`
}

// ScenarioManifestFileContent denotes deterministic content pattern payloads.
type ScenarioManifestFileContent struct {
	Pattern     ContentPattern `json:"pattern" yaml:"pattern"`
	LogicalSize int64          `json:"logicalSize" yaml:"logicalSize"`

	Seed int64 `json:"seed,omitempty" yaml:"seed,omitempty"`

	RepeatedBlock []byte `json:"repeatedBlock,omitempty" yaml:"repeatedBlock,omitempty"`

	SparseExtents    []ScenarioManifestContentRange `json:"sparseExtents,omitempty" yaml:"sparseExtents,omitempty"`
	OverwriteExtents []ScenarioManifestContentRange `json:"overwriteExtents,omitempty" yaml:"overwriteExtents,omitempty"`
}

// ScenarioManifestContentRange denotes one deterministic content range.
type ScenarioManifestContentRange struct {
	Offset int64 `json:"offset" yaml:"offset"`
	Length int64 `json:"length" yaml:"length"`
	Seed   int64 `json:"seed,omitempty" yaml:"seed,omitempty"`
}

// ScenarioManifestMetadata denotes deterministic metadata replay controls.
type ScenarioManifestMetadata struct {
	Ownership  *ScenarioManifestOwnership  `json:"ownership,omitempty" yaml:"ownership,omitempty"`
	Timestamps *ScenarioManifestTimestamps `json:"timestamps,omitempty" yaml:"timestamps,omitempty"`

	XAttrs []ScenarioManifestXAttr `json:"xattrs,omitempty" yaml:"xattrs,omitempty"`
	ACL    []string                `json:"acl,omitempty" yaml:"acl,omitempty"`
}

// ScenarioManifestOwnership denotes uid/gid metadata.
type ScenarioManifestOwnership struct {
	UID int `json:"uid" yaml:"uid"`
	GID int `json:"gid" yaml:"gid"`
}

// ScenarioManifestTimestamps denotes atime/mtime metadata.
type ScenarioManifestTimestamps struct {
	AtimeUnixNano int64 `json:"atimeUnixNano" yaml:"atimeUnixNano"`
	MtimeUnixNano int64 `json:"mtimeUnixNano" yaml:"mtimeUnixNano"`
}

// ScenarioManifestXAttr denotes one xattr tuple.
type ScenarioManifestXAttr struct {
	Name  string `json:"name" yaml:"name"`
	Value []byte `json:"value,omitempty" yaml:"value,omitempty"`
}

// BuildScenarioManifest builds a portable replay manifest from generator plan + operation stream.
func BuildScenarioManifest(g *Generator, ops []Operation) (ScenarioManifest, error) {
	if g == nil {
		return ScenarioManifest{}, ErrNilGenerator
	}

	if err := g.validateRunConfiguration(); err != nil {
		return ScenarioManifest{}, err
	}

	normalizedOps := make([]Operation, len(ops))
	for i, op := range ops {
		normalized, err := normalizeOperation(op)
		if err != nil {
			return ScenarioManifest{}, fmt.Errorf("invalid operation at index %d: %w", i, err)
		}

		normalizedOps[i] = normalized
	}

	planner := *g
	planner.rndSrc = rand.New(rand.NewSource(g.seed)) // #nosec G404

	plan, err := planner.planRun()
	if err != nil {
		return ScenarioManifest{}, err
	}

	entries := make([]ScenarioManifestEntry, len(plan.entries))
	for i, entry := range plan.entries {
		normalized, normalizeErr := scenarioManifestEntryFromPlannedEntry(g.basePath, entry)
		if normalizeErr != nil {
			return ScenarioManifest{}, fmt.Errorf("failed to serialize planned entry at index %d: %w", i, normalizeErr)
		}

		entries[i] = normalized
	}

	manifest := ScenarioManifest{
		Version: scenarioManifestVersion,
		Generator: ScenarioManifestGenerator{
			Seed:           g.seed,
			RunMode:        g.runMode,
			PlanEntryLimit: g.planEntryLimit,
		},
		Entries:    entries,
		Operations: normalizedOps,
		Integrity:  ScenarioManifestIntegrity{},
	}

	manifest.RequiredCapabilities = deriveScenarioManifestCapabilities(manifest.Entries, manifest.Operations)

	normalizedManifest, err := normalizeScenarioManifest(manifest)
	if err != nil {
		return ScenarioManifest{}, err
	}

	sealed, err := sealScenarioManifestIntegrity(normalizedManifest)
	if err != nil {
		return ScenarioManifest{}, err
	}

	return sealed, nil
}

// BuildScenarioManifest builds a portable replay manifest from generator plan + operation stream.
func (g *Generator) BuildScenarioManifest(ops []Operation) (ScenarioManifest, error) {
	return BuildScenarioManifest(g, ops)
}

// ApplyScenarioManifest validates and applies a replay manifest to basePath.
func ApplyScenarioManifest(basePath string, manifest ScenarioManifest) error {
	if strings.TrimSpace(basePath) == "" {
		return ErrBasePathEmpty
	}

	normalizedManifest, err := normalizeScenarioManifest(manifest)
	if err != nil {
		return err
	}

	if err := verifyScenarioManifestIntegrity(normalizedManifest); err != nil {
		return err
	}

	if err := ensureScenarioManifestCapabilities(normalizedManifest.RequiredCapabilities); err != nil {
		return err
	}

	g := New(basePath)
	if err := g.Configure(
		WithSeed(normalizedManifest.Generator.Seed),
		WithRunMode(normalizedManifest.Generator.RunMode),
		WithPlanEntryLimit(normalizedManifest.Generator.PlanEntryLimit),
	); err != nil {
		return fmt.Errorf("failed to configure generator for scenario manifest apply: %w", err)
	}

	plan, err := runPlanFromScenarioManifest(basePath, normalizedManifest.Entries)
	if err != nil {
		return err
	}

	execCtx, err := newExecutionContext(FaultProfile{})
	if err != nil {
		return err
	}

	if _, err := g.applyRunPlan(plan, execCtx); err != nil {
		return fmt.Errorf("failed to apply scenario manifest planned entries: %w", err)
	}

	if len(normalizedManifest.Operations) == 0 {
		return nil
	}

	if err := ApplyOperations(basePath, normalizedManifest.Operations); err != nil {
		return fmt.Errorf("failed to apply scenario manifest operation stream: %w", err)
	}

	return nil
}

// ApplyScenarioManifest validates and applies a replay manifest to the generator base path.
func (g *Generator) ApplyScenarioManifest(manifest ScenarioManifest) error {
	if g == nil {
		return ErrNilGenerator
	}

	return ApplyScenarioManifest(g.basePath, manifest)
}

func normalizeScenarioManifest(manifest ScenarioManifest) (ScenarioManifest, error) {
	if err := validateScenarioManifestVersion(manifest.Version); err != nil {
		return ScenarioManifest{}, err
	}

	if err := validateRunMode(manifest.Generator.RunMode); err != nil {
		return ScenarioManifest{}, fmt.Errorf("generator run mode: %w", err)
	}

	if manifest.Generator.PlanEntryLimit <= 0 {
		return ScenarioManifest{}, fmt.Errorf("generator plan entry limit: %w", ErrPlanEntryLimitInvalid)
	}

	normalizedSettings := make([]ScenarioManifestSetting, len(manifest.Generator.DeterministicSettings))
	seenSettings := make(map[string]struct{}, len(normalizedSettings))
	for i, setting := range manifest.Generator.DeterministicSettings {
		name := strings.TrimSpace(setting.Name)
		if name == "" {
			return ScenarioManifest{}, fmt.Errorf("generator deterministic setting at index %d has empty name", i)
		}

		if _, exists := seenSettings[name]; exists {
			return ScenarioManifest{}, fmt.Errorf("generator deterministic setting %q is duplicated", name)
		}
		seenSettings[name] = struct{}{}

		normalizedSettings[i] = ScenarioManifestSetting{Name: name, Value: setting.Value}
	}

	sort.Slice(normalizedSettings, func(i, j int) bool {
		return normalizedSettings[i].Name < normalizedSettings[j].Name
	})

	normalizedCapabilities, err := normalizeScenarioManifestCapabilities(manifest.RequiredCapabilities)
	if err != nil {
		return ScenarioManifest{}, err
	}

	if len(manifest.Entries) == 0 {
		return ScenarioManifest{}, fmt.Errorf("scenario manifest entries must not be empty")
	}

	normalizedEntries := make([]ScenarioManifestEntry, len(manifest.Entries))
	for i, entry := range manifest.Entries {
		normalizedEntry, normalizeErr := normalizeScenarioManifestEntry(i, entry)
		if normalizeErr != nil {
			return ScenarioManifest{}, normalizeErr
		}

		normalizedEntries[i] = normalizedEntry
	}

	if normalizedEntries[0].Type != ScenarioManifestEntryTypeDir || normalizedEntries[0].Path != "/" {
		return ScenarioManifest{}, fmt.Errorf("first scenario manifest entry must be root directory \"/\"")
	}

	pathSet := make(map[string]ScenarioManifestEntryType, len(normalizedEntries))
	for i, entry := range normalizedEntries {
		if existingType, exists := pathSet[entry.Path]; exists {
			return ScenarioManifest{}, fmt.Errorf(
				"manifest entry at index %d path %q duplicates existing %s entry",
				i,
				entry.Path,
				existingType,
			)
		}
		pathSet[entry.Path] = entry.Type

		if entry.Path == "/" {
			continue
		}

		parentPath := gopath.Clean(gopath.Dir(entry.Path))
		if parentPath == "." {
			parentPath = "/"
		}

		parentType, exists := pathSet[parentPath]
		if !exists {
			return ScenarioManifest{}, fmt.Errorf("manifest entry at index %d path %q has missing parent directory %q", i, entry.Path, parentPath)
		}
		if parentType != ScenarioManifestEntryTypeDir {
			return ScenarioManifest{}, fmt.Errorf("manifest entry at index %d path %q parent %q is not a directory", i, entry.Path, parentPath)
		}
	}

	for i, entry := range normalizedEntries {
		if entry.Type != ScenarioManifestEntryTypeHardlink {
			continue
		}

		sourceType, exists := pathSet[entry.SourcePath]
		if !exists {
			return ScenarioManifest{}, fmt.Errorf("manifest hardlink entry at index %d source path %q does not exist in manifest", i, entry.SourcePath)
		}
		if sourceType != ScenarioManifestEntryTypeFile && sourceType != ScenarioManifestEntryTypeHardlink {
			return ScenarioManifest{}, fmt.Errorf("manifest hardlink entry at index %d source path %q is not a regular file path", i, entry.SourcePath)
		}
	}

	for i, entry := range normalizedEntries {
		if entry.Type != ScenarioManifestEntryTypeSymlink {
			continue
		}

		if entry.LinkTargetType != ScenarioManifestLinkTargetTypeManifestPath {
			continue
		}

		if _, exists := pathSet[entry.LinkTarget]; !exists {
			return ScenarioManifest{}, fmt.Errorf("manifest symlink entry at index %d target path %q does not exist in manifest", i, entry.LinkTarget)
		}
	}

	normalizedOps := make([]Operation, len(manifest.Operations))
	for i, op := range manifest.Operations {
		normalizedOp, normalizeErr := normalizeOperation(op)
		if normalizeErr != nil {
			return ScenarioManifest{}, fmt.Errorf("invalid manifest operation at index %d: %w", i, normalizeErr)
		}

		normalizedOps[i] = normalizedOp
	}

	requiredCapabilitiesByPayload := deriveScenarioManifestCapabilities(normalizedEntries, normalizedOps)
	requiredCapabilitySet := make(map[ScenarioManifestCapability]struct{}, len(requiredCapabilitiesByPayload))
	for _, requiredCapability := range requiredCapabilitiesByPayload {
		requiredCapabilitySet[requiredCapability] = struct{}{}
	}

	for _, declaredCapability := range normalizedCapabilities {
		delete(requiredCapabilitySet, declaredCapability)
	}

	if len(requiredCapabilitySet) > 0 {
		missing := make([]string, 0, len(requiredCapabilitySet))
		for capability := range requiredCapabilitySet {
			missing = append(missing, string(capability))
		}
		sort.Strings(missing)

		return ScenarioManifest{}, fmt.Errorf("%w: missing %s", ErrScenarioManifestCapabilitiesIncomplete, strings.Join(missing, ", "))
	}

	normalizedIntegrity := ScenarioManifestIntegrity{
		Algorithm: strings.ToLower(strings.TrimSpace(manifest.Integrity.Algorithm)),
		Checksum:  strings.ToLower(strings.TrimSpace(manifest.Integrity.Checksum)),
	}

	normalizedManifest := ScenarioManifest{
		Version: manifest.Version,
		Generator: ScenarioManifestGenerator{
			Seed:                  manifest.Generator.Seed,
			RunMode:               manifest.Generator.RunMode,
			PlanEntryLimit:        manifest.Generator.PlanEntryLimit,
			DeterministicSettings: normalizedSettings,
		},
		Entries:              normalizedEntries,
		Operations:           normalizedOps,
		RequiredCapabilities: normalizedCapabilities,
		Integrity:            normalizedIntegrity,
	}

	return normalizedManifest, nil
}

func validateScenarioManifestVersion(version int) error {
	if version < scenarioManifestVersionMinSupported {
		return fmt.Errorf(
			"%w: got %d, minimum supported is %d",
			ErrScenarioManifestVersionTooOld,
			version,
			scenarioManifestVersionMinSupported,
		)
	}

	if version > scenarioManifestVersionMaxSupported {
		return fmt.Errorf(
			"%w: got %d, maximum supported is %d",
			ErrScenarioManifestVersionTooNew,
			version,
			scenarioManifestVersionMaxSupported,
		)
	}

	return nil
}

func normalizeScenarioManifestCapabilities(capabilities []ScenarioManifestCapability) ([]ScenarioManifestCapability, error) {
	if len(capabilities) == 0 {
		return nil, nil
	}

	normalized := make([]ScenarioManifestCapability, len(capabilities))
	seen := make(map[ScenarioManifestCapability]struct{}, len(capabilities))
	for i, capability := range capabilities {
		name := ScenarioManifestCapability(strings.TrimSpace(string(capability)))
		if name == "" {
			return nil, fmt.Errorf("required capability at index %d is empty", i)
		}

		if err := validateScenarioManifestCapability(name); err != nil {
			return nil, fmt.Errorf("required capability at index %d: %w", i, err)
		}

		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("required capability %q is duplicated", name)
		}
		seen[name] = struct{}{}

		normalized[i] = name
	}

	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i] < normalized[j]
	})

	return normalized, nil
}

func normalizeScenarioManifestEntry(index int, entry ScenarioManifestEntry) (ScenarioManifestEntry, error) {
	normalized := ScenarioManifestEntry{Type: entry.Type}

	if err := validateScenarioManifestEntryType(normalized.Type); err != nil {
		return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d: %w", index, err)
	}

	allowRoot := normalized.Type == ScenarioManifestEntryTypeDir
	normalizedPath, err := normalizeOperationPath(entry.Path, allowRoot)
	if err != nil {
		return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d path: %w", index, err)
	}
	normalized.Path = normalizedPath

	if err := validateMode(entry.Mode); err != nil {
		if entry.Mode != 0 || normalized.Type == ScenarioManifestEntryTypeDir || normalized.Type == ScenarioManifestEntryTypeFile || normalized.Type == ScenarioManifestEntryTypeSpecial {
			return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d mode: %w", index, err)
		}
	}
	normalized.Mode = entry.Mode

	switch normalized.Type {
	case ScenarioManifestEntryTypeDir:
		if normalized.Path != "/" && entry.Mode == 0 {
			return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d directory mode must be set", index)
		}

		normalizedMetadata, metadataErr := normalizeScenarioManifestMetadata(entry.Metadata)
		if metadataErr != nil {
			return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d metadata: %w", index, metadataErr)
		}
		normalized.Metadata = normalizedMetadata

	case ScenarioManifestEntryTypeFile:
		if entry.Mode == 0 {
			return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d file mode must be set", index)
		}

		normalized.Data = cloneBytes(entry.Data)

		normalizedContent, contentErr := normalizeScenarioManifestFileContent(entry.Content)
		if contentErr != nil {
			return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d content: %w", index, contentErr)
		}
		normalized.Content = normalizedContent

		if normalized.Content != nil && len(normalized.Data) > 0 {
			return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d file must not define both data and content pattern", index)
		}

		normalizedMetadata, metadataErr := normalizeScenarioManifestMetadata(entry.Metadata)
		if metadataErr != nil {
			return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d metadata: %w", index, metadataErr)
		}
		normalized.Metadata = normalizedMetadata

	case ScenarioManifestEntryTypeSymlink:
		if strings.TrimSpace(entry.LinkTarget) == "" {
			return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d symlink target must not be empty", index)
		}
		if strings.Contains(entry.LinkTarget, "\x00") {
			return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d symlink target must not contain NUL bytes", index)
		}

		normalized.LinkTarget = entry.LinkTarget
		normalizedTargetType, targetTypeErr := normalizeScenarioManifestLinkTargetType(entry.LinkTargetType)
		if targetTypeErr != nil {
			return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d symlink target type: %w", index, targetTypeErr)
		}
		normalized.LinkTargetType = normalizedTargetType

		if normalized.LinkTargetType == ScenarioManifestLinkTargetTypeManifestPath {
			normalizedTargetPath, targetPathErr := normalizeOperationPath(entry.LinkTarget, true)
			if targetPathErr != nil {
				return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d symlink target path: %w", index, targetPathErr)
			}

			normalized.LinkTarget = normalizedTargetPath
		}

	case ScenarioManifestEntryTypeHardlink:
		normalizedSourcePath, sourcePathErr := normalizeOperationPath(entry.SourcePath, false)
		if sourcePathErr != nil {
			return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d hardlink source path: %w", index, sourcePathErr)
		}
		if normalizedSourcePath == normalized.Path {
			return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d hardlink source and destination must differ", index)
		}
		normalized.SourcePath = normalizedSourcePath

	case ScenarioManifestEntryTypeSpecial:
		if entry.Mode == 0 {
			return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d special file mode must be set", index)
		}

		if entry.Special == nil {
			return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d special file metadata must be set", index)
		}

		normalizedSpecial, specialErr := normalizeScenarioManifestSpecialFile(*entry.Special)
		if specialErr != nil {
			return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d special file metadata: %w", index, specialErr)
		}
		normalized.Special = &normalizedSpecial

		normalizedMetadata, metadataErr := normalizeScenarioManifestMetadata(entry.Metadata)
		if metadataErr != nil {
			return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d metadata: %w", index, metadataErr)
		}
		if normalizedMetadata != nil {
			return ScenarioManifestEntry{}, fmt.Errorf("manifest entry at index %d special files do not support metadata in manifest version %d", index, scenarioManifestVersion)
		}
	}

	return normalized, nil
}

func validateScenarioManifestEntryType(typeID ScenarioManifestEntryType) error {
	switch typeID {
	case ScenarioManifestEntryTypeDir,
		ScenarioManifestEntryTypeFile,
		ScenarioManifestEntryTypeSymlink,
		ScenarioManifestEntryTypeHardlink,
		ScenarioManifestEntryTypeSpecial:
		return nil
	default:
		return fmt.Errorf("invalid manifest entry type %q", typeID)
	}
}

func normalizeScenarioManifestLinkTargetType(typeID ScenarioManifestLinkTargetType) (ScenarioManifestLinkTargetType, error) {
	if typeID == "" {
		return ScenarioManifestLinkTargetTypeLiteral, nil
	}

	switch typeID {
	case ScenarioManifestLinkTargetTypeLiteral,
		ScenarioManifestLinkTargetTypeManifestPath:
		return typeID, nil
	default:
		return "", fmt.Errorf("invalid symlink target type %q", typeID)
	}
}

func normalizeScenarioManifestSpecialFile(special ScenarioManifestSpecialFile) (ScenarioManifestSpecialFile, error) {
	if err := validateSpecialFileType(special.Type); err != nil {
		return ScenarioManifestSpecialFile{}, err
	}

	if special.DeviceMajor < 0 {
		return ScenarioManifestSpecialFile{}, fmt.Errorf("special device major must be >= 0, got %d", special.DeviceMajor)
	}
	if special.DeviceMinor < 0 {
		return ScenarioManifestSpecialFile{}, fmt.Errorf("special device minor must be >= 0, got %d", special.DeviceMinor)
	}

	if isSpecialDeviceType(special.Type) {
		return special, nil
	}

	special.DeviceMajor = 0
	special.DeviceMinor = 0

	return special, nil
}

func normalizeScenarioManifestFileContent(content *ScenarioManifestFileContent) (*ScenarioManifestFileContent, error) {
	if content == nil {
		return nil, nil
	}

	if err := validateContentPattern(content.Pattern); err != nil {
		return nil, err
	}

	if content.LogicalSize < 0 {
		return nil, fmt.Errorf("content logical size must be >= 0, got %d", content.LogicalSize)
	}

	normalized := &ScenarioManifestFileContent{
		Pattern:       content.Pattern,
		LogicalSize:   content.LogicalSize,
		Seed:          content.Seed,
		RepeatedBlock: cloneBytes(content.RepeatedBlock),
	}

	if len(content.SparseExtents) > 0 {
		normalized.SparseExtents = make([]ScenarioManifestContentRange, len(content.SparseExtents))
		for i, extent := range content.SparseExtents {
			if extent.Offset < 0 {
				return nil, fmt.Errorf("sparse extent at index %d offset must be >= 0, got %d", i, extent.Offset)
			}
			if extent.Length <= 0 {
				return nil, fmt.Errorf("sparse extent at index %d length must be > 0, got %d", i, extent.Length)
			}
			if extent.Offset+extent.Length > content.LogicalSize {
				return nil, fmt.Errorf(
					"sparse extent at index %d exceeds logical size: offset=%d length=%d logicalSize=%d",
					i,
					extent.Offset,
					extent.Length,
					content.LogicalSize,
				)
			}

			normalized.SparseExtents[i] = extent
		}
	}

	if len(content.OverwriteExtents) > 0 {
		normalized.OverwriteExtents = make([]ScenarioManifestContentRange, len(content.OverwriteExtents))
		for i, extent := range content.OverwriteExtents {
			if extent.Offset < 0 {
				return nil, fmt.Errorf("overwrite extent at index %d offset must be >= 0, got %d", i, extent.Offset)
			}
			if extent.Length <= 0 {
				return nil, fmt.Errorf("overwrite extent at index %d length must be > 0, got %d", i, extent.Length)
			}
			if extent.Offset+extent.Length > content.LogicalSize {
				return nil, fmt.Errorf(
					"overwrite extent at index %d exceeds logical size: offset=%d length=%d logicalSize=%d",
					i,
					extent.Offset,
					extent.Length,
					content.LogicalSize,
				)
			}

			normalized.OverwriteExtents[i] = extent
		}
	}

	if normalized.Pattern == ContentPatternRepeatedBlocks && normalized.LogicalSize > 0 && len(normalized.RepeatedBlock) == 0 {
		return nil, ErrContentPatternRepeatedBlockEmpty
	}

	return normalized, nil
}

func normalizeScenarioManifestMetadata(metadata *ScenarioManifestMetadata) (*ScenarioManifestMetadata, error) {
	if metadata == nil {
		return nil, nil
	}

	normalized := &ScenarioManifestMetadata{}

	if metadata.Ownership != nil {
		if metadata.Ownership.UID < 0 {
			return nil, fmt.Errorf("metadata ownership uid must be >= 0, got %d", metadata.Ownership.UID)
		}
		if metadata.Ownership.GID < 0 {
			return nil, fmt.Errorf("metadata ownership gid must be >= 0, got %d", metadata.Ownership.GID)
		}

		normalized.Ownership = &ScenarioManifestOwnership{
			UID: metadata.Ownership.UID,
			GID: metadata.Ownership.GID,
		}
	}

	if metadata.Timestamps != nil {
		normalized.Timestamps = &ScenarioManifestTimestamps{
			AtimeUnixNano: metadata.Timestamps.AtimeUnixNano,
			MtimeUnixNano: metadata.Timestamps.MtimeUnixNano,
		}
	}

	if len(metadata.XAttrs) > 0 {
		normalized.XAttrs = make([]ScenarioManifestXAttr, len(metadata.XAttrs))
		seen := make(map[string]struct{}, len(metadata.XAttrs))
		for i, xattr := range metadata.XAttrs {
			name, err := validateXAttrName(xattr.Name)
			if err != nil {
				return nil, fmt.Errorf("xattr at index %d: %w", i, err)
			}

			if _, exists := seen[name]; exists {
				return nil, fmt.Errorf("xattr %q is duplicated", name)
			}
			seen[name] = struct{}{}

			normalized.XAttrs[i] = ScenarioManifestXAttr{
				Name:  name,
				Value: cloneBytes(xattr.Value),
			}
		}

		sort.Slice(normalized.XAttrs, func(i, j int) bool {
			return normalized.XAttrs[i].Name < normalized.XAttrs[j].Name
		})
	}

	if len(metadata.ACL) > 0 {
		normalizedACL, err := normalizeACLEntries(metadata.ACL)
		if err != nil {
			return nil, err
		}

		normalized.ACL = normalizedACL
	}

	if normalized.Ownership == nil &&
		normalized.Timestamps == nil &&
		len(normalized.XAttrs) == 0 &&
		len(normalized.ACL) == 0 {
		return nil, nil
	}

	return normalized, nil
}

func sealScenarioManifestIntegrity(manifest ScenarioManifest) (ScenarioManifest, error) {
	digest, err := scenarioManifestChecksum(manifest)
	if err != nil {
		return ScenarioManifest{}, err
	}

	manifest.Integrity = ScenarioManifestIntegrity{
		Algorithm: scenarioManifestChecksumAlgorithmSHA256,
		Checksum:  digest,
	}

	return manifest, nil
}

func verifyScenarioManifestIntegrity(manifest ScenarioManifest) error {
	if strings.TrimSpace(manifest.Integrity.Algorithm) == "" {
		return fmt.Errorf("%w: integrity algorithm is empty", ErrScenarioManifestChecksumAlgorithmUnsupported)
	}

	algorithm := strings.ToLower(manifest.Integrity.Algorithm)
	if algorithm != scenarioManifestChecksumAlgorithmSHA256 {
		return fmt.Errorf("%w: %q", ErrScenarioManifestChecksumAlgorithmUnsupported, manifest.Integrity.Algorithm)
	}

	expectedChecksum := strings.ToLower(strings.TrimSpace(manifest.Integrity.Checksum))
	if expectedChecksum == "" {
		return fmt.Errorf("%w: checksum is empty", ErrScenarioManifestChecksumMismatch)
	}

	if _, err := hex.DecodeString(expectedChecksum); err != nil {
		return fmt.Errorf("%w: checksum is not valid hex: %v", ErrScenarioManifestChecksumMismatch, err)
	}

	actualChecksum, err := scenarioManifestChecksum(manifest)
	if err != nil {
		return err
	}

	if subtle.ConstantTimeCompare([]byte(expectedChecksum), []byte(actualChecksum)) == 1 {
		return nil
	}

	return fmt.Errorf(
		"%w: expected %s, got %s",
		ErrScenarioManifestChecksumMismatch,
		expectedChecksum,
		actualChecksum,
	)
}

func scenarioManifestChecksum(manifest ScenarioManifest) (string, error) {
	manifest.Integrity = ScenarioManifestIntegrity{}

	payload, err := scenarioManifestCanonicalJSON.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("failed to serialize scenario manifest checksum payload: %w", err)
	}

	sum := sha256.Sum256(payload)

	return hex.EncodeToString(sum[:]), nil
}

func ensureScenarioManifestCapabilities(capabilities []ScenarioManifestCapability) error {
	for _, capability := range capabilities {
		if err := validateScenarioManifestCapability(capability); err != nil {
			return err
		}

		switch capability {
		case ScenarioManifestCapabilityLinuxSpecialFiles,
			ScenarioManifestCapabilityLinuxOwnershipMetadata,
			ScenarioManifestCapabilityLinuxTimestampMetadata,
			ScenarioManifestCapabilityLinuxXAttrMetadata,
			ScenarioManifestCapabilityLinuxACLMetadata,
			ScenarioManifestCapabilityOperationXAttr:
			if runtime.GOOS != "linux" {
				return fmt.Errorf("%w on %s: %s", ErrScenarioManifestCapabilityUnsupported, runtime.GOOS, capability)
			}

		case ScenarioManifestCapabilityOperationChown:
			if runtime.GOOS == "windows" {
				return fmt.Errorf("%w on %s: %s", ErrScenarioManifestCapabilityUnsupported, runtime.GOOS, capability)
			}
		}
	}

	return nil
}

func validateScenarioManifestCapability(capability ScenarioManifestCapability) error {
	switch capability {
	case ScenarioManifestCapabilityLinuxSpecialFiles,
		ScenarioManifestCapabilityLinuxOwnershipMetadata,
		ScenarioManifestCapabilityLinuxTimestampMetadata,
		ScenarioManifestCapabilityLinuxXAttrMetadata,
		ScenarioManifestCapabilityLinuxACLMetadata,
		ScenarioManifestCapabilityOperationXAttr,
		ScenarioManifestCapabilityOperationChown:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrScenarioManifestCapabilityUnsupported, capability)
	}
}

func deriveScenarioManifestCapabilities(entries []ScenarioManifestEntry, ops []Operation) []ScenarioManifestCapability {
	capabilitySet := make(map[ScenarioManifestCapability]struct{})

	for _, entry := range entries {
		switch entry.Type {
		case ScenarioManifestEntryTypeSpecial:
			capabilitySet[ScenarioManifestCapabilityLinuxSpecialFiles] = struct{}{}
		}

		if entry.Metadata == nil {
			continue
		}

		if entry.Metadata.Ownership != nil {
			capabilitySet[ScenarioManifestCapabilityLinuxOwnershipMetadata] = struct{}{}
		}
		if entry.Metadata.Timestamps != nil {
			capabilitySet[ScenarioManifestCapabilityLinuxTimestampMetadata] = struct{}{}
		}
		if len(entry.Metadata.XAttrs) > 0 {
			capabilitySet[ScenarioManifestCapabilityLinuxXAttrMetadata] = struct{}{}
		}
		if len(entry.Metadata.ACL) > 0 {
			capabilitySet[ScenarioManifestCapabilityLinuxACLMetadata] = struct{}{}
		}
	}

	for _, op := range ops {
		switch op.Kind {
		case OperationKindSetXAttr, OperationKindRemoveXAttr:
			capabilitySet[ScenarioManifestCapabilityOperationXAttr] = struct{}{}
		case OperationKindChown:
			capabilitySet[ScenarioManifestCapabilityOperationChown] = struct{}{}
		}
	}

	if len(capabilitySet) == 0 {
		return nil
	}

	capabilities := make([]ScenarioManifestCapability, 0, len(capabilitySet))
	for capability := range capabilitySet {
		capabilities = append(capabilities, capability)
	}

	sort.Slice(capabilities, func(i, j int) bool {
		return capabilities[i] < capabilities[j]
	})

	return capabilities
}

func scenarioManifestEntryFromPlannedEntry(basePath string, entry plannedEntry) (ScenarioManifestEntry, error) {
	manifestPath, err := planPathToManifestPath(basePath, entry.path)
	if err != nil {
		return ScenarioManifestEntry{}, err
	}

	typeID, err := plannedEntryTypeToScenarioManifestEntryType(entry.typeID)
	if err != nil {
		return ScenarioManifestEntry{}, err
	}

	manifestEntry := ScenarioManifestEntry{
		Type: typeID,
		Path: manifestPath,
		Mode: entry.mode,
	}

	metadata, err := scenarioManifestMetadataFromConfig(entry.metadata)
	if err != nil {
		return ScenarioManifestEntry{}, err
	}
	manifestEntry.Metadata = metadata

	switch entry.typeID {
	case plannedEntryTypeFile:
		manifestEntry.Data = cloneBytes(entry.data)

		content, contentErr := scenarioManifestFileContentFromPlanned(entry.contentPattern)
		if contentErr != nil {
			return ScenarioManifestEntry{}, contentErr
		}
		manifestEntry.Content = content

	case plannedEntryTypeSymlink:
		target, asManifestPath, targetErr := tryTranslateAbsolutePathToManifestPath(basePath, entry.linkTarget)
		if targetErr != nil {
			return ScenarioManifestEntry{}, targetErr
		}

		if asManifestPath {
			manifestEntry.LinkTarget = target
			manifestEntry.LinkTargetType = ScenarioManifestLinkTargetTypeManifestPath
		} else {
			manifestEntry.LinkTarget = entry.linkTarget
			manifestEntry.LinkTargetType = ScenarioManifestLinkTargetTypeLiteral
		}

	case plannedEntryTypeHardlink:
		sourcePath, sourceErr := planPathToManifestPath(basePath, entry.linkTarget)
		if sourceErr != nil {
			return ScenarioManifestEntry{}, sourceErr
		}

		manifestEntry.SourcePath = sourcePath

	case plannedEntryTypeSpecial:
		manifestEntry.Special = &ScenarioManifestSpecialFile{
			Type:        entry.specialFileType,
			DeviceMajor: entry.specialDeviceMajor,
			DeviceMinor: entry.specialDeviceMinor,
		}
	}

	return manifestEntry, nil
}

func scenarioManifestMetadataFromConfig(metadata metadataConfig) (*ScenarioManifestMetadata, error) {
	if !metadata.hasOwnership && !metadata.hasTimestamps && !metadata.hasXAttrs && !metadata.hasACL {
		return nil, nil
	}

	manifestMetadata := &ScenarioManifestMetadata{}

	if metadata.hasOwnership {
		manifestMetadata.Ownership = &ScenarioManifestOwnership{
			UID: metadata.uid,
			GID: metadata.gid,
		}
	}

	if metadata.hasTimestamps {
		manifestMetadata.Timestamps = &ScenarioManifestTimestamps{
			AtimeUnixNano: metadata.atime.UTC().UnixNano(),
			MtimeUnixNano: metadata.mtime.UTC().UnixNano(),
		}
	}

	if metadata.hasXAttrs {
		names := make([]string, 0, len(metadata.xattrs))
		for name := range metadata.xattrs {
			names = append(names, name)
		}
		sort.Strings(names)

		manifestMetadata.XAttrs = make([]ScenarioManifestXAttr, 0, len(names))
		for _, name := range names {
			manifestMetadata.XAttrs = append(manifestMetadata.XAttrs, ScenarioManifestXAttr{
				Name:  name,
				Value: cloneBytes(metadata.xattrs[name]),
			})
		}
	}

	if metadata.hasACL {
		manifestMetadata.ACL = append([]string(nil), metadata.aclEntries...)
	}

	normalized, err := normalizeScenarioManifestMetadata(manifestMetadata)
	if err != nil {
		return nil, err
	}

	return normalized, nil
}

func scenarioManifestFileContentFromPlanned(content plannedFileContent) (*ScenarioManifestFileContent, error) {
	if content.pattern == 0 {
		return nil, nil
	}

	rangesFromPlanned := func(input []plannedContentRange) []ScenarioManifestContentRange {
		if len(input) == 0 {
			return nil
		}

		normalized := make([]ScenarioManifestContentRange, len(input))
		for i := range input {
			normalized[i] = ScenarioManifestContentRange{
				Offset: input[i].offset,
				Length: input[i].length,
				Seed:   input[i].seed,
			}
		}

		return normalized
	}

	normalized := &ScenarioManifestFileContent{
		Pattern:          content.pattern,
		LogicalSize:      content.logicalSize,
		Seed:             content.seed,
		RepeatedBlock:    cloneBytes(content.repeatedBlock),
		SparseExtents:    rangesFromPlanned(content.sparseExtents),
		OverwriteExtents: rangesFromPlanned(content.overwriteExtents),
	}

	return normalizeScenarioManifestFileContent(normalized)
}

func runPlanFromScenarioManifest(basePath string, entries []ScenarioManifestEntry) (runPlan, error) {
	plan := runPlan{entries: make([]plannedEntry, len(entries))}

	for i, entry := range entries {
		planned, err := plannedEntryFromScenarioManifestEntry(basePath, entry)
		if err != nil {
			return runPlan{}, fmt.Errorf("failed to deserialize manifest entry at index %d: %w", i, err)
		}

		plan.entries[i] = planned
	}

	return plan, nil
}

func plannedEntryFromScenarioManifestEntry(basePath string, entry ScenarioManifestEntry) (plannedEntry, error) {
	path, err := operationPathToFS(basePath, entry.Path)
	if err != nil {
		return plannedEntry{}, fmt.Errorf("manifest entry path %q: %w", entry.Path, err)
	}
	if err := ensureNoSymlinkParents(basePath, path); err != nil {
		return plannedEntry{}, fmt.Errorf("manifest entry path %q: %w", entry.Path, err)
	}

	typeID, err := scenarioManifestEntryTypeToPlannedEntryType(entry.Type)
	if err != nil {
		return plannedEntry{}, err
	}

	metadata, err := metadataConfigFromScenarioManifestMetadata(entry.Metadata)
	if err != nil {
		return plannedEntry{}, err
	}

	planned := plannedEntry{
		typeID:   typeID,
		path:     path,
		mode:     entry.Mode,
		metadata: metadata,
	}

	switch entry.Type {
	case ScenarioManifestEntryTypeFile:
		planned.data = cloneBytes(entry.Data)

		contentPattern, contentErr := plannedFileContentFromScenarioManifestFileContent(entry.Content)
		if contentErr != nil {
			return plannedEntry{}, contentErr
		}
		planned.contentPattern = contentPattern

	case ScenarioManifestEntryTypeSymlink:
		targetType := entry.LinkTargetType
		if targetType == "" {
			targetType = ScenarioManifestLinkTargetTypeLiteral
		}

		target := entry.LinkTarget
		if targetType == ScenarioManifestLinkTargetTypeManifestPath {
			translatedTarget, targetErr := operationPathToFS(basePath, entry.LinkTarget)
			if targetErr != nil {
				return plannedEntry{}, fmt.Errorf("manifest symlink target path %q: %w", entry.LinkTarget, targetErr)
			}

			target = translatedTarget
		}

		planned.linkTarget = target

	case ScenarioManifestEntryTypeHardlink:
		sourcePath, sourceErr := operationPathToFS(basePath, entry.SourcePath)
		if sourceErr != nil {
			return plannedEntry{}, fmt.Errorf("manifest hardlink source path %q: %w", entry.SourcePath, sourceErr)
		}
		if err := ensureNoSymlinkParents(basePath, sourcePath); err != nil {
			return plannedEntry{}, fmt.Errorf("manifest hardlink source path %q: %w", entry.SourcePath, err)
		}

		planned.linkTarget = sourcePath

	case ScenarioManifestEntryTypeSpecial:
		planned.specialFileType = entry.Special.Type
		planned.specialDeviceMajor = entry.Special.DeviceMajor
		planned.specialDeviceMinor = entry.Special.DeviceMinor
	}

	return planned, nil
}

func metadataConfigFromScenarioManifestMetadata(metadata *ScenarioManifestMetadata) (metadataConfig, error) {
	if metadata == nil {
		return metadataConfig{}, nil
	}

	normalizedMetadata, err := normalizeScenarioManifestMetadata(metadata)
	if err != nil {
		return metadataConfig{}, err
	}

	if normalizedMetadata == nil {
		return metadataConfig{}, nil
	}

	config := metadataConfig{}

	if normalizedMetadata.Ownership != nil {
		config.hasOwnership = true
		config.uid = normalizedMetadata.Ownership.UID
		config.gid = normalizedMetadata.Ownership.GID
	}

	if normalizedMetadata.Timestamps != nil {
		config.hasTimestamps = true
		config.atime = time.Unix(0, normalizedMetadata.Timestamps.AtimeUnixNano).UTC()
		config.mtime = time.Unix(0, normalizedMetadata.Timestamps.MtimeUnixNano).UTC()
	}

	if len(normalizedMetadata.XAttrs) > 0 {
		config.hasXAttrs = true
		config.xattrs = make(map[string][]byte, len(normalizedMetadata.XAttrs))
		for _, xattr := range normalizedMetadata.XAttrs {
			config.xattrs[xattr.Name] = cloneBytes(xattr.Value)
		}
	}

	if len(normalizedMetadata.ACL) > 0 {
		config.hasACL = true
		config.aclEntries = append([]string(nil), normalizedMetadata.ACL...)
	}

	return config, nil
}

func plannedFileContentFromScenarioManifestFileContent(content *ScenarioManifestFileContent) (plannedFileContent, error) {
	if content == nil {
		return plannedFileContent{}, nil
	}

	normalizedContent, err := normalizeScenarioManifestFileContent(content)
	if err != nil {
		return plannedFileContent{}, err
	}

	planned := plannedFileContent{
		pattern:       normalizedContent.Pattern,
		logicalSize:   normalizedContent.LogicalSize,
		seed:          normalizedContent.Seed,
		repeatedBlock: cloneBytes(normalizedContent.RepeatedBlock),
	}

	if len(normalizedContent.SparseExtents) > 0 {
		planned.sparseExtents = make([]plannedContentRange, len(normalizedContent.SparseExtents))
		for i := range normalizedContent.SparseExtents {
			planned.sparseExtents[i] = plannedContentRange{
				offset: normalizedContent.SparseExtents[i].Offset,
				length: normalizedContent.SparseExtents[i].Length,
				seed:   normalizedContent.SparseExtents[i].Seed,
			}
		}
	}

	if len(normalizedContent.OverwriteExtents) > 0 {
		planned.overwriteExtents = make([]plannedContentRange, len(normalizedContent.OverwriteExtents))
		for i := range normalizedContent.OverwriteExtents {
			planned.overwriteExtents[i] = plannedContentRange{
				offset: normalizedContent.OverwriteExtents[i].Offset,
				length: normalizedContent.OverwriteExtents[i].Length,
				seed:   normalizedContent.OverwriteExtents[i].Seed,
			}
		}
	}

	return planned, nil
}

func plannedEntryTypeToScenarioManifestEntryType(typeID plannedEntryType) (ScenarioManifestEntryType, error) {
	switch typeID {
	case plannedEntryTypeDir:
		return ScenarioManifestEntryTypeDir, nil
	case plannedEntryTypeFile:
		return ScenarioManifestEntryTypeFile, nil
	case plannedEntryTypeSymlink:
		return ScenarioManifestEntryTypeSymlink, nil
	case plannedEntryTypeHardlink:
		return ScenarioManifestEntryTypeHardlink, nil
	case plannedEntryTypeSpecial:
		return ScenarioManifestEntryTypeSpecial, nil
	default:
		return "", fmt.Errorf("unsupported planned entry type %d", typeID)
	}
}

func scenarioManifestEntryTypeToPlannedEntryType(typeID ScenarioManifestEntryType) (plannedEntryType, error) {
	switch typeID {
	case ScenarioManifestEntryTypeDir:
		return plannedEntryTypeDir, nil
	case ScenarioManifestEntryTypeFile:
		return plannedEntryTypeFile, nil
	case ScenarioManifestEntryTypeSymlink:
		return plannedEntryTypeSymlink, nil
	case ScenarioManifestEntryTypeHardlink:
		return plannedEntryTypeHardlink, nil
	case ScenarioManifestEntryTypeSpecial:
		return plannedEntryTypeSpecial, nil
	default:
		return 0, fmt.Errorf("unsupported manifest entry type %q", typeID)
	}
}

func planPathToManifestPath(basePath, path string) (string, error) {
	relPath, err := filepath.Rel(filepath.Clean(basePath), filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("failed to derive relative planned path for %q: %w", path, err)
	}

	if relPath == "." {
		return "/", nil
	}

	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("planned path %q escapes base path %q", path, basePath)
	}

	manifestPath := "/" + filepath.ToSlash(relPath)
	normalizedPath, err := normalizeOperationPath(manifestPath, false)
	if err != nil {
		return "", err
	}

	return normalizedPath, nil
}

func tryTranslateAbsolutePathToManifestPath(basePath, path string) (string, bool, error) {
	if strings.TrimSpace(path) == "" {
		return "", false, fmt.Errorf("path must not be empty")
	}

	if !filepath.IsAbs(path) {
		return "", false, nil
	}

	relPath, err := filepath.Rel(filepath.Clean(basePath), filepath.Clean(path))
	if err != nil {
		return "", false, fmt.Errorf("failed to derive relative path for %q: %w", path, err)
	}

	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return "", false, nil
	}

	if relPath == "." {
		return "/", true, nil
	}

	manifestPath := "/" + filepath.ToSlash(relPath)
	normalizedPath, err := normalizeOperationPath(manifestPath, false)
	if err != nil {
		return "", false, err
	}

	return normalizedPath, true, nil
}
