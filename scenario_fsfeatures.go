package randfiletree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FilesystemFeature denotes an opt-in filesystem-specific scenario feature.
type FilesystemFeature uint8

const (
	FilesystemFeatureImmutable FilesystemFeature = iota + 1
	FilesystemFeatureAppendOnly
	FilesystemFeatureReflink
)

func (f FilesystemFeature) String() string {
	switch f {
	case FilesystemFeatureImmutable:
		return "immutable"
	case FilesystemFeatureAppendOnly:
		return "append-only"
	case FilesystemFeatureReflink:
		return "reflink"
	default:
		return fmt.Sprintf("unknown(%d)", f)
	}
}

func validateFilesystemFeature(feature FilesystemFeature) error {
	switch feature {
	case FilesystemFeatureImmutable,
		FilesystemFeatureAppendOnly,
		FilesystemFeatureReflink:
		return nil
	default:
		return fmt.Errorf("invalid filesystem feature %d", feature)
	}
}

func allFilesystemFeatures() []FilesystemFeature {
	return []FilesystemFeature{
		FilesystemFeatureImmutable,
		FilesystemFeatureAppendOnly,
		FilesystemFeatureReflink,
	}
}

// FilesystemFeatureAvailability denotes filesystem feature availability.
type FilesystemFeatureAvailability string

const (
	FilesystemFeatureAvailabilityAvailable        FilesystemFeatureAvailability = "available"
	FilesystemFeatureAvailabilityPermissionDenied FilesystemFeatureAvailability = "permission-denied"
	FilesystemFeatureAvailabilityUnsupported      FilesystemFeatureAvailability = "unsupported"
	FilesystemFeatureAvailabilityUnavailable      FilesystemFeatureAvailability = "unavailable"
)

// FilesystemFeatureStatus reports feature availability and diagnostics.
type FilesystemFeatureStatus struct {
	Feature      FilesystemFeature
	Availability FilesystemFeatureAvailability
	Diagnostic   string
}

// IsAvailable reports whether the requested feature is available.
func (s FilesystemFeatureStatus) IsAvailable() bool {
	return s.Availability == FilesystemFeatureAvailabilityAvailable
}

type filesystemFlagRestore struct {
	feature FilesystemFeature
	path    string
	flags   int
}

// FilesystemFeatureScenario describes an opt-in filesystem feature test fixture.
type FilesystemFeatureScenario struct {
	BasePath string

	ImmutablePath     string
	AppendOnlyPath    string
	ReflinkSourcePath string
	ReflinkClonePath  string

	FeatureStatus []FilesystemFeatureStatus

	flagRestores []filesystemFlagRestore
}

// ProbeFilesystemFeatures probes requested filesystem features for basePath.
//
// When no features are provided, all known filesystem features are probed.
func ProbeFilesystemFeatures(basePath string, features ...FilesystemFeature) ([]FilesystemFeatureStatus, error) {
	cleanBasePath, normalizedFeatures, err := prepareFilesystemFeatureInputs(basePath, features, true)
	if err != nil {
		return nil, err
	}

	return probeFilesystemFeatures(cleanBasePath, normalizedFeatures)
}

// SetupFilesystemFeatureScenario initializes a deterministic filesystem-feature scenario.
func SetupFilesystemFeatureScenario(basePath string, features ...FilesystemFeature) (*FilesystemFeatureScenario, error) {
	cleanBasePath, normalizedFeatures, err := prepareFilesystemFeatureInputs(basePath, features, false)
	if err != nil {
		return nil, err
	}

	return setupFilesystemFeatureScenario(cleanBasePath, normalizedFeatures)
}

// Close tears down mutable feature state created for the scenario.
func (s *FilesystemFeatureScenario) Close() error {
	if s == nil {
		return nil
	}

	var errs []error

	for i := len(s.flagRestores) - 1; i >= 0; i-- {
		if err := restoreFilesystemFeatureFlags(s.flagRestores[i]); err != nil {
			errs = append(errs, err)
		}
	}

	s.flagRestores = nil

	if len(errs) == 0 {
		return nil
	}

	return errors.Join(errs...)
}

func prepareFilesystemFeatureInputs(basePath string, features []FilesystemFeature, allowEmpty bool) (string, []FilesystemFeature, error) {
	if strings.TrimSpace(basePath) == "" {
		return "", nil, ErrBasePathEmpty
	}

	normalizedFeatures, err := normalizeFilesystemFeatures(features, allowEmpty)
	if err != nil {
		return "", nil, err
	}

	cleanBasePath := filepath.Clean(basePath)
	if err := os.MkdirAll(cleanBasePath, 0o750); err != nil {
		return "", nil, fmt.Errorf("failed to prepare filesystem feature base path `%s`: %w", cleanBasePath, err)
	}

	return cleanBasePath, normalizedFeatures, nil
}

func normalizeFilesystemFeatures(features []FilesystemFeature, allowEmpty bool) ([]FilesystemFeature, error) {
	if len(features) == 0 {
		if !allowEmpty {
			return nil, ErrFilesystemFeatureSelectionEmpty
		}

		return allFilesystemFeatures(), nil
	}

	featureSet := make(map[FilesystemFeature]struct{}, len(features))
	for i, feature := range features {
		if err := validateFilesystemFeature(feature); err != nil {
			return nil, fmt.Errorf("filesystem feature at index %d: %w", i, err)
		}

		featureSet[feature] = struct{}{}
	}

	normalized := make([]FilesystemFeature, 0, len(featureSet))
	for feature := range featureSet {
		normalized = append(normalized, feature)
	}

	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i] < normalized[j]
	})

	return normalized, nil
}

func ensureFilesystemFeatureAvailability(basePath string, statuses []FilesystemFeatureStatus) error {
	unavailable := make([]FilesystemFeatureStatus, 0)
	for _, status := range statuses {
		if status.IsAvailable() {
			continue
		}

		unavailable = append(unavailable, status)
	}

	if len(unavailable) == 0 {
		return nil
	}

	parts := make([]string, 0, len(unavailable))
	for _, status := range unavailable {
		reason := strings.TrimSpace(status.Diagnostic)
		if reason == "" {
			reason = "no diagnostic provided"
		}

		parts = append(parts, fmt.Sprintf("%s=%s (%s)", status.Feature, status.Availability, reason))
	}

	return fmt.Errorf(
		"%w for base `%s`: %s",
		ErrFilesystemFeatureScenarioUnavailable,
		basePath,
		strings.Join(parts, "; "),
	)
}

func classifyFilesystemFeatureAvailability(err error) FilesystemFeatureAvailability {
	switch {
	case err == nil:
		return FilesystemFeatureAvailabilityAvailable
	case errors.Is(err, ErrFilesystemFeaturePermissionDenied):
		return FilesystemFeatureAvailabilityPermissionDenied
	case errors.Is(err, ErrFilesystemFeatureUnsupported):
		return FilesystemFeatureAvailabilityUnsupported
	default:
		return FilesystemFeatureAvailabilityUnavailable
	}
}

func statusFromFilesystemFeatureError(feature FilesystemFeature, err error) FilesystemFeatureStatus {
	status := FilesystemFeatureStatus{
		Feature:      feature,
		Availability: classifyFilesystemFeatureAvailability(err),
	}

	if err != nil {
		status.Diagnostic = err.Error()
	}

	return status
}
