package diff

import "fmt"

// TimestampPrecision defines the comparison granularity used for timestamps.
type TimestampPrecision uint8

const (
	// TimestampPrecisionSeconds compares mtime with second precision.
	TimestampPrecisionSeconds TimestampPrecision = iota
	// TimestampPrecisionNanoseconds compares mtime with nanosecond precision.
	TimestampPrecisionNanoseconds
)

// MetadataComparator is a pluggable comparator hook for future metadata checks.
// It receives the relative path plus corresponding left/right nodes.
type MetadataComparator func(path string, left, right Node) error

// Options defines metadata strictness for diff comparisons.
type Options struct {
	CompareContentHash      bool
	TimestampPrecision      TimestampPrecision
	CompareOwnership        bool
	CompareHardlinkTopology bool

	CompareXAttrs   bool
	XAttrComparator MetadataComparator

	CompareACLs   bool
	ACLComparator MetadataComparator
}

// DefaultOptions returns compatibility behavior matching Paths historically.
func DefaultOptions() Options {
	return Options{
		CompareContentHash:      true,
		TimestampPrecision:      TimestampPrecisionSeconds,
		CompareOwnership:        false,
		CompareHardlinkTopology: true,
		CompareXAttrs:           false,
		CompareACLs:             false,
	}
}

// StrictLinuxOptions returns a stricter profile suitable for Linux metadata parity.
func StrictLinuxOptions() Options {
	opts := DefaultOptions()
	opts.TimestampPrecision = TimestampPrecisionNanoseconds
	opts.CompareOwnership = true

	return opts
}

func (o Options) validate() error {
	if o.TimestampPrecision != TimestampPrecisionSeconds && o.TimestampPrecision != TimestampPrecisionNanoseconds {
		return fmt.Errorf("invalid timestamp precision: %d", o.TimestampPrecision)
	}

	if o.CompareXAttrs && o.XAttrComparator == nil {
		return fmt.Errorf("xattr comparison enabled but xattr comparator is nil")
	}

	if o.CompareACLs && o.ACLComparator == nil {
		return fmt.Errorf("ACL comparison enabled but ACL comparator is nil")
	}

	return nil
}
