package randfiletree

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"
)

// coverageEpoch is the fixed time anchor used for every timestamp the
// coverage scenario produces. The seconds component is a recent UTC moment;
// the nanosecond component is set so atime/mtime exercise sub-second
// precision without colliding on a round value.
var coverageEpoch = time.Unix(1_700_000_000, 123_456_789).UTC()

// coverageModeClass enumerates the mode dimensions exercised by the coverage
// scenario. Files and directories share the same set, plus a "no-perm"
// representative to exercise the empty-permission case.
type coverageModeClass uint8

const (
	coverageModeClassRegular coverageModeClass = iota
	coverageModeClassExecutable
	coverageModeClassSetuid
	coverageModeClassSetgid
	coverageModeClassSticky
	coverageModeClassSetuidSetgid
)

// coverageModeClassAll lists the mode classes exercised by the coverage
// scenario. A zero-permission class is intentionally omitted: files with
// mode 0o000 cannot accept user.* xattrs or POSIX ACLs (write permission is
// required), and they cannot be read by non-root processes during the
// downstream diff parity checks. Restrictive-mode coverage is still
// exercised by the setuid/setgid/sticky classes.
var coverageModeClassAll = []coverageModeClass{
	coverageModeClassRegular,
	coverageModeClassExecutable,
	coverageModeClassSetuid,
	coverageModeClassSetgid,
	coverageModeClassSticky,
	coverageModeClassSetuidSetgid,
}

func (c coverageModeClass) String() string {
	switch c {
	case coverageModeClassRegular:
		return "mode-regular"
	case coverageModeClassExecutable:
		return "mode-exec"
	case coverageModeClassSetuid:
		return "mode-setuid"
	case coverageModeClassSetgid:
		return "mode-setgid"
	case coverageModeClassSticky:
		return "mode-sticky"
	case coverageModeClassSetuidSetgid:
		return "mode-setuid-setgid"
	default:
		return fmt.Sprintf("unknown(%d)", c)
	}
}

func coverageModeForFile(c coverageModeClass) uint32 {
	switch c {
	case coverageModeClassRegular:
		return 0o644
	case coverageModeClassExecutable:
		return 0o755
	case coverageModeClassSetuid:
		return 0o4750
	case coverageModeClassSetgid:
		return 0o2750
	case coverageModeClassSticky:
		return 0o1750
	case coverageModeClassSetuidSetgid:
		return 0o6750
	default:
		return 0o644
	}
}

func coverageModeForDir(c coverageModeClass) uint32 {
	switch c {
	case coverageModeClassRegular:
		return 0o755
	case coverageModeClassExecutable:
		return 0o755
	case coverageModeClassSetuid:
		return 0o4750
	case coverageModeClassSetgid:
		return 0o2750
	case coverageModeClassSticky:
		return 0o1750
	case coverageModeClassSetuidSetgid:
		return 0o6750
	default:
		return 0o755
	}
}

// coverageMetadataClass enumerates the metadata payload bundles exercised by
// the coverage scenario.
type coverageMetadataClass uint8

const (
	coverageMetadataClassNone coverageMetadataClass = iota
	coverageMetadataClassOwnership
	coverageMetadataClassTimestamps
	coverageMetadataClassFull
)

func (c coverageMetadataClass) String() string {
	switch c {
	case coverageMetadataClassNone:
		return "meta-none"
	case coverageMetadataClassOwnership:
		return "meta-ownership"
	case coverageMetadataClassTimestamps:
		return "meta-timestamps"
	case coverageMetadataClassFull:
		return "meta-full"
	default:
		return fmt.Sprintf("unknown(%d)", c)
	}
}

var coverageMetadataClassAll = []coverageMetadataClass{
	coverageMetadataClassNone,
	coverageMetadataClassOwnership,
	coverageMetadataClassTimestamps,
	coverageMetadataClassFull,
}

// coverageSymlinkStrategyAll mirrors the public SymlinkStrategy values in
// declaration order so pairwise iteration is deterministic.
var coverageSymlinkStrategyAll = []SymlinkStrategy{
	SymlinkStrategyAbsolute,
	SymlinkStrategyRelative,
	SymlinkStrategyDangling,
	SymlinkStrategySelfReferential,
	SymlinkStrategyChained,
	SymlinkStrategyCycle,
}

// coverageHardlinkGroupSizeAll lists the group sizes exercised: 2 mirrors
// the minimum-link case, 3 exercises a typical multi-link group, 4-cross-dir
// stresses cross-directory link topology.
var coverageHardlinkGroupSizeAll = []int{2, 3, 4}

// coverageSpecialFileTypeAll lists the special file types in declaration
// order.
var coverageSpecialFileTypeAll = []SpecialFileType{
	SpecialFileTypeFIFO,
	SpecialFileTypeSocket,
	SpecialFileTypeCharDevice,
	SpecialFileTypeBlockDevice,
}

// coverageXAttrVariant enumerates the user.* xattr value flavours exercised
// by the coverage scenario.
type coverageXAttrVariant uint8

const (
	coverageXAttrVariantUserShort coverageXAttrVariant = iota
	coverageXAttrVariantUserEmpty
	coverageXAttrVariantUserLarge
	coverageXAttrVariantUserBinary
	coverageXAttrVariantTrusted
	coverageXAttrVariantSecurity
)

func (v coverageXAttrVariant) String() string {
	switch v {
	case coverageXAttrVariantUserShort:
		return "xattr-user-short"
	case coverageXAttrVariantUserEmpty:
		return "xattr-user-empty"
	case coverageXAttrVariantUserLarge:
		return "xattr-user-large"
	case coverageXAttrVariantUserBinary:
		return "xattr-user-binary"
	case coverageXAttrVariantTrusted:
		return "xattr-trusted"
	case coverageXAttrVariantSecurity:
		return "xattr-security"
	default:
		return fmt.Sprintf("unknown(%d)", v)
	}
}

