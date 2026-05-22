package randfiletree

import (
	"fmt"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// ExportScenarioManifestYAML serializes a replay manifest to YAML.
func ExportScenarioManifestYAML(manifest ScenarioManifest) (string, error) {
	normalized, err := normalizeScenarioManifest(manifest)
	if err != nil {
		return "", err
	}

	sealed, err := sealScenarioManifestIntegrity(normalized)
	if err != nil {
		return "", err
	}

	payload, err := yaml.Marshal(sealed)
	if err != nil {
		return "", fmt.Errorf("failed to serialize scenario manifest YAML: %w", err)
	}

	return string(payload), nil
}

// ParseScenarioManifestYAML parses and validates a replay manifest from YAML.
func ParseScenarioManifestYAML(payload string) (ScenarioManifest, error) {
	if strings.TrimSpace(payload) == "" {
		return ScenarioManifest{}, ErrScenarioManifestPayloadEmpty
	}

	var manifest ScenarioManifest
	if err := yaml.Unmarshal([]byte(payload), &manifest); err != nil {
		return ScenarioManifest{}, fmt.Errorf("failed to parse scenario manifest YAML: %w", err)
	}

	normalized, err := normalizeScenarioManifest(manifest)
	if err != nil {
		return ScenarioManifest{}, err
	}

	if err := verifyScenarioManifestIntegrity(normalized); err != nil {
		return ScenarioManifest{}, err
	}

	return normalized, nil
}

// ApplyScenarioManifestYAML parses and applies a replay manifest from YAML.
func ApplyScenarioManifestYAML(basePath, payload string) error {
	manifest, err := ParseScenarioManifestYAML(payload)
	if err != nil {
		return err
	}

	return ApplyScenarioManifest(basePath, manifest)
}

// ApplyScenarioManifestYAML parses and applies a replay manifest from YAML to the generator base path.
func (g *Generator) ApplyScenarioManifestYAML(payload string) error {
	if g == nil {
		return ErrNilGenerator
	}

	return ApplyScenarioManifestYAML(g.basePath, payload)
}
