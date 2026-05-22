package aclxattr

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	// XAttrAccess denotes the Linux access ACL xattr name.
	XAttrAccess = "system.posix_acl_access"
	// XAttrDefault denotes the Linux default ACL xattr name.
	XAttrDefault = "system.posix_acl_default"

	xattrVersion uint32 = 0x0002

	TagUserObj  uint16 = 0x0001
	TagUser     uint16 = 0x0002
	TagGroupObj uint16 = 0x0004
	TagGroup    uint16 = 0x0008
	TagMask     uint16 = 0x0010
	TagOther    uint16 = 0x0020

	permRead  uint16 = 0x0004
	permWrite uint16 = 0x0002
	permExec  uint16 = 0x0001
)

// UndefinedID denotes an ACL entry without qualifier.
var UndefinedID = ^uint32(0)

// Entry denotes a single binary ACL entry.
type Entry struct {
	Tag  uint16
	Perm uint16
	ID   uint32
}

// AccessEntriesFromMode derives base ACL entries from POSIX mode bits.
func AccessEntriesFromMode(mode uint32) []Entry {
	return []Entry{
		{Tag: TagUserObj, Perm: uint16((mode >> 6) & 0x7), ID: UndefinedID},
		{Tag: TagGroupObj, Perm: uint16((mode >> 3) & 0x7), ID: UndefinedID},
		{Tag: TagOther, Perm: uint16(mode & 0x7), ID: UndefinedID},
	}
}

// Marshal serializes ACL entries to Linux xattr binary format.
//
// The kernel exposes posix_acl_xattr_* records in host-native byte order,
// so the encoding uses binary.NativeEndian to round-trip correctly on
// both little- and big-endian Linux architectures.
func Marshal(entries []Entry) ([]byte, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	sorted := SortEntries(entries)
	if err := Validate(sorted); err != nil {
		return nil, err
	}

	data := make([]byte, 4+(8*len(sorted)))
	binary.NativeEndian.PutUint32(data[:4], xattrVersion)

	offset := 4
	for _, entry := range sorted {
		binary.NativeEndian.PutUint16(data[offset:offset+2], entry.Tag)
		binary.NativeEndian.PutUint16(data[offset+2:offset+4], entry.Perm)
		binary.NativeEndian.PutUint32(data[offset+4:offset+8], entry.ID)
		offset += 8
	}

	return data, nil
}

// Parse deserializes Linux ACL xattr binary format.
func Parse(data []byte) ([]Entry, error) {
	if len(data) == 0 {
		return nil, nil
	}

	if len(data) < 4 {
		return nil, fmt.Errorf("ACL xattr payload too short: %d", len(data))
	}

	if (len(data)-4)%8 != 0 {
		return nil, fmt.Errorf("ACL xattr payload malformed length: %d", len(data))
	}

	version := binary.NativeEndian.Uint32(data[:4])
	if version != xattrVersion {
		return nil, fmt.Errorf("ACL xattr version mismatch: got %#x want %#x", version, xattrVersion)
	}

	nEntries := (len(data) - 4) / 8
	entries := make([]Entry, 0, nEntries)
	offset := 4
	for i := 0; i < nEntries; i++ {
		entries = append(entries, Entry{
			Tag:  binary.NativeEndian.Uint16(data[offset : offset+2]),
			Perm: binary.NativeEndian.Uint16(data[offset+2 : offset+4]),
			ID:   binary.NativeEndian.Uint32(data[offset+4 : offset+8]),
		})
		offset += 8
	}

	entries = SortEntries(entries)
	if err := Validate(entries); err != nil {
		return nil, err
	}

	return entries, nil
}

// ParseTextEntries parses ACL text entries into access/default binary entry lists.
func ParseTextEntries(entries []string) (access []Entry, defaults []Entry, err error) {
	if len(entries) == 0 {
		return nil, nil, nil
	}

	accessSet := make(map[entryKey]Entry, len(entries))
	defaultSet := make(map[entryKey]Entry, len(entries))

	for _, rawEntry := range entries {
		isDefault, entry, parseErr := ParseTextEntry(rawEntry)
		if parseErr != nil {
			return nil, nil, parseErr
		}

		key := entryKey{tag: entry.Tag, id: entry.ID}
		if isDefault {
			defaultSet[key] = entry
			continue
		}

		accessSet[key] = entry
	}

	access = entriesFromMap(accessSet)
	defaults = entriesFromMap(defaultSet)

	if len(access) > 0 {
		if err := Validate(access); err != nil {
			return nil, nil, fmt.Errorf("access ACL invalid: %w", err)
		}
	}

	if len(defaults) > 0 {
		if err := Validate(defaults); err != nil {
			return nil, nil, fmt.Errorf("default ACL invalid: %w", err)
		}
	}

	return access, defaults, nil
}