// coverageACLVariant enumerates the ACL flavours exercised by the coverage
// scenario.
type coverageACLVariant uint8

const (
	coverageACLVariantBase coverageACLVariant = iota
	coverageACLVariantNamedUserGroupMask
	coverageACLVariantDefaultOnDir
)

func (v coverageACLVariant) String() string {
	switch v {
	case coverageACLVariantBase:
		return "acl-base"
	case coverageACLVariantNamedUserGroupMask:
		return "acl-named-mask"
	case coverageACLVariantDefaultOnDir:
		return "acl-default-on-dir"
	default:
		return fmt.Sprintf("unknown(%d)", v)
	}
}

// coverageTimestampVariant enumerates the atime/mtime relationships exercised
// by the coverage scenario.
type coverageTimestampVariant uint8

const (
	coverageTimestampVariantEpoch coverageTimestampVariant = iota
	coverageTimestampVariantPast
	coverageTimestampVariantFuture
	coverageTimestampVariantAtimeBeforeMtime
	coverageTimestampVariantAtimeAfterMtime
)

func (v coverageTimestampVariant) String() string {
	switch v {
	case coverageTimestampVariantEpoch:
		return "ts-epoch"
	case coverageTimestampVariantPast:
		return "ts-past"
	case coverageTimestampVariantFuture:
		return "ts-future"
	case coverageTimestampVariantAtimeBeforeMtime:
		return "ts-atime-before-mtime"
	case coverageTimestampVariantAtimeAfterMtime:
		return "ts-atime-after-mtime"
	default:
		return fmt.Sprintf("unknown(%d)", v)
	}
}

func coverageTimestampPair(v coverageTimestampVariant) (atime, mtime time.Time) {
	switch v {
	case coverageTimestampVariantEpoch:
		return time.Unix(0, 0).UTC(), time.Unix(0, 0).UTC()
	case coverageTimestampVariantPast:
		return coverageEpoch.Add(-365 * 24 * time.Hour), coverageEpoch.Add(-365 * 24 * time.Hour)
	case coverageTimestampVariantFuture:
		return coverageEpoch.Add(365 * 24 * time.Hour), coverageEpoch.Add(365 * 24 * time.Hour)
	case coverageTimestampVariantAtimeBeforeMtime:
		return coverageEpoch, coverageEpoch.Add(time.Hour)
	case coverageTimestampVariantAtimeAfterMtime:
		return coverageEpoch.Add(time.Hour), coverageEpoch
	default:
		return coverageEpoch, coverageEpoch
	}
}

// coverageOwnershipTable is the fixed uid/gid table consulted when
// IncludePrivileged is true.
var coverageOwnershipTable = []struct{ uid, gid int }{
	{0, 0},
	{1000, 1000},
	{65534, 65534},
}

// coverageEffortMultiplier returns the per-cell repetition multiplier for the
// requested effort level. Numbers were chosen so xhigh stays under the
// default planEntryLimit (100k) for the pairwise dimension set the
// enumerator emits.
func coverageEffortMultiplier(effort DeterministicCoverageEffort) int {
	switch effort {
	case DeterministicCoverageEffortLow:
		return 1
	case DeterministicCoverageEffortMedium:
		return 3
	case DeterministicCoverageEffortHigh:
		return 10
	case DeterministicCoverageEffortXHigh:
		return 50
	default:
		return 1
	}
}

func validateDeterministicCoverageEffort(effort DeterministicCoverageEffort) error {
	switch effort {
	case DeterministicCoverageEffortLow,
		DeterministicCoverageEffortMedium,
		DeterministicCoverageEffortHigh,
		DeterministicCoverageEffortXHigh:
		return nil
	default:
		return fmt.Errorf("%w: %d", ErrDeterministicCoverageEffortInvalid, effort)
	}
}

// coveragePairwise returns a pairwise covering array for the given
// dimension cardinalities. Every (vᵢ, vⱼ) value pair across every pair of
// dimensions appears in at least one returned test case. The algorithm is a
// deterministic greedy IPOG variant: the seed pair is the first uncovered
// pair under lexicographic ordering and remaining slots are filled by the
// value that maximizes newly covered pairs, breaking ties by the lower value
// index. The output is byte-identical for fixed input.
func coveragePairwise(dimSizes []int) [][]int {
	n := len(dimSizes)
	if n == 0 {
		return nil
	}
	for _, size := range dimSizes {
		if size <= 0 {
			return nil
		}
	}
	if n == 1 {
		cases := make([][]int, dimSizes[0])
		for i := range cases {
			cases[i] = []int{i}
		}
		return cases
	}

	type pair struct{ d1, d2, v1, v2 int }
	uncovered := make([]pair, 0)
	for d1 := 0; d1 < n-1; d1++ {
		for d2 := d1 + 1; d2 < n; d2++ {
			for v1 := 0; v1 < dimSizes[d1]; v1++ {
				for v2 := 0; v2 < dimSizes[d2]; v2++ {
					uncovered = append(uncovered, pair{d1, d2, v1, v2})
				}
			}
		}
	}

	cases := make([][]int, 0)
	for len(uncovered) > 0 {
		seed := uncovered[0]
		tc := make([]int, n)
		for i := range tc {
			tc[i] = -1
		}
		tc[seed.d1] = seed.v1
		tc[seed.d2] = seed.v2

		for d := 0; d < n; d++ {
			if tc[d] != -1 {
				continue
			}

			bestV := 0
			bestCount := -1
			for v := 0; v < dimSizes[d]; v++ {
				count := 0
				for _, p := range uncovered {
					switch {
					case p.d1 == d && tc[p.d2] != -1 && p.v1 == v && p.v2 == tc[p.d2]:
						count++
					case p.d2 == d && tc[p.d1] != -1 && p.v2 == v && p.v1 == tc[p.d1]:
						count++
					}
				}
				if count > bestCount {
					bestCount = count
					bestV = v
				}
			}
			tc[d] = bestV
		}

		next := uncovered[:0]
		for _, p := range uncovered {
			if tc[p.d1] == p.v1 && tc[p.d2] == p.v2 {
				continue
			}
			next = append(next, p)
		}
		uncovered = next

		cases = append(cases, tc)
	}

	return cases
}

