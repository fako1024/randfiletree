package randfiletree

import (
	"errors"
	"fmt"
	gopath "path"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

const (
	operationSpecVersion = 1
	maxPOSIXFileMode     = 0o7777
)

var (
	ErrUnsupportedOperation        = errors.New("operation kind unsupported")
	ErrXAttrPlaceholderUnsupported = errors.New("xattr placeholder operation is not implemented")
)

// OperationKind denotes a single mutation operation kind.
type OperationKind uint8

const (
	OperationKindCreateFile OperationKind = iota + 1
	OperationKindCreateDir
	OperationKindCreateSymlink
	OperationKindCreateHardlink
	OperationKindDelete
	OperationKindRename
	OperationKindChmod
	OperationKindChown
	OperationKindTruncate
	OperationKindAppend
	OperationKindOverwriteRange
	OperationKindSetXAttr
	OperationKindRemoveXAttr
)

func (k OperationKind) String() string {
	switch k {
	case OperationKindCreateFile:
		return "create-file"
	case OperationKindCreateDir:
		return "create-dir"
	case OperationKindCreateSymlink:
		return "create-symlink"
	case OperationKindCreateHardlink:
		return "create-hardlink"
	case OperationKindDelete:
		return "delete"
	case OperationKindRename:
		return "rename"
	case OperationKindChmod:
		return "chmod"
	case OperationKindChown:
		return "chown"
	case OperationKindTruncate:
		return "truncate"
	case OperationKindAppend:
		return "append"
	case OperationKindOverwriteRange:
		return "overwrite-range"
	case OperationKindSetXAttr:
		return "set-xattr"
	case OperationKindRemoveXAttr:
		return "remove-xattr"
	default:
		return fmt.Sprintf("unknown(%d)", k)
	}
}

func validateOperationKind(kind OperationKind) error {
	switch kind {
	case OperationKindCreateFile,
		OperationKindCreateDir,
		OperationKindCreateSymlink,
		OperationKindCreateHardlink,
		OperationKindDelete,
		OperationKindRename,
		OperationKindChmod,
		OperationKindChown,
		OperationKindTruncate,
		OperationKindAppend,
		OperationKindOverwriteRange,
		OperationKindSetXAttr,
		OperationKindRemoveXAttr:
		return nil
	default:
		return fmt.Errorf("invalid operation kind %d", kind)
	}
}

// Operation denotes a single deterministic mutation step.
type Operation struct {
	Kind OperationKind `json:"kind"`

	Path        string `json:"path"`
	Destination string `json:"destination,omitempty"`
	SourcePath  string `json:"sourcePath,omitempty"`
	LinkTarget  string `json:"linkTarget,omitempty"`

	Mode uint32 `json:"mode,omitempty"`
	UID  int    `json:"uid,omitempty"`
	GID  int    `json:"gid,omitempty"`

	Size   int64 `json:"size,omitempty"`
	Offset int64 `json:"offset,omitempty"`

	Data []byte `json:"data,omitempty"`

	XAttrName  string `json:"xattrName,omitempty"`
	XAttrValue []byte `json:"xattrValue,omitempty"`
}

// OperationGenerationOptions defines deterministic operation-stream generation.
type OperationGenerationOptions struct {
	Seed int64

	Count       int
	MaxDataSize int

	AllowedKinds []OperationKind
}

// DefaultOperationGenerationOptions returns portable defaults.
func DefaultOperationGenerationOptions() OperationGenerationOptions {
	return OperationGenerationOptions{
		Seed:         defaultSeed,
		Count:        16,
		MaxDataSize:  64,
		AllowedKinds: defaultOperationKinds(),
	}
}

func defaultOperationKinds() []OperationKind {
	return []OperationKind{
		OperationKindCreateFile,
		OperationKindCreateDir,
		OperationKindDelete,
		OperationKindRename,
		OperationKindChmod,
		OperationKindTruncate,
		OperationKindAppend,
		OperationKindOverwriteRange,
	}
}

func (o OperationGenerationOptions) validate() error {
	if o.Count <= 0 {
		return fmt.Errorf("operation count must be > 0, got %d", o.Count)
	}
	if o.MaxDataSize <= 0 {
		return fmt.Errorf("max data size must be > 0, got %d", o.MaxDataSize)
	}
	if len(o.AllowedKinds) == 0 {
		return fmt.Errorf("allowed operation kinds must not be empty")
	}

	seen := make(map[OperationKind]struct{}, len(o.AllowedKinds))
	for i, kind := range o.AllowedKinds {
		if err := validateOperationKind(kind); err != nil {
			return fmt.Errorf("allowed kind at index %d: %w", i, err)
		}

		if _, exists := seen[kind]; exists {
			return fmt.Errorf("allowed operation kinds contain duplicate %s", kind)
		}
		seen[kind] = struct{}{}
	}

	return nil
}

type operationSpec struct {
	Version    int         `json:"version"`
	Operations []Operation `json:"operations"`
}

// ExportOperationSpec serializes operation streams for CI replay.
func ExportOperationSpec(ops []Operation) (string, error) {
	normalized := make([]Operation, len(ops))
	for i, op := range ops {
		next, err := normalizeOperation(op)
		if err != nil {
			return "", fmt.Errorf("invalid operation at index %d: %w", i, err)
		}

		normalized[i] = next
	}

	payload, err := jsoniter.Marshal(operationSpec{Version: operationSpecVersion, Operations: normalized})
	if err != nil {
		return "", fmt.Errorf("failed to serialize operation spec: %w", err)
	}

	return string(payload), nil
}

func normalizeOperation(op Operation) (Operation, error) {
	if err := validateOperationKind(op.Kind); err != nil {
		return Operation{}, err
	}

	next := op
	next.Data = cloneBytes(op.Data)
	next.XAttrValue = cloneBytes(op.XAttrValue)

	switch next.Kind {
	case OperationKindCreateFile:
		path, err := normalizeOperationPath(next.Path, false)
		if err != nil {
			return Operation{}, fmt.Errorf("create-file path: %w", err)
		}
		if err := validateMode(next.Mode); err != nil {
			return Operation{}, fmt.Errorf("create-file mode: %w", err)
		}
		next.Path = path

	case OperationKindCreateDir:
		path, err := normalizeOperationPath(next.Path, false)
		if err != nil {
			return Operation{}, fmt.Errorf("create-dir path: %w", err)
		}
		if err := validateMode(next.Mode); err != nil {
			return Operation{}, fmt.Errorf("create-dir mode: %w", err)
		}
		next.Path = path

	case OperationKindCreateSymlink:
		path, err := normalizeOperationPath(next.Path, false)
		if err != nil {
			return Operation{}, fmt.Errorf("create-symlink path: %w", err)
		}
		if next.LinkTarget == "" {
			return Operation{}, fmt.Errorf("create-symlink link target must not be empty")
		}
		if strings.Contains(next.LinkTarget, "\x00") {
			return Operation{}, fmt.Errorf("create-symlink link target must not contain NUL bytes")
		}
		next.Path = path

	case OperationKindCreateHardlink:
		path, err := normalizeOperationPath(next.Path, false)
		if err != nil {
			return Operation{}, fmt.Errorf("create-hardlink path: %w", err)
		}
		source, err := normalizeOperationPath(next.SourcePath, false)
		if err != nil {
			return Operation{}, fmt.Errorf("create-hardlink source path: %w", err)
		}
		if path == source {
			return Operation{}, fmt.Errorf("create-hardlink source and destination must differ")
		}
		next.Path = path
		next.SourcePath = source

	case OperationKindDelete:
		path, err := normalizeOperationPath(next.Path, false)
		if err != nil {
			return Operation{}, fmt.Errorf("delete path: %w", err)
		}
		next.Path = path

	case OperationKindRename:
		source, err := normalizeOperationPath(next.Path, false)
		if err != nil {
			return Operation{}, fmt.Errorf("rename source path: %w", err)
		}
		destination, err := normalizeOperationPath(next.Destination, false)
		if err != nil {
			return Operation{}, fmt.Errorf("rename destination path: %w", err)
		}
		if source == destination {
			return Operation{}, fmt.Errorf("rename source and destination must differ")
		}
		next.Path = source
		next.Destination = destination

	case OperationKindChmod:
		path, err := normalizeOperationPath(next.Path, true)
		if err != nil {
			return Operation{}, fmt.Errorf("chmod path: %w", err)
		}
		if err := validateMode(next.Mode); err != nil {
			return Operation{}, fmt.Errorf("chmod mode: %w", err)
		}
		next.Path = path

	case OperationKindChown:
		path, err := normalizeOperationPath(next.Path, true)
		if err != nil {
			return Operation{}, fmt.Errorf("chown path: %w", err)
		}
		if next.UID < 0 {
			return Operation{}, fmt.Errorf("chown uid must be >= 0, got %d", next.UID)
		}
		if next.GID < 0 {
			return Operation{}, fmt.Errorf("chown gid must be >= 0, got %d", next.GID)
		}
		next.Path = path

	case OperationKindTruncate:
		path, err := normalizeOperationPath(next.Path, false)
		if err != nil {
			return Operation{}, fmt.Errorf("truncate path: %w", err)
		}
		if next.Size < 0 {
			return Operation{}, fmt.Errorf("truncate size must be >= 0, got %d", next.Size)
		}
		next.Path = path

	case OperationKindAppend:
		path, err := normalizeOperationPath(next.Path, false)
		if err != nil {
			return Operation{}, fmt.Errorf("append path: %w", err)
		}
		if len(next.Data) == 0 {
			return Operation{}, fmt.Errorf("append data must not be empty")
		}
		next.Path = path

	case OperationKindOverwriteRange:
		path, err := normalizeOperationPath(next.Path, false)
		if err != nil {
			return Operation{}, fmt.Errorf("overwrite-range path: %w", err)
		}
		if next.Offset < 0 {
			return Operation{}, fmt.Errorf("overwrite-range offset must be >= 0, got %d", next.Offset)
		}
		if len(next.Data) == 0 {
			return Operation{}, fmt.Errorf("overwrite-range data must not be empty")
		}
		next.Path = path

	case OperationKindSetXAttr:
		path, err := normalizeOperationPath(next.Path, true)
		if err != nil {
			return Operation{}, fmt.Errorf("set-xattr path: %w", err)
		}
		if next.XAttrName == "" {
			return Operation{}, fmt.Errorf("set-xattr name must not be empty")
		}
		if strings.Contains(next.XAttrName, "\x00") {
			return Operation{}, fmt.Errorf("set-xattr name must not contain NUL bytes")
		}
		next.Path = path

	case OperationKindRemoveXAttr:
		path, err := normalizeOperationPath(next.Path, true)
		if err != nil {
			return Operation{}, fmt.Errorf("remove-xattr path: %w", err)
		}
		if next.XAttrName == "" {
			return Operation{}, fmt.Errorf("remove-xattr name must not be empty")
		}
		if strings.Contains(next.XAttrName, "\x00") {
			return Operation{}, fmt.Errorf("remove-xattr name must not contain NUL bytes")
		}
		next.Path = path
	}

	return next, nil
}

func normalizeOperationPath(raw string, allowRoot bool) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	if strings.Contains(raw, "\x00") {
		return "", fmt.Errorf("path must not contain NUL bytes")
	}

	cleaned := gopath.Clean(raw)
	if !strings.HasPrefix(cleaned, "/") {
		return "", fmt.Errorf("path must be rooted (start with '/'), got %q", raw)
	}
	if !allowRoot && cleaned == "/" {
		return "", fmt.Errorf("path must not be root")
	}

	return cleaned, nil
}

func validateMode(mode uint32) error {
	if mode > maxPOSIXFileMode {
		return fmt.Errorf("mode must be <= %#o, got %#o", maxPOSIXFileMode, mode)
	}

	return nil
}

func cloneBytes(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}

	copyData := make([]byte, len(data))
	copy(copyData, data)

	return copyData
}
