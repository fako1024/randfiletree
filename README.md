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
- Generate deterministic streams from a baseline tree with
  `GenerateOperations(basePath, opts)` or `(*Generator).GenerateOperations(opts)`.
- Export replay specs for CI diagnostics with `ExportOperationSpec(ops)` and parse
  with `ParseOperationSpec(spec)`.

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
