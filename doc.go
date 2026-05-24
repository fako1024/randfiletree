// Package randfiletree generates deterministic and randomized filesystem
// trees for backup, replication, and migration testing.
//
// The package centers on Generator, which produces a directory tree under a
// configurable base path via a plan/apply flow (see plan.go). Common
// entrypoints are:
//
//   - BuildBuiltInScenario: name-based catalog of pre-wired scenarios
//     covering hardlinks, sparse files, xattrs, ACLs, cross-device mounts,
//     and filesystem-specific features.
//   - WithDeterministicCoverage / RunDeterministicCoverage: synthesize a
//     deterministic capability-coverage plan independent of random state.
//   - GenerateOperations / ApplyOperations: the mutation engine for
//     deriving or replaying change sets on an existing tree.
//   - BuildScenarioManifest plus ExportScenarioManifest{JSON,YAML} and
//     ApplyScenarioManifest{JSON,YAML}: canonical replay manifests with
//     checksum-verified integrity.
//
// Run modes (RunModeAppend, RunModeStrict, RunModeReplace) control how a
// plan is applied to a pre-existing base path. Linux-specific behavior
// (xattrs, ACLs, mount boundaries, filesystem features) is split across
// *_linux.go / *_other.go files and is capability-gated; the registry in
// coverage_capabilities.go declares each gate's ID and prerequisites.
//
// See README.md in the module root for end-to-end examples, scenario
// catalog details, and per-feature documentation. The sibling package
// github.com/fako1024/randfiletree/diff verifies tree parity.
package randfiletree
