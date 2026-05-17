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

For configurable strictness, use `diff.PathsWithOptions`:

- `diff.DefaultOptions()` keeps compatibility with `diff.Paths`.
- `diff.StrictLinuxOptions()` enables ownership, access-time, and nanosecond timestamp checks.
- `Options` can independently toggle content hash, timestamp precision,
  ownership, access-time, hardlink topology, and future xattr/ACL comparator hooks.

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

## Mutation Engine

`randfiletree` supports deterministic mutation streams for multi-snapshot tests.

- Define explicit `Operation` values and apply them with `ApplyOperations(basePath, ops)`.
- Generate deterministic streams from a baseline tree with
  `GenerateOperations(basePath, opts)` or `(*Generator).GenerateOperations(opts)`.
- Export replay specs for CI diagnostics with `ExportOperationSpec(ops)` and parse
  with `ParseOperationSpec(spec)`.

Supported operation kinds include create (file/dir/symlink/hardlink), delete,
rename, chmod/chown, truncate, append, overwrite-range, and xattr placeholders.

`set-xattr` and `remove-xattr` are currently placeholders and return
`ErrXAttrPlaceholderUnsupported` when applied.
