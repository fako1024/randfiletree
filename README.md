# randfiletree
A file tree generator providing various (random and non-random) customization options

## Run Modes

Tree generation uses a deterministic plan/apply flow and supports explicit run modes:

- `RunModeAppend` (default): add planned entries while allowing already-existing matching paths.
- `RunModeStrict`: fail if any planned path already exists.
- `RunModeReplace`: clear base path before applying the plan.

Planning enforces unique generated paths with bounded retries. If unique path planning cannot
be completed, `Run()` returns `ErrPlanPathCollisionExhausted` instead of silently under-generating
the tree.

## Built-in Scenario Catalog

Use the name-based catalog entrypoint to select realistic backup edge-case trees without
manually wiring every low-level option:

- `BuildBuiltInScenario(name, seed)` returns a deterministic scenario spec
- `BuiltInScenarioCatalog()` returns all available scenario descriptors (intent,
  capabilities, prerequisites, pitfalls)
- scenario names are normalized (`hardlink heavy`, `hardlink_heavy`, and
  `hardlink-heavy` are equivalent)

Scenario contract:

- input: scenario `name` + deterministic `seed`
- output: `BuiltInScenarioSpec` with descriptor + fully configured `[]Option`
- deterministic behavior: same name+seed produces equivalent planning behavior
- capability handling: unsupported environments return explicit existing errors
  (for example Linux metadata/xattr/ACL unsupported diagnostics)

### Built-in Scenarios

- `hardlink-heavy`
  - stresses: inode sharing and hardlink group parity
  - prerequisites: hardlink creation support
  - pitfalls: mutating one linked path mutates all links in that inode group
- `symlink-cycle`
  - stresses: cycle-aware symlink traversal, chained links, dangling links
  - prerequisites: symlink creation support
  - pitfalls: recursive traversal must guard against cycles
- `metadata-heavy`
  - stresses: dense mode-bit + timestamp metadata handling
  - prerequisites: Linux nanosecond timestamp metadata support
  - pitfalls: non-Linux systems fail with explicit metadata unsupported errors
- `sparse-large`
  - stresses: sparse allocation and large logical-size replay semantics
  - prerequisites: enough free space for configured logical sizes
  - pitfalls: logical size and allocated blocks are intentionally different
- `xattr-acl-heavy`
  - stresses: xattr/ACL metadata parity
  - prerequisites: Linux filesystem with user xattr and POSIX ACL support
  - pitfalls: some filesystems/mounts disable ACL/xattr features by default
- `deterministic-coverage-low`, `deterministic-coverage-medium`,
  `deterministic-coverage-high`, `deterministic-coverage-xhigh`
  - stresses: every supported filesystem dimension, deterministically (no
    randomness anywhere in the plan)
  - prerequisites: capability-gated dimensions are skipped gracefully on
    unsupported platforms / unprivileged runs and reported in
    `DeterministicCoverageSpec.SkippedDimensions`
  - pitfalls: the `seed` argument is **ignored** — the same tree is produced
    for any seed value; privileged dimensions (char/block devices,
    `trusted.*` / `security.*` xattrs, non-effective uid/gid) require the
    `IncludePrivileged: true` opt-in via `WithDeterministicCoverage`

### E2E Usage Examples

Generic flow:

```go
basePath := filepath.Join(os.TempDir(), "randfiletree-hardlink")

scenarioSpec, err := randfiletree.BuildBuiltInScenario(randfiletree.ScenarioNameHardlinkHeavy, 42)
if err != nil {
	return err
}

g, err := randfiletree.NewWithOptions(basePath, scenarioSpec.Options...)
if err != nil {
	return err
}

if err := g.Run(); err != nil {
	return err
}
```

Select by catalog descriptor:

```go
for _, descriptor := range randfiletree.BuiltInScenarioCatalog() {
	scenarioSpec, err := randfiletree.BuildBuiltInScenario(descriptor.Name, 20260522)
	if err != nil {
		return err
	}

	g, err := randfiletree.NewWithOptions(filepath.Join(basePath, descriptor.Name), scenarioSpec.Options...)
	if err != nil {
		return err
	}

	if err := g.Run(); err != nil {
		return err
	}
}
```

