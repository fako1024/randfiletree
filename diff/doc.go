// Package diff performs recursive parity checks between two filesystem
// trees produced by randfiletree (or any equivalent tree).
//
// Paths is the compatibility entrypoint and is equivalent to
// PathsWithOptions(a, b, DefaultOptions()). For configurable strictness,
// use PathsWithOptions with one of the provided profiles:
//
//   - DefaultOptions: backwards-compatible profile matching Paths.
//   - StrictLinuxOptions: enables ownership, nanosecond timestamp, and
//     extended-metadata parity checks suitable for Linux backup validation.
//
// Optional comparators are toggled via Options fields (CompareXAttrs,
// CompareACLs, CompareSparseness, CompareDeviceIDs); each depends on
// platform metadata collection and fails explicitly when unavailable.
// See README.md in the module root for cross-device, sparse-file, and
// xattr/ACL scenarios that exercise these comparators end-to-end.
package diff
