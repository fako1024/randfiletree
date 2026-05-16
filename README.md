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