### Deterministic Coverage Scenarios

The four `deterministic-coverage-*` catalog entries (and the dedicated
`RunDeterministicCoverage` entrypoint) produce a tree that exercises every
supported filesystem dimension by enumerating a pairwise covering array
across the configured capability set. Coverage is **complete already at
`low`**; the effort level only multiplies per-cell repetition to increase
stress without adding new dimensions.

Determinism contract:

- No `rand.Rand`, no `time.Now()`, no map-iteration order, no process state
  influences the synthesized plan.
- Same `(basePath, opts)` => byte-identical plan and (with `RunModeReplace`
  on the same `basePath`) byte-identical on-disk tree.
- All entries — including symlinks and special files — carry a fixed
  deterministic atime/mtime when the timestamp capability is enabled.

Pairwise dimensions exercised (subject to capability gates):

- mode classes (regular, exec, setuid, setgid, sticky, setuid+setgid)
- file content patterns (`Plain`, `DenseRandom`, `SparseHoles`,
  `RepeatedBlocks`, `PartialRangeOverwrite`)
- file size classes straddling the 64 KiB chunk boundary plus a multi-MiB
  sample
- byte-edge name classes (basic, leading spaces, trailing spaces, leading
  dots, NL/CR/TAB, control chars, invalid UTF-8, Unicode combining,
  near-`NameMaxBytes`)
- symlink strategies (absolute, relative, dangling, self-referential,
  chained, cycle)
- hardlink group sizes (2, 3, 4)
- special file types (FIFO, socket; char/block devices when privileged)
- xattr variants (user.* short / empty / large / binary; trusted.* /
  security.* when privileged)
- ACL variants (base, named-user+group+mask, default-on-dir)
- timestamp variants (epoch, fixed past/future, atime≷mtime)
- metadata bundles (none, ownership, timestamps, full)

Effort scale (entry counts on Linux with default capabilities):

| Effort | Multiplier | Approx entries (Linux, unprivileged) |
|---|---|---|
| `low`    | x1  | ~700    |
| `medium` | x3  | ~2,000  |
| `high`   | x10 | ~7,000  |
| `xhigh`  | x50 | ~33,000 |

All effort levels stay below the default `planEntryLimit` of 100,000.

Direct entrypoint:

```go
coverageSpec, metrics, err := randfiletree.RunDeterministicCoverage(basePath, randfiletree.DeterministicCoverageOptions{
    Effort:            randfiletree.DeterministicCoverageEffortLow,
    IncludeLinuxOnly:  true,
    IncludePrivileged: false,
    RunMode:           randfiletree.RunModeReplace,
})
if err != nil {
    return err
}

log.Printf("planned=%d applied=%d", metrics.Nodes, metrics.AppliedEntries)

for _, skipped := range coverageSpec.SkippedDimensions {
    log.Printf("coverage skipped %s: %s", skipped.Name, skipped.Reason)
}
```

`RunDeterministicCoverage` applies the generated deterministic plan immediately
and returns a `DeterministicCoverageSpec` report plus `RunMetrics`; it does not
return `[]Option`.

Generator mode (without catalog wrapper):

```go
g, err := randfiletree.NewWithOptions(basePath,
	randfiletree.WithRunMode(randfiletree.RunModeReplace),
	randfiletree.WithDeterministicCoverage(randfiletree.DeterministicCoverageOptions{
		Effort:            randfiletree.DeterministicCoverageEffortLow,
		IncludeLinuxOnly:  true,
		IncludePrivileged: false,
	}),
)
if err != nil {
	return err
}

if err := g.Run(); err != nil {
	return err
}
```

Catalog wrapper:

```go
scenarioSpec, err := randfiletree.BuildBuiltInScenario(
    randfiletree.ScenarioNameDeterministicCoverageLow,
    0, // seed is ignored for coverage scenarios
)
if err != nil {
    return err
}

g, err := randfiletree.NewWithOptions(basePath, scenarioSpec.Options...)
if err != nil {
    return err
}

if err := g.Run(); err != nil {
    return err
}
```

Caveats:

