package randfiletree

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"
)

// TimestampGenerator denotes a generator function for timestamps.
type TimestampGenerator func(r *rand.Rand) time.Time

// ACLGenerator denotes a generator function for ACL entries.
type ACLGenerator func(r *rand.Rand) ([]string, error)

// TimestampGeneratorConstant returns a fixed timestamp generator.
func TimestampGeneratorConstant(ts time.Time) TimestampGenerator {
	return func(r *rand.Rand) time.Time {
		return ts
	}
}

// ACLGeneratorFixed returns a fixed ACL entry set.
func ACLGeneratorFixed(entries []string) ACLGenerator {
	fixed := append([]string(nil), entries...)

	return func(r *rand.Rand) ([]string, error) {
		return append([]string(nil), fixed...), nil
	}
}

type xattrValueGeneratorConfig struct {
	name     string
	valueGen DataGenerator
}

type metadataConfig struct {
	hasOwnership bool
	uid          int
	gid          int

	hasTimestamps bool
	atime         time.Time
	mtime         time.Time

	hasXAttrs bool
	xattrs    map[string][]byte

	hasACL      bool
	aclEntries  []string
	aclUseTools bool
}

func (g *Generator) resolveMetadata(r *rand.Rand) (metadataConfig, error) {
	metadata := metadataConfig{}

	if g.ownershipUIDGen != nil && g.ownershipGIDGen != nil {
		metadata.hasOwnership = true
		metadata.uid = g.ownershipUIDGen(r)
		metadata.gid = g.ownershipGIDGen(r)
	}

	if g.atimeGen != nil && g.mtimeGen != nil {
		metadata.hasTimestamps = true
		metadata.atime = g.atimeGen(r)
		metadata.mtime = g.mtimeGen(r)
	}

	if len(g.xattrValueGens) > 0 {
		xattrs := make(map[string][]byte, len(g.xattrValueGens))
		for _, cfg := range g.xattrValueGens {
			value, err := cfg.valueGen(r)
			if err != nil {
				return metadataConfig{}, fmt.Errorf("failed to generate xattr value for `%s`: %w", cfg.name, err)
			}

			xattrs[cfg.name] = cloneBytes(value)
		}

		metadata.hasXAttrs = true
		metadata.xattrs = xattrs
	}

	if g.aclEntriesGen != nil {
		entries, err := g.aclEntriesGen(r)
		if err != nil {
			return metadataConfig{}, fmt.Errorf("failed to generate ACL entries: %w", err)
		}

		normalized, err := normalizeACLEntries(entries)
		if err != nil {
			return metadataConfig{}, err
		}

		metadata.hasACL = true
		metadata.aclEntries = normalized
		metadata.aclUseTools = g.aclCommandBackendEnabled
	}

	return metadata, nil
}

func (g *Generator) validateXAttrConfiguration() error {
	if len(g.xattrValueGens) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(g.xattrValueGens))
	for i, cfg := range g.xattrValueGens {
		if cfg.valueGen == nil {
			return fmt.Errorf("xattr value generator at index %d must not be nil", i)
		}

		name, err := validateXAttrName(cfg.name)
		if err != nil {
			return fmt.Errorf("xattr config at index %d: %w", i, err)
		}

		if _, exists := seen[name]; exists {
			return fmt.Errorf("duplicate xattr name configured: %s", name)
		}
		seen[name] = struct{}{}

		namespace := xattrNamespace(name)
		switch namespace {
		case "user":
		case "trusted":
			if !g.xattrAllowTrustedNamespace {
				return fmt.Errorf("%w: `%s`", ErrXAttrNamespaceNotAllowed, name)
			}
		case "security":
			if !g.xattrAllowSecurityNamespace {
				return fmt.Errorf("%w: `%s`", ErrXAttrNamespaceNotAllowed, name)
			}
		default:
			return fmt.Errorf("%w: `%s`", ErrXAttrNamespaceUnsupported, name)
		}
	}

	return nil
}

func validateXAttrName(name string) (string, error) {
	if name == "" {
		return "", ErrXAttrNameEmpty
	}

	if strings.Contains(name, "\x00") {
		return "", ErrXAttrNameContainsNUL
	}

	namespace := xattrNamespace(name)
	if namespace == "" {
		return "", ErrXAttrNameMissingNamespace
	}

	if len(name) <= len(namespace)+1 {
		return "", ErrXAttrNameMissingNamespace
	}

	return name, nil
}

func xattrNamespace(name string) string {
	idx := strings.IndexByte(name, '.')
	if idx <= 0 {
		return ""
	}

	return name[:idx]
}

func normalizeACLEntries(entries []string) ([]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(entries))
	for i, entry := range entries {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			return nil, fmt.Errorf("ACL entry at index %d: %w", i, ErrACLEntryEmpty)
		}
		if strings.Contains(trimmed, "\x00") {
			return nil, fmt.Errorf("ACL entry at index %d: %w", i, ErrACLEntryContainsNUL)
		}
		if strings.Contains(trimmed, ",") {
			return nil, fmt.Errorf("ACL entry at index %d: %w", i, ErrACLEntryContainsComma)
		}

		normalized = append(normalized, trimmed)
	}

	sort.Strings(normalized)

	result := normalized[:0]
	for i := range normalized {
		if i > 0 && normalized[i-1] == normalized[i] {
			continue
		}

		result = append(result, normalized[i])
	}

	return append([]string(nil), result...), nil
}