// coveragePlanBuilder accumulates planned entries and hardlink groups as the
// per-sub-tree enumerators emit them. It also tracks already-used paths so
// downstream consistency invariants (registered hardlink targets etc.) are
// honored.
type coveragePlanBuilder struct {
	basePath string

	entries   []plannedEntry
	hardlinks []plannedHardlinkGroup
	used      map[string]struct{}

	enabledCapabilities map[coverageCapabilityID]bool

	// nextCellIndex is incremented on every entry that earns a numeric
	// prefix; the cellID hash uses the same counter so paths and content
	// stay correlated.
	nextCellIndex int
}

func newCoveragePlanBuilder(basePath string, caps []capabilityReport) *coveragePlanBuilder {
	return &coveragePlanBuilder{
		basePath:            basePath,
		entries:             make([]plannedEntry, 0, 1024),
		used:                map[string]struct{}{},
		enabledCapabilities: coverageCapabilitySet(caps),
	}
}

func (b *coveragePlanBuilder) capabilityEnabled(id coverageCapabilityID) bool {
	return b.enabledCapabilities[id]
}

func (b *coveragePlanBuilder) registerPath(path string) error {
	if _, exists := b.used[path]; exists {
		return fmt.Errorf("coverage path collision: %s", path)
	}
	b.used[path] = struct{}{}
	return nil
}

func (b *coveragePlanBuilder) appendDir(path string, mode uint32, metadata metadataConfig) error {
	if err := b.registerPath(path); err != nil {
		return err
	}
	b.entries = append(b.entries, plannedEntry{
		typeID:   plannedEntryTypeDir,
		path:     path,
		mode:     mode,
		metadata: b.ensureDeterministicTimestamps(metadata),
	})
	return nil
}

func (b *coveragePlanBuilder) appendFile(entry plannedEntry) error {
	if entry.typeID == 0 {
		entry.typeID = plannedEntryTypeFile
	}
	if err := b.registerPath(entry.path); err != nil {
		return err
	}
	entry.metadata = b.ensureDeterministicTimestamps(entry.metadata)
	b.entries = append(b.entries, entry)
	return nil
}

// ensureDeterministicTimestamps stamps the entry's metadata with the fixed
// coverage epoch when the caller has not already set explicit timestamps and
// the timestamp capability is enabled.
func (b *coveragePlanBuilder) ensureDeterministicTimestamps(cfg metadataConfig) metadataConfig {
	if !b.enabledCapabilities[coverageCapabilityTimestampMetadata] {
		return cfg
	}
	if cfg.hasTimestamps {
		return cfg
	}
	cfg.hasTimestamps = true
	cfg.atime = coverageEpoch
	cfg.mtime = coverageEpoch
	return cfg
}

func (b *coveragePlanBuilder) appendSymlink(path, target string) error {
	if err := b.registerPath(path); err != nil {
		return err
	}
	b.entries = append(b.entries, plannedEntry{
		typeID:     plannedEntryTypeSymlink,
		path:       path,
		linkTarget: target,
		metadata:   b.deterministicLinkMetadata(),
	})
	return nil
}

// deterministicLinkMetadata returns the timestamp-only metadata applied to
// every symlink and to dir/special entries that don't otherwise carry
// metadata. This is required for two-path apply determinism: without it the
// filesystem records time.Now() at creation, which differs between runs.
func (b *coveragePlanBuilder) deterministicLinkMetadata() metadataConfig {
	if !b.enabledCapabilities[coverageCapabilityTimestampMetadata] {
		return metadataConfig{}
	}
	return metadataConfig{
		hasTimestamps: true,
		atime:         coverageEpoch,
		mtime:         coverageEpoch,
	}
}

func (b *coveragePlanBuilder) appendHardlink(path, target string) error {
	if err := b.registerPath(path); err != nil {
		return err
	}
	b.entries = append(b.entries, plannedEntry{
		typeID:     plannedEntryTypeHardlink,
		path:       path,
		linkTarget: target,
	})
	return nil
}

func (b *coveragePlanBuilder) appendSpecial(entry plannedEntry) error {
	if err := b.registerPath(entry.path); err != nil {
		return err
	}
	entry.typeID = plannedEntryTypeSpecial
	entry.metadata = b.ensureDeterministicTimestamps(entry.metadata)
	b.entries = append(b.entries, entry)
	return nil
}

func (b *coveragePlanBuilder) registerHardlinkGroup(origin string, mirrors []string) {
	paths := append([]string{origin}, mirrors...)
	sort.Strings(paths)
	b.hardlinks = append(b.hardlinks, plannedHardlinkGroup{
		origin: origin,
		paths:  paths,
	})
}