- ACL parity diffs (`diff.Options.CompareACLs`) and access-time parity
  (`CompareAccessTime`) are not strictly comparable on trees containing
  symlinks: Linux rejects ACL xattr reads on symlinks (EOPNOTSUPP), and
  atime is updated by the diff's own file reads. The coverage scenario
  still **applies** ACLs and timestamps; the limitation is in the
  comparison step.
- Absolute-strategy symlinks encode `basePath` verbatim, so two coverage
  trees built at **different** base paths will not byte-compare. Use the
  same base path with `RunModeReplace` (or compare plans rather than
  on-disk trees) for strict diff parity.

### Performance Harness

Large-scale planning and apply diagnostics are exposed via `RunWithMetrics(opts)`:

- `Nodes` - planned entry count
- `Retries` / `Collisions` - bounded unique-path retry pressure
- `HardlinkGroups` - planned hardlink topology group count
- `AppliedEntries` / `FinalizedDirectories` - apply-phase execution counts
- `PlanningElapsed`, `ApplyElapsed`, and total `Elapsed`

Planner memory growth is bounded by a configurable entry ceiling:

- default plan entry limit: `100000`
- override with `WithPlanEntryLimit(limit)`
- exceeding the limit fails deterministically with `ErrPlanEntryLimitExceeded`

Benchmark suites are available locally (not run in CI by default):

- generator scales: `BenchmarkGeneratorRunScales` (`performance_test.go`)
- diff scales: `BenchmarkDiffPathsScales` (`diff/performance_test.go`)
- run with alloc stats:
  - `go test -run '^$' -bench . -benchmem ./...`

Regression guardrail guidance:

- track `nodes/op`, `retries/op`, `collisions/op` and benchmark latency trends
- treat sustained increases in retries/collisions at the same scale as planning regressions
- compare benchmark output across commits on the same machine/profile

## Link Topologies

The generator supports first-class link topology controls:

- hard links via `WithHardlinkGenerator` / `WithHardlinkProbability`
- symlink strategy selection via `WithSymlinkStrategyGenerator` or weighted
  `WithSymlinkStrategyProbabilities`

Supported symlink strategies:

- `SymlinkStrategyAbsolute`
- `SymlinkStrategyRelative`
- `SymlinkStrategyDangling`
- `SymlinkStrategySelfReferential`
- `SymlinkStrategyChained`
- `SymlinkStrategyCycle`

`diff.Paths` validates both symlink target parity and hardlink topology parity.

## Linux Special Inode Generation

The generator supports Linux special inode scenarios to model realistic backup/restore inputs:

- global special-file probability via `WithSpecialFileGenerator(...)` or
  `WithSpecialFileProbability(...)`
- special-file type selection via `WithSpecialFileTypeGenerator(...)` or weighted
  `WithSpecialFileTypeProbabilities(...)`
- supported special types: FIFO, Unix socket path, char device, block device
- deterministic device numbers for char/block devices via
  `WithSpecialDeviceNumbers(...)` / `WithSpecialDeviceNumberGenerators(...)`

Important:

- special inode generation is Linux-only and returns explicit unsupported errors elsewhere
- char/block device creation may require elevated privileges (`mknod` capability/root)
- if char/block generation is selected, major and minor generators must both be configured

For configurable strictness, use `diff.PathsWithOptions`:

- `diff.DefaultOptions()` keeps compatibility with `diff.Paths`.
- `diff.StrictLinuxOptions()` enables ownership, access-time, and nanosecond timestamp checks.
- `Options` can independently toggle content hash, timestamp precision,
  ownership, access-time, hardlink topology, and future xattr/ACL comparator hooks.
- diff now includes inode-type parity, and for char/block devices compares major/minor numbers.

## Cross-Device and Mount-Boundary Scenarios (Linux)

The package provides Linux helpers to exercise cross-device semantics explicitly:

- `SetupCrossDeviceScenario(basePath)` sets up sibling roots (`left`/`right`) and
  attempts to ensure they are on distinct devices
- mount helpers:
  - `MountTmpfs(target, sizeBytes)`
  - `MountBind(source, target)`
  - `Unmount(target)`

Cross-device scenario setup is capability-aware:

