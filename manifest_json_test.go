package randfiletree

import (
	"testing"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/require"
)

func TestScenarioManifestJSONRoundTripAndApply(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	g := newManifestConfiguredGenerator(t, base)

	manifest, err := g.BuildScenarioManifest([]Operation{{
		Kind: OperationKindCreateFile,
		Path: "/a.txt",
		Mode: 0o600,
		Data: []byte("a"),
	}})
	require.NoError(t, err)

	payloadA, err := ExportScenarioManifestJSON(manifest)
	require.NoError(t, err)
	payloadB, err := ExportScenarioManifestJSON(manifest)
	require.NoError(t, err)
	require.Equal(t, payloadA, payloadB)

	parsed, err := ParseScenarioManifestJSON(payloadA)
	require.NoError(t, err)
	require.Equal(t, manifest, parsed)

	applyBase := t.TempDir()
	require.NoError(t, ApplyScenarioManifestJSON(applyBase, payloadA))
}

func TestScenarioManifestJSONRejectsEmptyPayload(t *testing.T) {
	t.Parallel()

	_, err := ParseScenarioManifestJSON("  ")
	require.ErrorIs(t, err, ErrScenarioManifestPayloadEmpty)
}

func TestScenarioManifestJSONRejectsCorruption(t *testing.T) {
	t.Parallel()

	g := newManifestConfiguredGenerator(t, t.TempDir())
	manifest, err := g.BuildScenarioManifest(nil)
	require.NoError(t, err)

	payload, err := ExportScenarioManifestJSON(manifest)
	require.NoError(t, err)

	parsed, err := ParseScenarioManifestJSON(payload)
	require.NoError(t, err)

	parsed.Integrity.Checksum = "00"
	rawPayload, err := jsoniter.Marshal(parsed)
	require.NoError(t, err)

	_, err = ParseScenarioManifestJSON(string(rawPayload))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrScenarioManifestChecksumMismatch)
}

func TestScenarioManifestJSONRejectsUnsupportedChecksumAlgorithm(t *testing.T) {
	t.Parallel()

	g := newManifestConfiguredGenerator(t, t.TempDir())
	manifest, err := g.BuildScenarioManifest(nil)
	require.NoError(t, err)

	payload, err := ExportScenarioManifestJSON(manifest)
	require.NoError(t, err)

	parsed, err := ParseScenarioManifestJSON(payload)
	require.NoError(t, err)

	parsed.Integrity.Algorithm = "sha1"
	rawPayload, err := jsoniter.Marshal(parsed)
	require.NoError(t, err)

	_, err = ParseScenarioManifestJSON(string(rawPayload))
	require.Error(t, err)
	require.ErrorIs(t, err, ErrScenarioManifestChecksumAlgorithmUnsupported)
}

func TestScenarioManifestJSONGeneratorApplyReceiver(t *testing.T) {
	t.Parallel()

	g := newManifestConfiguredGenerator(t, t.TempDir())
	manifest, err := g.BuildScenarioManifest(nil)
	require.NoError(t, err)

	payload, err := ExportScenarioManifestJSON(manifest)
	require.NoError(t, err)

	replay := New(t.TempDir())
	require.NoError(t, replay.ApplyScenarioManifestJSON(payload))

	var nilGen *Generator
	err = nilGen.ApplyScenarioManifestJSON(payload)
	require.ErrorIs(t, err, ErrNilGenerator)
}