func (b *coveragePlanBuilder) buildRunPlan() runPlan {
	groups := make([]plannedHardlinkGroup, 0, len(b.hardlinks))
	for _, g := range b.hardlinks {
		paths := append([]string(nil), g.paths...)
		sort.Strings(paths)
		groups = append(groups, plannedHardlinkGroup{origin: g.origin, paths: paths})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].origin < groups[j].origin })

	return runPlan{
		entries:        b.entries,
		hardlinkGroups: groups,
	}
}

// enumerateCoveragePlan synthesizes the deterministic plan and the set of
// active capabilities for the given options + base path.
func enumerateCoveragePlan(basePath string, opts DeterministicCoverageOptions) (runPlan, []capabilityReport, error) {
	caps := resolveCoverageCapabilities(opts, basePath)
	b := newCoveragePlanBuilder(basePath, caps)
	multiplier := coverageEffortMultiplier(opts.Effort)

	if err := b.appendDir(basePath, 0o755, metadataConfig{}); err != nil {
		return runPlan{}, caps, err
	}

	subTrees := []struct {
		name string
		fn   func(*coveragePlanBuilder, int) error
	}{
		{"dirs", enumerateCoverageDirSubTree},
		{"files", enumerateCoverageFileSubTree},
		{"symlinks", enumerateCoverageSymlinkSubTree},
		{"hardlinks", enumerateCoverageHardlinkSubTree},
		{"special", enumerateCoverageSpecialSubTree},
		{"xattr-variants", enumerateCoverageXAttrSubTree},
		{"acl-variants", enumerateCoverageACLSubTree},
		{"timestamp-variants", enumerateCoverageTimestampSubTree},
	}

	for _, st := range subTrees {
		root := filepath.Join(basePath, st.name)
		if err := b.appendDir(root, 0o755, metadataConfig{}); err != nil {
			return runPlan{}, caps, err
		}
		if err := st.fn(b, multiplier); err != nil {
			return runPlan{}, caps, err
		}
	}

	return b.buildRunPlan(), caps, nil
}

// coverageCellWrapperDir returns a portable wrapper directory for a coverage
// test case. The wrapper isolates the entry-under-test from sibling cells and
// keeps byte-edge names confined to the leaf component.
func coverageCellWrapperDir(parent, kind string, index int) string {
	return filepath.Join(parent, fmt.Sprintf("cell-%s-%05d", kind, index))
}

func coverageMetadataFor(class coverageMetadataClass, caps map[coverageCapabilityID]bool, cellID uint64, ownerIndex int, tsVariant coverageTimestampVariant, xattrs map[string][]byte, aclEntries []string) metadataConfig {
	cfg := metadataConfig{}

	// Timestamps are always stamped when the capability is available. This
	// is required for two-path diff determinism: entries without an
	// explicit utimes call inherit time.Now() at creation, which differs
	// between runs and breaks reproducibility.
	if caps[coverageCapabilityTimestampMetadata] {
		atime, mtime := coverageTimestampPair(tsVariant)
		cfg.hasTimestamps = true
		cfg.atime = atime
		cfg.mtime = mtime
	}

	switch class {
	case coverageMetadataClassNone:
		return cfg

	case coverageMetadataClassOwnership:
		if caps[coverageCapabilityOwnershipMetadata] {
			uid, gid := coverageEffectiveOwnership(caps, ownerIndex)
			cfg.hasOwnership = true
			cfg.uid = uid
			cfg.gid = gid
		}

	case coverageMetadataClassTimestamps:
		// Already handled above; no additional payload.

	case coverageMetadataClassFull:
		if caps[coverageCapabilityOwnershipMetadata] {
			uid, gid := coverageEffectiveOwnership(caps, ownerIndex)
			cfg.hasOwnership = true
			cfg.uid = uid
			cfg.gid = gid
		}
		if caps[coverageCapabilityXAttrUser] && len(xattrs) > 0 {
			cfg.hasXAttrs = true
			cfg.xattrs = xattrs
		}
		if caps[coverageCapabilityACL] && len(aclEntries) > 0 {
			cfg.hasACL = true
			cfg.aclEntries = aclEntries
		}
	}

	_ = cellID
	return cfg
}

func coverageEffectiveOwnership(caps map[coverageCapabilityID]bool, ownerIndex int) (int, int) {
	if !caps[coverageCapabilityNonDefaultOwnership] {
		// Use the running process's identity so chown does not fail.
		uid := -1
		gid := -1
		if uid < 0 || gid < 0 {
			uid = coverageCurrentUID()
			gid = coverageCurrentGID()
		}
		return uid, gid
	}
	pair := coverageOwnershipTable[ownerIndex%len(coverageOwnershipTable)]
	return pair.uid, pair.gid
}

func coverageDefaultUserXAttrs(cellID uint64) map[string][]byte {
	value := []byte(fmt.Sprintf("v-%016x", cellID))
	return map[string][]byte{
		"user.coverage.value": value,
	}
}

func coverageDefaultACLEntries() []string {
	return []string{"u::rw-", "g::r--", "o::---"}
}

