package randfiletree

import (
	"fmt"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

// ExportScenarioManifestJSON serializes a replay manifest to canonical JSON.
func ExportScenarioManifestJSON(manifest ScenarioManifest) (string, error) {
	normalized, err := normalizeScenarioManifest(manifest)
	if err != nil {
		return "", err
	}

	sealed, err := sealScenarioManifestIntegrity(normalized)
	if err != nil {
		return "", err
	}

	payload, err := jsoniter.Marshal(sealed)
	if err != nil {
		return "", fmt.Errorf("failed to serialize scenario manifest JSON: %w", err)
	}

	return string(payload), nil
}

// ParseScenarioManifestJSON parses and validates a replay manifest from JSON.
func ParseScenarioManifestJSON(payload string) (ScenarioManifest, error) {
	if strings.TrimSpace(payload) == "" {
		return ScenarioManifest{}, ErrScenarioManifestPayloadEmpty
	}

	var manifest ScenarioManifest
	if err := jsoniter.Unmarshal([]byte(payload), &manifest); err != nil {
		return ScenarioManifest{}, fmt.Errorf("failed to parse scenario manifest JSON: %w", err)
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

// ApplyScenarioManifestJSON parses and applies a replay manifest from JSON.
func ApplyScenarioManifestJSON(basePath, payload string) error {
	manifest, err := ParseScenarioManifestJSON(payload)
	if err != nil {
		return err
	}

	return ApplyScenarioManifest(basePath, manifest)
}

// ApplyScenarioManifestJSON parses and applies a replay manifest from JSON to the generator base path.
func (g *Generator) ApplyScenarioManifestJSON(payload string) error {
	if g == nil {
		return ErrNilGenerator
	}

	return ApplyScenarioManifestJSON(g.basePath, payload)
}