- prefers `tmpfs` mount for the secondary root
- falls back to bind-mounting a distinct-device source where available
- validates `basePath` and existing parent components as real directories (symlink paths are rejected)
- returns explicit unavailable errors when mount privileges or features are missing

Resource cleanup is explicit:

- call `scenario.Close()` to tear down mounts and temporary bind sources
- if teardown fails, cleanup state is preserved so `Close()` can be retried

This allows deterministic validation of Linux mount-boundary semantics such as:

- rename across devices => `EXDEV`
- hardlink creation across devices rejected (`EXDEV`)

`diff.PathsWithOptions` also supports optional device-ID parity checks via:

- `diff.Options{CompareDeviceIDs: true}`

`CompareDeviceIDs` defaults to `false` to preserve historical behavior.

## Filesystem-Specific Feature Scenarios (Linux)

The package provides opt-in Linux scenario helpers for filesystem-specific
behavior:

- immutable flag scenario (`FilesystemFeatureImmutable`)
- append-only flag scenario (`FilesystemFeatureAppendOnly`)
- reflink clone scenario (`FilesystemFeatureReflink`)

Use `SetupFilesystemFeatureScenario(basePath, features...)` to build explicit
fixtures, and `ProbeFilesystemFeatures(basePath, features...)` to run
capability-aware probes before setup.

Important:

- feature scenarios are Linux-only and return explicit unsupported diagnostics
  elsewhere
- setup is explicit opt-in; no filesystem-specific behavior is enabled by
  default
- probe/setup failures classify unavailable features deterministically
  (`permission-denied`, `unsupported`, `unavailable`)
- scenario cleanup should call `(*FilesystemFeatureScenario).Close()` to
  restore mutable inode flags

## Linux Metadata Controls

The generator can now apply Linux metadata controls for created files and
directories:

- ownership (`uid`/`gid`) via `WithOwnership(...)` / `WithOwnershipGenerators(...)`
- nanosecond timestamp control for `atime`/`mtime` via
  `WithTimestamps(...)` / `WithTimestampGenerators(...)`
- special mode bits (`setuid`, `setgid`, `sticky`) through existing mode
  generators (`WithFileModeGenerator`, `WithDirModeGenerator`)

Important:

- metadata controls are Linux-focused; non-Linux systems return explicit
  unsupported errors when ownership/timestamp metadata controls are requested
- ownership updates may require elevated capabilities; insufficient privileges
  fail with explicit errors
- `ctime` cannot be set directly from user space and is intentionally not
  configurable

## Sparse and Large File Content Patterns

The generator supports deterministic content pattern strategies for large files:

- pattern selection via `WithContentPattern(...)`,
  `WithContentPatternGenerator(...)`, or weighted
  `WithContentPatternProbabilities(...)`
- logical file sizes via `WithContentLogicalSize(...)`,
  `WithContentLogicalSizeGenerator(...)`, or `WithContentLogicalSizeRange(...)`
- supported patterns:
  - `ContentPatternDenseRandom`
  - `ContentPatternSparseHoles`
  - `ContentPatternRepeatedBlocks`
  - `ContentPatternPartialRangeOverwrite`

Important:

- content-pattern mode requires both a pattern generator and a logical-size
  generator
- large logical files are written in bounded chunks; full-file in-memory buffers
  are avoided
- repeated-block pattern reuses `WithDataGenerator(...)` output as the repeated
  block when configured
- sparse parity checks are available in diff via
  `diff.Options{CompareSparseness: true}` and compare sparse-vs-dense parity,
  not exact allocated block counts, to reduce filesystem noise
- sparseness comparison currently requires Linux metadata collection

## Mutation Engine

`randfiletree` supports deterministic mutation streams for multi-snapshot tests.

- Define explicit `Operation` values and apply them with `ApplyOperations(basePath, ops)`.
- Use `ApplyOperationsWithOptions(basePath, ops, opts)` for deterministic
  resume/retry and fault-injection runs.
- Generate deterministic streams from a baseline tree with
  `GenerateOperations(basePath, opts)` or `(*Generator).GenerateOperations(opts)`.
- Export replay specs for CI diagnostics with `ExportOperationSpec(ops)` and parse
  with `ParseOperationSpec(spec)`.