// enumerateCoverageDirSubTree fills the dirs/ sub-tree with directories
// exercising the (mode, name, metadata) pairwise matrix.
func enumerateCoverageDirSubTree(b *coveragePlanBuilder, multiplier int) error {
	root := filepath.Join(b.basePath, "dirs")
	caps := b.enabledCapabilities

	metaValues := coverageEnabledMetadataValues(caps)
	dimSizes := []int{
		len(coverageModeClassAll),
		len(coverageNameClassAll),
		len(metaValues),
	}
	cases := coveragePairwise(dimSizes)

	for idx, tc := range cases {
		modeClass := coverageModeClassAll[tc[0]]
		nameClass := coverageNameClassAll[tc[1]]
		metaClass := metaValues[tc[2]]

		for rep := 0; rep < multiplier; rep++ {
			cellID := coverageCellID("dirs", modeClass.String(), nameClass.String(), metaClass.String(), fmt.Sprintf("rep-%d", rep))
			wrapper := coverageCellWrapperDir(root, "dirs", b.nextCellIndex)
			b.nextCellIndex++
			if err := b.appendDir(wrapper, 0o755, metadataConfig{}); err != nil {
				return err
			}

			dirName := coverageDeterministicName(nameClass, coverageNameDefaultLen, cellID)
			dirPath := filepath.Join(wrapper, dirName)
			metadata := coverageMetadataFor(
				metaClass, caps, cellID, idx+rep,
				coverageTimestampVariantPast,
				coverageDefaultUserXAttrs(cellID),
				coverageDefaultACLEntries(),
			)
			if err := b.appendDir(dirPath, coverageModeForDir(modeClass), metadata); err != nil {
				return err
			}
		}
	}

	return nil
}

// enumerateCoverageFileSubTree fills the files/ sub-tree with regular files
// exercising the (mode, content, size, name, metadata) pairwise matrix.
func enumerateCoverageFileSubTree(b *coveragePlanBuilder, multiplier int) error {
	root := filepath.Join(b.basePath, "files")
	caps := b.enabledCapabilities

	metaValues := coverageEnabledMetadataValues(caps)
	dimSizes := []int{
		len(coverageModeClassAll),
		len(coverageContentClassAll),
		len(coverageSizeClassAll),
		len(coverageNameClassAll),
		len(metaValues),
	}
	cases := coveragePairwise(dimSizes)

	for idx, tc := range cases {
		modeClass := coverageModeClassAll[tc[0]]
		contentClass := coverageContentClassAll[tc[1]]
		sizeClass := coverageSizeClassAll[tc[2]]
		nameClass := coverageNameClassAll[tc[3]]
		metaClass := metaValues[tc[4]]

		for rep := 0; rep < multiplier; rep++ {
			cellID := coverageCellID(
				"files",
				modeClass.String(),
				contentClass.String(),
				sizeClass.String(),
				nameClass.String(),
				metaClass.String(),
				fmt.Sprintf("rep-%d", rep),
			)

			wrapper := coverageCellWrapperDir(root, "files", b.nextCellIndex)
			b.nextCellIndex++
			if err := b.appendDir(wrapper, 0o755, metadataConfig{}); err != nil {
				return err
			}

			fileName := coverageDeterministicName(nameClass, coverageNameDefaultLen, cellID)
			filePath := filepath.Join(wrapper, fileName)

			content, err := coverageContentFor(contentClass, sizeClass, cellID)
			if err != nil {
				return err
			}

			entry := plannedEntry{
				typeID: plannedEntryTypeFile,
				path:   filePath,
				mode:   coverageModeForFile(modeClass),
			}

			if content.pattern == 0 {
				entry.data = coveragePlainData(coverageSizeBytes(sizeClass), cellID)
			} else {
				entry.contentPattern = content
			}

			entry.metadata = coverageMetadataFor(
				metaClass, caps, cellID, idx+rep,
				coverageTimestampForCell(cellID),
				coverageDefaultUserXAttrs(cellID),
				coverageDefaultACLEntries(),
			)

			if err := b.appendFile(entry); err != nil {
				return err
			}
		}
	}

	return nil
}

func coverageTimestampForCell(cellID uint64) coverageTimestampVariant {
	return coverageTimestampVariant(cellID % uint64(len([]coverageTimestampVariant{
		coverageTimestampVariantEpoch,
		coverageTimestampVariantPast,
		coverageTimestampVariantFuture,
		coverageTimestampVariantAtimeBeforeMtime,
		coverageTimestampVariantAtimeAfterMtime,
	})))
}

// enumerateCoverageSymlinkSubTree fills the symlinks/ sub-tree with symlinks
// exercising the (strategy, name) pairwise matrix.
func enumerateCoverageSymlinkSubTree(b *coveragePlanBuilder, multiplier int) error {
	root := filepath.Join(b.basePath, "symlinks")

	if !b.capabilityEnabled(coverageCapabilitySymlinks) {
		return nil
	}

	// Anchor file every absolute/relative strategy resolves against.
	anchorFilePath := filepath.Join(root, "anchor-file.bin")
	if err := b.appendFile(plannedEntry{
		typeID: plannedEntryTypeFile,
		path:   anchorFilePath,
		mode:   0o644,
		data:   []byte("coverage-anchor-file"),
	}); err != nil {
		return err
	}

	// Anchor symlink every chained strategy resolves against.
	anchorSymlinkPath := filepath.Join(root, "anchor-symlink.lnk")
	if err := b.appendSymlink(anchorSymlinkPath, "anchor-file.bin"); err != nil {
		return err
	}

	dimSizes := []int{
		len(coverageSymlinkStrategyAll),
		len(coverageNameClassAll),
	}
	cases := coveragePairwise(dimSizes)

	for _, tc := range cases {
		strategy := coverageSymlinkStrategyAll[tc[0]]
		nameClass := coverageNameClassAll[tc[1]]

		for rep := 0; rep < multiplier; rep++ {
			cellID := coverageCellID("symlinks", strategy.String(), nameClass.String(), fmt.Sprintf("rep-%d", rep))

			wrapper := coverageCellWrapperDir(root, "symlinks", b.nextCellIndex)
			b.nextCellIndex++
			if err := b.appendDir(wrapper, 0o755, metadataConfig{}); err != nil {
				return err
			}

			if err := emitCoverageSymlinkCell(b, wrapper, anchorFilePath, anchorSymlinkPath, strategy, nameClass, cellID); err != nil {
				return err
			}
		}
	}

	return nil
}

