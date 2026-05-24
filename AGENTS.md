# AGENTS.md

## Repo Shape
- Go module: `github.com/fako1024/randfiletree` (`go 1.21`).
- Main packages: root `randfiletree` (generator/mutation/manifest/scenario APIs) and `diff` (tree parity checks). Both packages have `doc.go` overviews; consult them before grepping for entrypoints.
- Internal Linux ACL codec lives in `internal/aclxattr`; keep it internal-only.
- `README.md` is the narrative reference for scenarios, mutation engine, manifests, metadata, sparse files, and xattr/ACL — consult it before re-deriving usage from code.

## Commands (No Task Runner)
- CI-equivalent sequence (matches `.github/workflows/go.yml`, which runs the same steps on Linux, macOS, and Windows):
  - `go build -v -x ./...`
  - `go test -v ./...`
- No linter gates CI; run `go vet ./...` (and optionally `golangci-lint run ./...`) locally before submitting.
- Fast local loop:
  - `go test ./...`
  - single test: `go test ./... -run '^TestName$'`
  - single package: `go test ./diff -run '^TestName$'`
- Benchmarks (local only; not in CI): `go test -run '^$' -bench . -benchmem ./...`

## Execution Model (Root Package)
- `Generator.Run()` is plan/apply: planning and apply logic are both in `plan.go` (`planRun`, `applyRunPlan`).
- Default run mode is `RunModeAppend`; `RunModeStrict` fails on pre-existing planned paths; `RunModeReplace` clears `basePath` first.
- Planner uniqueness is bounded; path-collision exhaustion returns `ErrPlanPathCollisionExhausted` (not partial silent output).
- Manual `Generator` setup must be complete; missing required generators fail with `generator configuration incomplete...`.
- Built-in scenarios (`BuildBuiltInScenario`) are the safest way to get a fully configured generator quickly.

## Deterministic Coverage / Scenarios
- Coverage entrypoints (`WithDeterministicCoverage`, `RunDeterministicCoverage`) bypass random generator state and synthesize deterministic plans.
- Built-in `deterministic-coverage-*` scenarios ignore the `seed` argument by design.
- For strict reproducibility checks, compare two runs on the same `basePath` (coverage includes absolute symlink targets).

## Platform & Capability Gotchas
- Linux-specific behavior is split via `*_linux.go` / `*_other.go`; non-Linux implementations return explicit unsupported errors.
- Metadata/xattr/ACL/mount/cross-device/filesystem-feature helpers are capability-gated; expect skips/errors on unsupported environments.
- The capability registry in `coverage_capabilities.go` declares each gate's ID, platform requirement, and root-privilege need — start there when diagnosing unexpected skips.
- Some Linux tests skip when run as root (guarded by `os.Geteuid() == 0`) because the scenarios require an unprivileged uid.

## Diff & Metadata Gotchas
- `diff.Paths()` is a compatibility wrapper over `diff.PathsWithOptions(..., diff.DefaultOptions())`.
- `diff.StrictLinuxOptions()` enables stricter ownership/access-time/nanosecond checks.
- `CompareXAttrs`, `CompareACLs`, `CompareSparseness`, and `CompareDeviceIDs` depend on platform metadata collection and fail explicitly when unavailable.

## Mutation / Manifest Contracts
- Operation paths are virtual rooted paths like `/dir/file` relative to `basePath`, not host-absolute paths.
- Path validation rejects base-path escapes and symlink-parent traversal.
- Manifests are normalized + checksummed; use `Export/Parse/ApplyScenarioManifest{JSON,YAML}` helpers instead of hand-built payloads.
- Manifest checksum compatibility is pinned by tests (`manifest_test.go`); schema/serialization changes require deliberate versioning updates.

## Existing Code Conventions
- JSON serialization uses `github.com/json-iterator/go` (not `encoding/json`).
- Tests use `github.com/stretchr/testify/require` heavily; follow that style in new tests.
