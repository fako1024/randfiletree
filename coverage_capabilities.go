package randfiletree

import (
	"fmt"
	"os"
	"runtime"
	"sort"
)

// SkippedDimension records one dimension that was excluded from a
// deterministic coverage plan, with a stable machine-readable reason.
type SkippedDimension struct {
	Name   string
	Reason string
}

// coverageCapabilityID enumerates capability gates the coverage scenario
// consults when deciding whether to include a dimension.
type coverageCapabilityID uint8

const (
	coverageCapabilityHardlinks coverageCapabilityID = iota
	coverageCapabilitySymlinks
	coverageCapabilitySpecialFIFO
	coverageCapabilitySpecialSocket
	coverageCapabilitySpecialCharDevice
	coverageCapabilitySpecialBlockDevice
	coverageCapabilityOwnershipMetadata
	coverageCapabilityTimestampMetadata
	coverageCapabilityXAttrUser
	coverageCapabilityXAttrTrusted
	coverageCapabilityXAttrSecurity
	coverageCapabilityACL
	coverageCapabilityNonDefaultOwnership
)

func (c coverageCapabilityID) String() string {
	switch c {
	case coverageCapabilityHardlinks:
		return "hardlinks"
	case coverageCapabilitySymlinks:
		return "symlinks"
	case coverageCapabilitySpecialFIFO:
		return "special-fifo"
	case coverageCapabilitySpecialSocket:
		return "special-socket"
	case coverageCapabilitySpecialCharDevice:
		return "special-char-device"
	case coverageCapabilitySpecialBlockDevice:
		return "special-block-device"
	case coverageCapabilityOwnershipMetadata:
		return "ownership-metadata"
	case coverageCapabilityTimestampMetadata:
		return "timestamp-metadata"
	case coverageCapabilityXAttrUser:
		return "xattr-user"
	case coverageCapabilityXAttrTrusted:
		return "xattr-trusted"
	case coverageCapabilityXAttrSecurity:
		return "xattr-security"
	case coverageCapabilityACL:
		return "acl"
	case coverageCapabilityNonDefaultOwnership:
		return "non-default-ownership"
	default:
		return fmt.Sprintf("unknown(%d)", c)
	}
}

type coverageCapabilityRequirement struct {
	linuxOnly  bool
	privileged bool
}

var coverageCapabilityRequirements = map[coverageCapabilityID]coverageCapabilityRequirement{
	coverageCapabilityHardlinks:           {linuxOnly: false, privileged: false},
	coverageCapabilitySymlinks:            {linuxOnly: false, privileged: false},
	coverageCapabilitySpecialFIFO:         {linuxOnly: true, privileged: false},
	coverageCapabilitySpecialSocket:       {linuxOnly: true, privileged: false},
	coverageCapabilitySpecialCharDevice:   {linuxOnly: true, privileged: true},
	coverageCapabilitySpecialBlockDevice:  {linuxOnly: true, privileged: true},
	coverageCapabilityOwnershipMetadata:   {linuxOnly: true, privileged: false},
	coverageCapabilityTimestampMetadata:   {linuxOnly: true, privileged: false},
	coverageCapabilityXAttrUser:           {linuxOnly: true, privileged: false},
	coverageCapabilityXAttrTrusted:        {linuxOnly: true, privileged: true},
	coverageCapabilityXAttrSecurity:       {linuxOnly: true, privileged: true},
	coverageCapabilityACL:                 {linuxOnly: true, privileged: false},
	coverageCapabilityNonDefaultOwnership: {linuxOnly: true, privileged: true},
}

// capabilityReport records the resolution outcome for one capability.
type capabilityReport struct {
	id      coverageCapabilityID
	enabled bool
	reason  string
}

