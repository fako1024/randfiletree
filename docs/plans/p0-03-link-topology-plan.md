# [P0-03] Complete Link Topology Support - Implementation Plan

Issue: #3  
Branch: `feature/3-p0-03-complete-link-topology-support`

## Objective

Add first-class, deterministic support for complete link topology generation and verification:

- hard link creation and hardlink-group tracking
- explicit symlink topology strategies
- diff parity checks for symlink target and hardlink relationship semantics
- bounded behavior that avoids accidental infinite recursion while still allowing intentional cycles

## Current Baseline (from `main`)

- Generator supports regular files, directories, and basic symlink creation paths.
- Planner/apply flow exists and is deterministic for seed + options.
- Diff checks path/mode/time/hash/symlink target but does not validate hardlink topology.
- Existing symlink knobs (`WithSymlinkProbability`, `WithRelativeSymlinkProbability`) are binary and do not encode richer topology intent.

## Scope and Compatibility

### In Scope

1. Add hardlink generation for regular files.
2. Add explicit symlink topology strategies:
   - absolute
   - relative
   - dangling
   - self-referential
   - chained
   - bounded cycle
3. Extend plan/runtime metadata to preserve hardlink group intent.
4. Extend `diff.Paths` to fail on hardlink topology mismatches.
5. Add deterministic tests for all strategies and parity checks.

### Out of Scope

- Full diff v2 option surface from issue #2.
- Ownership/xattr/ACL/special inode work from other issues.
- Non-deterministic or unbounded stress harness changes.

### Compatibility Rules

- Keep `New(...)`, `Run()`, and `diff.Paths(a, b)` public signatures unchanged.
- Preserve current symlink option behavior unless explicit strategy mode is configured.
- Default behavior remains backward compatible for existing users.

## Design

## 1) Link Topology Types

Add a new `links.go` in root package with:

- `type SymlinkStrategy uint8`
- constants for each strategy
- `type SymlinkStrategyGenerator func(*rand.Rand) SymlinkStrategy`
- weighted helper:
  - `SymlinkStrategyGeneratorProbabilityWeighted(map[SymlinkStrategy]float64)`

Validation:

- reject invalid strategy values
- reject empty/invalid strategy-probability maps
- deterministic strategy selection order (sorted keys)

## 2) Generator Configuration Surface

Extend `Generator` with:

- `hardlinkProbGen BooleanGenerator`
- `symlinkStrategyGen SymlinkStrategyGenerator`

Add options in `options.go`:

- `WithHardlinkGenerator(BooleanGenerator)`
- `WithHardlinkProbability(float64)`
- `WithSymlinkStrategyGenerator(SymlinkStrategyGenerator)`
- `WithSymlinkStrategyProbabilities(map[SymlinkStrategy]float64)`

Validation behavior:

- non-nil generators
- probability in `[0,1]`
- strategy probability map must be valid and sum > 0

Compatibility behavior:

- if explicit strategy generator is set, it drives symlink behavior
- otherwise existing symlink relative probability semantics continue to work

## 3) Planner / Apply Enhancements

Extend planning model (`plan.go`):

- add `plannedEntryTypeHardlink`
- hardlink entry uses `path` + `linkTarget` (existing/planned regular-file path)
- add plan metadata:
  - `plannedHardlinkGroup{origin, paths[]}`
  - attached to `runPlan`

Extend plan state with deterministic candidate pools:

- `filePaths []string`
- `symlinkPaths []string`
- `hardlinkGroupByPath map[string]*plannedHardlinkGroup`

Generation decision order per file slot (bounded and deterministic):

1. symlink candidate path
2. hardlink candidate path
3. regular file fallback

Symlink strategy planning details:

- `absolute`: target is an existing planned file absolute path
- `relative`: target is `filepath.Rel(linkDir, targetFile)`
- `dangling`: target path intentionally unresolved
- `self-referential`: `link -> basename(link)`
- `chained`: link target points to existing planned symlink path
- `cycle`: generate bounded cycle (default length 2)

Apply changes:

- use `os.Link(target, linkPath)` for hardlinks
- keep run mode semantics (`append`, `strict`, `replace`) unchanged

## 4) Diff Hardlink Topology Parity

Refactor `diff` internals while keeping `Paths` API stable:

- collect regular node metadata as today
- additionally collect hardlink identity groups using `(dev, ino)` on non-Windows
- normalize groups as sorted relative path sets
- compare normalized group lists and return dedicated mismatch error

Platform split:

- `diff/fileid_unix.go`: identity collection via `unix.Lstat`
- `diff/fileid_windows.go`: return unsupported (no hardlink grouping)

Comparison rule:

- group topology parity is path-set based (not raw inode equality across trees)

## 5) Recursion / Cycle Safety

- planning remains bounded by explicit path depth and item counts
- cycle generation creates finite, explicit entries only
- no recursion through symlink targets in planner/apply
- test traversal safety with `filepath.Walk` to ensure no accidental infinite loop

## Implementation Steps (Execution Order)

1. Add `links.go` strategy types, stringers, validators, weighted generator helper.
2. Add option APIs and generator fields + validation logic.
3. Update planning/apply flow for hardlinks + symlink strategy planning.
4. Add hardlink-group metadata to plan.
5. Extend diff collection/comparison for hardlink topology.
6. Add/expand tests for generator/options/diff.
7. Update README with new link topology options.

## Test Plan

### Unit / Behavior Tests

`generator_test.go`

- hardlink creation parity (same file identity via `os.SameFile`)
- hardlink data mutation propagation
- strategy-specific tests:
  - absolute
  - relative
  - dangling
  - self-referential
  - chained
  - cycle
- traversal safety in cycle scenario

`options_test.go`

- success tests for new options
- validation tests for nil generators, invalid probabilities, invalid strategy maps

`diff/diff_test.go`

- hardlink parity pass
- hardlink mismatch fail
- existing symlink mismatch tests stay passing

### Full Validation Command

```bash
go test ./...
```

## Risk and Mitigations

1. **Behavior drift in existing symlink generation**
   - Mitigation: keep legacy path unless explicit strategy mode is configured.
2. **Platform-specific file identity handling**
   - Mitigation: build-tagged collectors and capability-aware tests/skips.
3. **Unbounded topology generation**
   - Mitigation: bounded cycle length and existing planner bounds.
4. **Diff flakiness from ordering**
   - Mitigation: canonical sort for groups and paths before compare.

## Rollback Plan

- Revert `links.go` + option additions + planner hardlink/symlink strategy branches.
- Revert diff hardlink-group comparison path.
- Keep existing deterministic planner baseline from issue #4 intact.

## Acceptance Criteria Mapping

- Common Linux link topologies intentionally generatable -> covered by strategy + hardlink tests.
- Link semantics verifiable by diff tests -> covered by hardlink mismatch parity tests and symlink target tests.
- No accidental infinite recursion in generator/apply flow -> bounded cycle generation + traversal safety test.

## PR Plan

This branch will be used for implementation commits and a linked PR with:

- title: `Add complete link topology support (symlink variants + hard links)`
- body references issue #3 and includes summary + validation output
- initially opened as draft until full implementation and tests are complete