Deterministic fault injection is opt-in via `FaultProfile` rules:

- fail at Nth matching execution point (`FaultRule{Nth: ...}`)
- scope matching (`FaultScopeMutation`, `FaultScopeRun`, or `FaultScopeAny`)
- optional operation kind and path pattern matching
- injected failures return `FaultInjectionError` with scope, kind, path, and index

Mutation resume/retry hooks:

- set `OperationApplyOptions{StartIndex: i}` to continue operation execution from
  `ops[i]` after a partial failure
- `StartIndex` must be within `[0, len(ops)]`

## Scenario Manifest Import/Export

Portable scenario manifests provide deterministic replay across machines and CI runs.

- build manifest payloads from a configured generator + operation stream with
  `BuildScenarioManifest(...)` / `(*Generator).BuildScenarioManifest(...)`
- export/import JSON manifests with `ExportScenarioManifestJSON(...)`,
  `ParseScenarioManifestJSON(...)`, and `ApplyScenarioManifestJSON(...)`
- export/import YAML manifests with `ExportScenarioManifestYAML(...)`,
  `ParseScenarioManifestYAML(...)`, and `ApplyScenarioManifestYAML(...)`
- apply typed manifests directly with `ApplyScenarioManifest(basePath, manifest)`

Manifest behavior:

- manifest format is versioned (`version`) and fails explicitly for unsupported
  versions
- integrity is verified with a deterministic checksum before apply; corruption
  or payload tampering returns explicit checksum mismatch errors
- required capability declarations are validated and enforced before replay
  execution
- manifests include `deterministicSettings` metadata for future non-random
  deterministic controls without breaking schema compatibility

Supported operation kinds include create (file/dir/symlink/hardlink), delete,
rename, chmod/chown, truncate, append, overwrite-range, set-xattr, and remove-xattr.

`set-xattr` and `remove-xattr` are fully supported on Linux and return explicit
unsupported errors on non-Linux platforms.

## Linux XAttr and ACL Metadata

The generator can apply xattrs and ACL metadata for created files and directories on Linux.

- xattrs via `WithXAttr(...)`, `WithXAttrsFixed(...)`, or `WithXAttrValueGenerator(...)`
- optional namespace opt-in for `trusted.*` and `security.*` via
  `WithTrustedXAttrNamespace(true)` / `WithSecurityXAttrNamespace(true)`
- ACL entries via `WithACL(...)` or `WithACLGenerator(...)`

Important:

- Linux-only behavior for xattr/ACL metadata controls; non-Linux returns explicit unsupported errors
- xattr and ACL capability failures are explicit (permission denied, unsupported filesystem)
- `diff.PathsWithOptions` can now compare xattr and ACL parity deterministically with
  `CompareXAttrs` and `CompareACLs` options

## Byte-Accurate Name Generation

The generator supports byte-level filename generation to cover Linux path edge cases
that are common sources of backup/restore failures.

- byte-level generators via `WithByteFileNameGenerator(...)` and `WithByteDirNameGenerator(...)`
- `ByteNameGeneratorAlphabet(alphabet []byte)` for custom byte alphabets
- named edge-case presets:
  - `ByteNamePresetLeadingSpaces` - names starting with spaces
  - `ByteNamePresetTrailingSpaces` - names ending with spaces
  - `ByteNamePresetLeadingDots` - hidden-file-style names with leading dots
  - `ByteNamePresetNewlineTab` - names containing `\n`, `\r`, `\t`
  - `ByteNamePresetControlChars` - names with ASCII control characters (0x01-0x1F)
  - `ByteNamePresetInvalidUTF8` - names with invalid UTF-8 byte sequences
  - `ByteNamePresetUnicodeNormalization` - names with combining characters

Important:

- byte generators produce raw byte strings that may contain invalid UTF-8
- NUL (`\x00`) and slash (`/`) bytes are never generated (Linux path constraints)
- `NameMaxBytes` (255) and `PathMaxBytes` (4096) constants are exported for boundary testing
- diff error output escapes non-printable bytes as `\xNN` for readability
- edge-case names are deterministic under fixed seed