// ParseTextEntry parses a single text ACL entry.
func ParseTextEntry(rawEntry string) (isDefault bool, entry Entry, err error) {
	entry.ID = UndefinedID

	trimmed := strings.TrimSpace(rawEntry)
	if trimmed == "" {
		return false, entry, fmt.Errorf("empty ACL entry")
	}

	if strings.HasPrefix(trimmed, "d:") {
		isDefault = true
		trimmed = strings.TrimSpace(trimmed[2:])
	}
	if strings.HasPrefix(trimmed, "default:") {
		isDefault = true
		trimmed = strings.TrimSpace(trimmed[len("default:"):])
	}

	parts := strings.Split(trimmed, ":")
	if len(parts) != 3 {
		return false, entry, fmt.Errorf("ACL entry must have 3 colon-separated fields, got %q", rawEntry)
	}

	role := strings.ToLower(strings.TrimSpace(parts[0]))
	qualifier := strings.TrimSpace(parts[1])
	perm, err := parsePerm(strings.TrimSpace(parts[2]))
	if err != nil {
		return false, entry, err
	}

	entry.Perm = perm

	switch role {
	case "u", "user":
		if qualifier == "" {
			entry.Tag = TagUserObj
			entry.ID = UndefinedID
			return isDefault, entry, nil
		}

		id, parseErr := parseID(qualifier)
		if parseErr != nil {
			return false, entry, fmt.Errorf("invalid user qualifier %q: %w", qualifier, parseErr)
		}

		entry.Tag = TagUser
		entry.ID = id

		return isDefault, entry, nil

	case "g", "group":
		if qualifier == "" {
			entry.Tag = TagGroupObj
			entry.ID = UndefinedID
			return isDefault, entry, nil
		}

		id, parseErr := parseID(qualifier)
		if parseErr != nil {
			return false, entry, fmt.Errorf("invalid group qualifier %q: %w", qualifier, parseErr)
		}

		entry.Tag = TagGroup
		entry.ID = id

		return isDefault, entry, nil

	case "m", "mask":
		if qualifier != "" {
			return false, entry, fmt.Errorf("mask ACL entry must not define qualifier")
		}

		entry.Tag = TagMask
		entry.ID = UndefinedID

		return isDefault, entry, nil

	case "o", "other":
		if qualifier != "" {
			return false, entry, fmt.Errorf("other ACL entry must not define qualifier")
		}

		entry.Tag = TagOther
		entry.ID = UndefinedID

		return isDefault, entry, nil

	default:
		return false, entry, fmt.Errorf("unsupported ACL entry role %q", role)
	}
}

// FormatTextEntries renders canonical textual ACL entries.
func FormatTextEntries(access []Entry, defaults []Entry) []string {
	result := make([]string, 0, len(access)+len(defaults))

	for _, entry := range SortEntries(access) {
		result = append(result, formatTextEntry(false, entry))
	}

	for _, entry := range SortEntries(defaults) {
		result = append(result, formatTextEntry(true, entry))
	}

	return result
}

// SortEntries sorts ACL entries into canonical deterministic order.
func SortEntries(entries []Entry) []Entry {
	if len(entries) == 0 {
		return nil
	}

	sorted := append([]Entry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool {
		ri := tagSortRank(sorted[i].Tag)
		rj := tagSortRank(sorted[j].Tag)
		if ri != rj {
			return ri < rj
		}

		if sorted[i].ID != sorted[j].ID {
			return sorted[i].ID < sorted[j].ID
		}

		return sorted[i].Perm < sorted[j].Perm
	})

	return sorted
}