func emitCoverageSymlinkCell(
	b *coveragePlanBuilder,
	wrapper string,
	anchorFilePath, anchorSymlinkPath string,
	strategy SymlinkStrategy,
	nameClass coverageNameClass,
	cellID uint64,
) error {
	linkName := coverageDeterministicName(nameClass, coverageNameDefaultLen, cellID)
	linkPath := filepath.Join(wrapper, linkName)

	switch strategy {
	case SymlinkStrategyAbsolute:
		return b.appendSymlink(linkPath, anchorFilePath)

	case SymlinkStrategyRelative:
		rel, err := filepath.Rel(wrapper, anchorFilePath)
		if err != nil {
			return fmt.Errorf("coverage relative symlink: %w", err)
		}
		return b.appendSymlink(linkPath, rel)

	case SymlinkStrategyDangling:
		danglingTargetName := coverageDeterministicName(coverageNameClassBasic, coverageNameDefaultLen, cellID^0xDEAD)
		return b.appendSymlink(linkPath, filepath.Join(wrapper, danglingTargetName))

	case SymlinkStrategySelfReferential:
		return b.appendSymlink(linkPath, filepath.Base(linkPath))

	case SymlinkStrategyChained:
		rel, err := filepath.Rel(wrapper, anchorSymlinkPath)
		if err != nil {
			return fmt.Errorf("coverage chained symlink: %w", err)
		}
		return b.appendSymlink(linkPath, rel)

	case SymlinkStrategyCycle:
		// Cycle emits two interlinked symlinks under the same wrapper.
		linkName2 := coverageDeterministicName(nameClass, coverageNameDefaultLen, cellID^0xBEEF)
		linkPath2 := filepath.Join(wrapper, linkName2)
		if err := b.appendSymlink(linkPath, filepath.Base(linkPath2)); err != nil {
			return err
		}
		if err := b.appendSymlink(linkPath2, filepath.Base(linkPath)); err != nil {
			return err
		}
		return nil

	default:
		return fmt.Errorf("unsupported coverage symlink strategy %d", strategy)
	}
}

// enumerateCoverageHardlinkSubTree fills the hardlinks/ sub-tree with
// hardlink groups exercising the (group-size, name) pairwise matrix.
func enumerateCoverageHardlinkSubTree(b *coveragePlanBuilder, multiplier int) error {
	root := filepath.Join(b.basePath, "hardlinks")

	if !b.capabilityEnabled(coverageCapabilityHardlinks) {
		return nil
	}

	dimSizes := []int{
		len(coverageHardlinkGroupSizeAll),
		len(coverageNameClassAll),
	}
	cases := coveragePairwise(dimSizes)

	for _, tc := range cases {
		groupSize := coverageHardlinkGroupSizeAll[tc[0]]
		nameClass := coverageNameClassAll[tc[1]]

		for rep := 0; rep < multiplier; rep++ {
			cellID := coverageCellID("hardlinks", fmt.Sprintf("size-%d", groupSize), nameClass.String(), fmt.Sprintf("rep-%d", rep))

			wrapper := coverageCellWrapperDir(root, "hardlinks", b.nextCellIndex)
			b.nextCellIndex++
			if err := b.appendDir(wrapper, 0o755, metadataConfig{}); err != nil {
				return err
			}

			originName := coverageDeterministicName(nameClass, coverageNameDefaultLen, cellID)
			originPath := filepath.Join(wrapper, originName)
			if err := b.appendFile(plannedEntry{
				typeID: plannedEntryTypeFile,
				path:   originPath,
				mode:   0o644,
				data:   coveragePlainData(64, cellID),
			}); err != nil {
				return err
			}

			mirrors := make([]string, 0, groupSize-1)
			for m := 1; m < groupSize; m++ {
				mirrorName := coverageDeterministicName(nameClass, coverageNameDefaultLen, cellID^uint64(m)*0x9E37)
				mirrorPath := filepath.Join(wrapper, mirrorName)
				if err := b.appendHardlink(mirrorPath, originPath); err != nil {
					return err
				}
				mirrors = append(mirrors, mirrorPath)
			}

			b.registerHardlinkGroup(originPath, mirrors)
		}
	}

	return nil
}

