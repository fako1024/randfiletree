package randfiletree

import (
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/require"
)

func TestScenarioManifestYAMLRoundTripAndApply(t *testing.T) {
	t.Parallel()

	g := newManifestConfiguredGenerator(t, t.TempDir())
	manifest, err := g.BuildScenarioManifest(nil)
	require.NoError(t, err)

	payloadA, err := ExportScenarioManifestYAML(manifest)
	require.NoError(t, err)
	payloadB, err := ExportScenarioManifestYAML(manifest)
	require.NoError(t, err)
	require.Equal(t, payloadA, payloadB)

	parsed, err := ParseScenarioManifestYAML(payloadA)
	require.NoError(t, err)
	require.Equal(t, manifest, parsed)

	replay := New(t.TempDir())
	require.NoError(t, replay.ApplyScenarioManifestYAML(payloadA))
}

func TestScenarioManifestYAMLRejectsEmptyPayload(t *testing.T) {
	t.Parallel()

	_, err := ParseScenarioManifestYAML("\n\t  ")
	require.ErrorIs(t, err, ErrScenarioManifestPayloadEmpty)
}

func TestScenarioManifestYAMLRejectsCorruption(t *testing.T) {
	t.Parallel()

	g := newManifestConfiguredGenerator(t, t.TempDir())
	manifest, err := g.BuildScenarioManifest(nil)
	require.NoError(t, err)

	tampered := manifest
	tampered.Integrity.Checksum = "00"

	rawPayload, err := jsoniter.Marshal(tampered)
	require.NoError(t, err)

	_, err = ParseScenarioManifestJSON(string(rawPayload))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrScenarioManifestChecksumMismatch)
}

func TestScenarioManifestYAMLRejectsUnsupportedChecksumAlgorithm(t *testing.T) {
	t.Parallel()

	g := newManifestConfiguredGenerator(t, t.TempDir())
	manifest, err := g.BuildScenarioManifest(nil)
	require.NoError(t, err)

	tampered := manifest
	tampered.Integrity.Algorithm = "sha1"

	rawPayload, err := jsoniter.Marshal(tampered)
	require.NoError(t, err)

	_, err = ParseScenarioManifestJSON(string(rawPayload))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrScenarioManifestChecksumAlgorithmUnsupported)
}

func TestScenarioManifestYAMLGeneratorApplyNilReceiver(t *testing.T) {
	t.Parallel()

	g := newManifestConfiguredGenerator(t, t.TempDir())
	manifest, err := g.BuildScenarioManifest(nil)
	require.NoError(t, err)

	payload, err := ExportScenarioManifestYAML(manifest)
	require.NoError(t, err)

	var nilGen *Generator
	err = nilGen.ApplyScenarioManifestYAML(payload)
	require.ErrorIs(t, err, ErrNilGenerator)
}