// Validate validates ACL entries according to POSIX ACL invariants.
func Validate(entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}

	counts := make(map[uint16]int)
	seen := make(map[entryKey]struct{}, len(entries))
	hasNamed := false

	for _, entry := range entries {
		if entry.Perm > 0x7 {
			return fmt.Errorf("invalid ACL permission bits %#o", entry.Perm)
		}

		switch entry.Tag {
		case TagUserObj, TagGroupObj, TagMask, TagOther:
			if entry.ID != UndefinedID {
				return fmt.Errorf("ACL entry tag %#x requires undefined qualifier", entry.Tag)
			}
		case TagUser, TagGroup:
			if entry.ID == UndefinedID {
				return fmt.Errorf("ACL entry tag %#x requires numeric qualifier", entry.Tag)
			}
			hasNamed = true
		default:
			return fmt.Errorf("unsupported ACL entry tag %#x", entry.Tag)
		}

		key := entryKey{tag: entry.Tag, id: entry.ID}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate ACL entry for tag %#x id %d", entry.Tag, entry.ID)
		}
		seen[key] = struct{}{}

		counts[entry.Tag]++
	}

	if counts[TagUserObj] != 1 {
		return fmt.Errorf("ACL must contain exactly one user:: entry")
	}
	if counts[TagGroupObj] != 1 {
		return fmt.Errorf("ACL must contain exactly one group:: entry")
	}
	if counts[TagOther] != 1 {
		return fmt.Errorf("ACL must contain exactly one other:: entry")
	}
	if counts[TagMask] > 1 {
		return fmt.Errorf("ACL must contain at most one mask:: entry")
	}
	if hasNamed && counts[TagMask] != 1 {
		return fmt.Errorf("ACL with named user/group entries requires exactly one mask:: entry")
	}

	return nil
}

type entryKey struct {
	tag uint16
	id  uint32
}

func entriesFromMap(raw map[entryKey]Entry) []Entry {
	if len(raw) == 0 {
		return nil
	}

	entries := make([]Entry, 0, len(raw))
	for _, entry := range raw {
		entries = append(entries, entry)
	}

	return SortEntries(entries)
}

func parseID(raw string) (uint32, error) {
	id, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, err
	}

	return uint32(id), nil
}

func parsePerm(raw string) (uint16, error) {
	if len(raw) != 3 {
		return 0, fmt.Errorf("ACL permission must be 3 characters, got %q", raw)
	}

	var perm uint16
	for i := 0; i < len(raw); i++ {
		switch i {
		case 0:
			switch raw[i] {
			case 'r':
				perm |= permRead
			case '-':
			default:
				return 0, fmt.Errorf("invalid ACL read permission %q", raw)
			}
		case 1:
			switch raw[i] {
			case 'w':
				perm |= permWrite
			case '-':
			default:
				return 0, fmt.Errorf("invalid ACL write permission %q", raw)
			}
		case 2:
			switch raw[i] {
			case 'x':
				perm |= permExec
			case '-':
			default:
				return 0, fmt.Errorf("invalid ACL execute permission %q", raw)
			}
		}
	}

	return perm, nil
}

func formatPerm(perm uint16) string {
	runes := []rune{'-', '-', '-'}
	if perm&permRead != 0 {
		runes[0] = 'r'
	}
	if perm&permWrite != 0 {
		runes[1] = 'w'
	}
	if perm&permExec != 0 {
		runes[2] = 'x'
	}

	return string(runes)
}

func formatTextEntry(isDefault bool, entry Entry) string {
	perm := formatPerm(entry.Perm)

	var formatted string
	switch entry.Tag {
	case TagUserObj:
		formatted = fmt.Sprintf("user::%s", perm)
	case TagUser:
		formatted = fmt.Sprintf("user:%d:%s", entry.ID, perm)
	case TagGroupObj:
		formatted = fmt.Sprintf("group::%s", perm)
	case TagGroup:
		formatted = fmt.Sprintf("group:%d:%s", entry.ID, perm)
	case TagMask:
		formatted = fmt.Sprintf("mask::%s", perm)
	case TagOther:
		formatted = fmt.Sprintf("other::%s", perm)
	default:
		formatted = fmt.Sprintf("unknown:%d::%s", entry.Tag, perm)
	}

	if isDefault {
		return "default:" + formatted
	}

	return formatted
}

func tagSortRank(tag uint16) int {
	switch tag {
	case TagUserObj:
		return 0
	case TagUser:
		return 1
	case TagGroupObj:
		return 2
	case TagGroup:
		return 3
	case TagMask:
		return 4
	case TagOther:
		return 5
	default:
		return 99
	}
}