// enumerateCoverageSpecialSubTree fills the special/ sub-tree with FIFOs,
// sockets, and (optionally) device nodes exercising the (type, mode)
// pairwise matrix.
//
// Unix sockets are constrained by sun_path (108 bytes on Linux), so the
// special-file paths are flat and short: a wrapper per-cell directory is
// intentionally omitted, and each entry is named NNNNNN-<type-char>. The
// socket capability is gated on the worst-case path length up front via
// coverageCapabilitySpecialSocket; this branch trusts that gate.
func enumerateCoverageSpecialSubTree(b *coveragePlanBuilder, multiplier int) error {
	root := filepath.Join(b.basePath, "special")

	types := make([]SpecialFileType, 0, len(coverageSpecialFileTypeAll))
	for _, t := range coverageSpecialFileTypeAll {
		switch t {
		case SpecialFileTypeFIFO:
			if b.capabilityEnabled(coverageCapabilitySpecialFIFO) {
				types = append(types, t)
			}
		case SpecialFileTypeSocket:
			if b.capabilityEnabled(coverageCapabilitySpecialSocket) {
				types = append(types, t)
			}
		case SpecialFileTypeCharDevice:
			if b.capabilityEnabled(coverageCapabilitySpecialCharDevice) {
				types = append(types, t)
			}
		case SpecialFileTypeBlockDevice:
			if b.capabilityEnabled(coverageCapabilitySpecialBlockDevice) {
				types = append(types, t)
			}
		}
	}
	if len(types) == 0 {
		return nil
	}

	modeValues := []coverageModeClass{coverageModeClassRegular, coverageModeClassExecutable}
	dimSizes := []int{len(types), len(modeValues)}
	cases := coveragePairwise(dimSizes)

	specialIndex := 0
	for _, tc := range cases {
		specialType := types[tc[0]]
		modeClass := modeValues[tc[1]]

		for rep := 0; rep < multiplier; rep++ {
			name := fmt.Sprintf("%06d-%s", specialIndex, coverageSpecialTypeChar(specialType))
			specialIndex++

			path := filepath.Join(root, name)
			entry := plannedEntry{
				path:            path,
				mode:            coverageModeForFile(modeClass),
				specialFileType: specialType,
			}
			if specialType == SpecialFileTypeCharDevice || specialType == SpecialFileTypeBlockDevice {
				entry.specialDeviceMajor = 1
				entry.specialDeviceMinor = 3
			}
			if err := b.appendSpecial(entry); err != nil {
				return err
			}
		}
	}

	return nil
}

func coverageSpecialTypeChar(t SpecialFileType) string {
	switch t {
	case SpecialFileTypeFIFO:
		return "f"
	case SpecialFileTypeSocket:
		return "s"
	case SpecialFileTypeCharDevice:
		return "c"
	case SpecialFileTypeBlockDevice:
		return "b"
	default:
		return "x"
	}
}

// enumerateCoverageXAttrSubTree fills the xattr-variants/ sub-tree with
// regular files carrying one xattr per cell, exercising the (variant, name)
// pairwise matrix. Trusted/security namespaces are included only when
// IncludePrivileged opted them in via the capability set.
func enumerateCoverageXAttrSubTree(b *coveragePlanBuilder, multiplier int) error {
	root := filepath.Join(b.basePath, "xattr-variants")

	if !b.capabilityEnabled(coverageCapabilityXAttrUser) {
		return nil
	}

	variants := []coverageXAttrVariant{
		coverageXAttrVariantUserShort,
		coverageXAttrVariantUserEmpty,
		coverageXAttrVariantUserLarge,
		coverageXAttrVariantUserBinary,
	}
	if b.capabilityEnabled(coverageCapabilityXAttrTrusted) {
		variants = append(variants, coverageXAttrVariantTrusted)
	}
	if b.capabilityEnabled(coverageCapabilityXAttrSecurity) {
		variants = append(variants, coverageXAttrVariantSecurity)
	}

	dimSizes := []int{len(variants), len(coverageNameClassAll)}
	cases := coveragePairwise(dimSizes)

	for _, tc := range cases {
		variant := variants[tc[0]]
		nameClass := coverageNameClassAll[tc[1]]

		for rep := 0; rep < multiplier; rep++ {
			cellID := coverageCellID("xattr-variants", variant.String(), nameClass.String(), fmt.Sprintf("rep-%d", rep))

			wrapper := coverageCellWrapperDir(root, "xattr", b.nextCellIndex)
			b.nextCellIndex++
			if err := b.appendDir(wrapper, 0o755, metadataConfig{}); err != nil {
				return err
			}

			fileName := coverageDeterministicName(nameClass, coverageNameDefaultLen, cellID)
			filePath := filepath.Join(wrapper, fileName)

			xattrs := coverageXAttrValueFor(variant, cellID)
			entry := plannedEntry{
				typeID: plannedEntryTypeFile,
				path:   filePath,
				mode:   0o644,
				data:   coveragePlainData(16, cellID),
				metadata: metadataConfig{
					hasXAttrs: true,
					xattrs:    xattrs,
				},
			}
			if err := b.appendFile(entry); err != nil {
				return err
			}
		}
	}

	return nil
}

func coverageXAttrValueFor(variant coverageXAttrVariant, cellID uint64) map[string][]byte {
	switch variant {
	case coverageXAttrVariantUserShort:
		return map[string][]byte{"user.coverage.short": []byte(fmt.Sprintf("v-%016x", cellID))}
	case coverageXAttrVariantUserEmpty:
		return map[string][]byte{"user.coverage.empty": nil}
	case coverageXAttrVariantUserLarge:
		return map[string][]byte{"user.coverage.large": coverageBytesOfLength(4096, cellID)}
	case coverageXAttrVariantUserBinary:
		return map[string][]byte{"user.coverage.binary": coverageBinaryXAttrPayload(cellID)}
	case coverageXAttrVariantTrusted:
		return map[string][]byte{"trusted.coverage.value": []byte(fmt.Sprintf("v-%016x", cellID))}
	case coverageXAttrVariantSecurity:
		return map[string][]byte{"security.coverage.value": []byte(fmt.Sprintf("v-%016x", cellID))}
	default:
		return map[string][]byte{"user.coverage.unknown": []byte(fmt.Sprintf("v-%016x", cellID))}
	}
}

func coverageBytesOfLength(n int, cellID uint64) []byte {
	out := make([]byte, n)
	stream := uint64(coverageDeterministicSeed("xattr-large", cellID))
	for i := range out {
		stream = stream*6364136223846793005 + 1442695040888963407
		out[i] = byte(stream >> 56)
	}
	return out
}

func coverageBinaryXAttrPayload(cellID uint64) []byte {
	out := make([]byte, 256)
	for i := range out {
		out[i] = byte(i ^ int(cellID))
	}
	return out
}