// resolveCoverageCapabilities returns the enabled/skipped capability set for
// the configured options. The probeBasePath is used to probe filesystem-level
// support (xattr, ACL) when available; an empty path skips probing and falls
// back to platform-level decisions only.
func resolveCoverageCapabilities(opts DeterministicCoverageOptions, probeBasePath string) []capabilityReport {
	ids := make([]coverageCapabilityID, 0, len(coverageCapabilityRequirements))
	for id := range coverageCapabilityRequirements {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	reports := make([]capabilityReport, 0, len(ids))

	xattrProbeRun := false
	xattrProbeAvailable := false
	aclProbeRun := false
	aclProbeAvailable := false

	for _, id := range ids {
		report := capabilityReport{id: id}
		req := coverageCapabilityRequirements[id]

		switch {
		case req.linuxOnly && !opts.IncludeLinuxOnly:
			report.enabled = false
			report.reason = fmt.Sprintf("%s: requires linux (set IncludeLinuxOnly=true to enable)", id)

		case req.privileged && !opts.IncludePrivileged:
			report.enabled = false
			report.reason = fmt.Sprintf("%s: requires privileged capabilities (set IncludePrivileged=true to enable)", id)

		default:
			ok, reason := runCoverageCapabilityProbe(
				id, opts, probeBasePath,
				&xattrProbeRun, &xattrProbeAvailable,
				&aclProbeRun, &aclProbeAvailable,
			)
			report.enabled = ok
			report.reason = reason
		}

		reports = append(reports, report)
	}

	return reports
}

func runCoverageCapabilityProbe(
	id coverageCapabilityID,
	opts DeterministicCoverageOptions,
	probeBasePath string,
	xattrProbeRun *bool, xattrProbeAvailable *bool,
	aclProbeRun *bool, aclProbeAvailable *bool,
) (bool, string) {
	switch id {
	case coverageCapabilityXAttrUser, coverageCapabilityXAttrTrusted, coverageCapabilityXAttrSecurity:
		if probeBasePath == "" {
			return true, ""
		}
		if !*xattrProbeRun {
			*xattrProbeAvailable = coverageProbeXAttrSupport(probeBasePath)
			*xattrProbeRun = true
		}
		if !*xattrProbeAvailable {
			return false, fmt.Sprintf("%s: filesystem at probe path does not support xattrs", id)
		}

	case coverageCapabilityACL:
		if probeBasePath == "" {
			return true, ""
		}
		if !*aclProbeRun {
			*aclProbeAvailable = coverageProbeACLSupport(probeBasePath)
			*aclProbeRun = true
		}
		if !*aclProbeAvailable {
			return false, fmt.Sprintf("%s: filesystem at probe path does not support ACLs", id)
		}

	case coverageCapabilityNonDefaultOwnership:
		if runtime.GOOS == "linux" && !isProcessRoot() {
			return false, fmt.Sprintf("%s: requires root (effective uid %d)", id, os.Geteuid())
		}

	case coverageCapabilitySpecialSocket:
		// Unix socket paths must fit in sun_path (108 bytes on Linux,
		// including the trailing NUL). The coverage enumerator emits
		// socket paths shaped as "{basePath}/special/NNNNNN-s"; the
		// worst-case suffix is 18 bytes. Leave a small safety margin to
		// stay below the kernel limit when the temp prefix changes.
		const socketPathSafetyBudget = 90
		if probeBasePath != "" && len(probeBasePath) > socketPathSafetyBudget {
			return false, fmt.Sprintf("%s: base path length %d exceeds sun_path budget %d", id, len(probeBasePath), socketPathSafetyBudget)
		}
	}

	_ = opts
	return true, ""
}

func isProcessRoot() bool {
	return os.Geteuid() == 0
}

func coverageCapabilitySet(reports []capabilityReport) map[coverageCapabilityID]bool {
	set := make(map[coverageCapabilityID]bool, len(reports))
	for _, r := range reports {
		set[r.id] = r.enabled
	}
	return set
}

func coverageCapabilitySkipped(reports []capabilityReport) []SkippedDimension {
	skipped := make([]SkippedDimension, 0)
	for _, r := range reports {
		if r.enabled {
			continue
		}
		skipped = append(skipped, SkippedDimension{
			Name:   r.id.String(),
			Reason: r.reason,
		})
	}
	return skipped
}

func coverageCapabilityEnabledNames(reports []capabilityReport) []string {
	names := make([]string, 0)
	for _, r := range reports {
		if !r.enabled {
			continue
		}
		names = append(names, r.id.String())
	}
	return names
}
