//go:build !linux

package randfiletree

import "fmt"

func probeFilesystemFeatures(basePath string, features []FilesystemFeature) ([]FilesystemFeatureStatus, error) {
	statuses := make([]FilesystemFeatureStatus, 0, len(features))
	for _, feature := range features {
		statuses = append(statuses, FilesystemFeatureStatus{
			Feature:      feature,
			Availability: FilesystemFeatureAvailabilityUnsupported,
			Diagnostic: fmt.Sprintf(
				"failed to probe filesystem feature `%s` for base `%s`: %v",
				feature,
				basePath,
				ErrFilesystemFeatureScenarioLinuxOnly,
			),
		})
	}

	return statuses, nil
}

func setupFilesystemFeatureScenario(basePath string, features []FilesystemFeature) (*FilesystemFeatureScenario, error) {
	_ = features

	return nil, fmt.Errorf("failed to setup filesystem feature scenario for base `%s`: %w", basePath, ErrFilesystemFeatureScenarioLinuxOnly)
}

func restoreFilesystemFeatureFlags(restore filesystemFlagRestore) error {
	_ = restore

	return nil
}