// enumerateCoverageACLSubTree fills the acl-variants/ sub-tree with entries
// carrying ACL metadata, exercising the (variant, name) pairwise matrix.
// Default-ACL entries are emitted on directories; the other variants are
// emitted on regular files.
func enumerateCoverageACLSubTree(b *coveragePlanBuilder, multiplier int) error {
	root := filepath.Join(b.basePath, "acl-variants")

	if !b.capabilityEnabled(coverageCapabilityACL) {
		return nil
	}

	variants := []coverageACLVariant{
		coverageACLVariantBase,
		coverageACLVariantNamedUserGroupMask,
		coverageACLVariantDefaultOnDir,
	}

	dimSizes := []int{len(variants), len(coverageNameClassAll)}
	cases := coveragePairwise(dimSizes)

	for _, tc := range cases {
		variant := variants[tc[0]]
		nameClass := coverageNameClassAll[tc[1]]

		for rep := 0; rep < multiplier; rep++ {
			cellID := coverageCellID("acl-variants", variant.String(), nameClass.String(), fmt.Sprintf("rep-%d", rep))

			wrapper := coverageCellWrapperDir(root, "acl", b.nextCellIndex)
			b.nextCellIndex++
			if err := b.appendDir(wrapper, 0o755, metadataConfig{}); err != nil {
				return err
			}

			entries := coverageACLEntriesFor(variant)

			if variant == coverageACLVariantDefaultOnDir {
				dirName := coverageDeterministicName(nameClass, coverageNameDefaultLen, cellID)
				dirPath := filepath.Join(wrapper, dirName)
				if err := b.appendDir(dirPath, 0o755, metadataConfig{
					hasACL:     true,
					aclEntries: entries,
				}); err != nil {
					return err
				}
				continue
			}

			fileName := coverageDeterministicName(nameClass, coverageNameDefaultLen, cellID)
			filePath := filepath.Join(wrapper, fileName)
			if err := b.appendFile(plannedEntry{
				typeID: plannedEntryTypeFile,
				path:   filePath,
				mode:   0o644,
				data:   coveragePlainData(16, cellID),
				metadata: metadataConfig{
					hasACL:     true,
					aclEntries: entries,
				},
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func coverageACLEntriesFor(variant coverageACLVariant) []string {
	switch variant {
	case coverageACLVariantBase:
		return []string{"u::rw-", "g::r--", "o::---"}
	case coverageACLVariantNamedUserGroupMask:
		return []string{
			"u::rw-",
			"g::r--",
			"o::---",
			"u:1000:r--",
			"g:1000:r--",
			"m::r--",
		}
	case coverageACLVariantDefaultOnDir:
		return []string{
			"u::rwx",
			"g::r-x",
			"o::---",
			"default:u::rwx",
			"default:g::r-x",
			"default:o::---",
		}
	default:
		return []string{"u::rw-", "g::r--", "o::---"}
	}
}

// enumerateCoverageTimestampSubTree fills the timestamp-variants/ sub-tree
// with regular files exercising the (timestamp-variant, name) pairwise
// matrix.
func enumerateCoverageTimestampSubTree(b *coveragePlanBuilder, multiplier int) error {
	root := filepath.Join(b.basePath, "timestamp-variants")

	if !b.capabilityEnabled(coverageCapabilityTimestampMetadata) {
		return nil
	}

	variants := []coverageTimestampVariant{
		coverageTimestampVariantEpoch,
		coverageTimestampVariantPast,
		coverageTimestampVariantFuture,
		coverageTimestampVariantAtimeBeforeMtime,
		coverageTimestampVariantAtimeAfterMtime,
	}

	dimSizes := []int{len(variants), len(coverageNameClassAll)}
	cases := coveragePairwise(dimSizes)

	for _, tc := range cases {
		variant := variants[tc[0]]
		nameClass := coverageNameClassAll[tc[1]]

		for rep := 0; rep < multiplier; rep++ {
			cellID := coverageCellID("timestamp-variants", variant.String(), nameClass.String(), fmt.Sprintf("rep-%d", rep))

			wrapper := coverageCellWrapperDir(root, "ts", b.nextCellIndex)
			b.nextCellIndex++
			if err := b.appendDir(wrapper, 0o755, metadataConfig{}); err != nil {
				return err
			}

			fileName := coverageDeterministicName(nameClass, coverageNameDefaultLen, cellID)
			filePath := filepath.Join(wrapper, fileName)
			atime, mtime := coverageTimestampPair(variant)
			if err := b.appendFile(plannedEntry{
				typeID: plannedEntryTypeFile,
				path:   filePath,
				mode:   0o644,
				data:   coveragePlainData(16, cellID),
				metadata: metadataConfig{
					hasTimestamps: true,
					atime:         atime,
					mtime:         mtime,
				},
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// coverageEnabledMetadataValues returns the metadata-class values that the
// current capability set can actually satisfy. None is always available; the
// others gracefully drop out when their underlying capability is disabled.
func coverageEnabledMetadataValues(caps map[coverageCapabilityID]bool) []coverageMetadataClass {
	values := []coverageMetadataClass{coverageMetadataClassNone}
	if caps[coverageCapabilityOwnershipMetadata] {
		values = append(values, coverageMetadataClassOwnership)
	}
	if caps[coverageCapabilityTimestampMetadata] {
		values = append(values, coverageMetadataClassTimestamps)
	}
	if caps[coverageCapabilityOwnershipMetadata] &&
		caps[coverageCapabilityTimestampMetadata] &&
		caps[coverageCapabilityXAttrUser] &&
		caps[coverageCapabilityACL] {
		values = append(values, coverageMetadataClassFull)
	}
	return values
}
