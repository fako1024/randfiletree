package randfiletree

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	gopath "path"
	"path/filepath"
	"sort"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

const (
	maxMutationPathCollisionRetries = 128
)

var (
	ErrMutationPathCollisionExhausted = errors.New("mutation path collision retries exhausted")
	ErrBasePathEmpty                  = errors.New("base path must not be empty")
	ErrBasePathSymlink                = errors.New("base path must not be a symlink")
	ErrOperationSpecEmpty             = errors.New("operation spec must not be empty")
	ErrOperationPathEscapesBase       = errors.New("operation path escapes base path")
	ErrOperationPathSymlinkParent     = errors.New("operation path uses symlink parent")
	errOperationPreconditions         = errors.New("operation preconditions not met")
)

// OperationApplyError denotes a failed operation execution with replay context.
//
// Spec holds the JSON-encoded replay snapshot of the original operation stream
// for use with ParseOperationSpec / ApplyOperationsWithOptions. It is populated
// lazily on the first call to ReplaySpec or Error so that a failing run does
// not pay the full O(N) serialization cost when the caller never inspects it.
type OperationApplyError struct {
	Index     int
	Operation Operation
	Spec      string
	Err       error

	specOps []Operation
}

// ReplaySpec returns the JSON replay snapshot, computing it on first call.
func (e *OperationApplyError) ReplaySpec() string {
	if e == nil {
		return ""
	}
	if e.Spec != "" || len(e.specOps) == 0 {
		return e.Spec
	}

	spec, specErr := ExportOperationSpec(e.specOps)
	if specErr != nil {
		fallback, marshalErr := jsoniter.Marshal(map[string]string{"exportError": specErr.Error()})
		if marshalErr != nil {
			spec = `{"exportError":"<unencodable>"}`
		} else {
			spec = string(fallback)
		}
	}
	e.Spec = spec
	e.specOps = nil

	return e.Spec
}

func (e *OperationApplyError) Error() string {
	if e == nil {
		return "<nil>"
	}

	spec := e.ReplaySpec()
	if spec == "" {
		return fmt.Sprintf("operation[%d] %s %s failed: %v", e.Index, e.Operation.Kind, operationPathLabel(e.Operation), e.Err)
	}

	return fmt.Sprintf(
		"operation[%d] %s %s failed: %v; replay-spec=%s",
		e.Index,
		e.Operation.Kind,
		operationPathLabel(e.Operation),
		e.Err,
		spec,
	)
}

func (e *OperationApplyError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

// ParseOperationSpec parses exported operation specs.
func ParseOperationSpec(spec string) ([]Operation, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, ErrOperationSpecEmpty
	}

	var parsed operationSpec
	if err := jsoniter.Unmarshal([]byte(spec), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse operation spec: %w", err)
	}

	if parsed.Version != operationSpecVersion {
		return nil, fmt.Errorf("operation spec version mismatch: got %d want %d", parsed.Version, operationSpecVersion)
	}

	ops := make([]Operation, len(parsed.Operations))
	for i, op := range parsed.Operations {
		normalized, err := normalizeOperation(op)
		if err != nil {
			return nil, fmt.Errorf("invalid operation at index %d: %w", i, err)
		}

		ops[i] = normalized
	}

	return ops, nil
}

// GenerateOperations creates a deterministic operation stream from a baseline path.
func GenerateOperations(basePath string, opts OperationGenerationOptions) ([]Operation, error) {
	if strings.TrimSpace(basePath) == "" {
		return nil, ErrBasePathEmpty
	}
	if err := opts.validate(); err != nil {
		return nil, err
	}

	state, err := scanMutationState(basePath)
	if err != nil {
		return nil, err
	}

	/* #nosec G404 */
	r := rand.New(rand.NewSource(opts.Seed))
	ops := make([]Operation, 0, opts.Count)

	for i := 0; i < opts.Count; i++ {
		op, err := state.planNextOperation(r, opts.AllowedKinds, opts.MaxDataSize)
		if err != nil {
			return nil, fmt.Errorf("failed to plan operation at index %d: %w", i, err)
		}

		normalized, err := normalizeOperation(op)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize operation at index %d: %w", i, err)
		}

		if err := state.applyVirtualOperation(normalized); err != nil {
			return nil, fmt.Errorf("failed to apply planned operation at index %d: %w", i, err)
		}

		ops = append(ops, normalized)
	}

	return ops, nil
}

// OperationApplyOptions defines optional deterministic execution behavior for ApplyOperations.
type OperationApplyOptions struct {
	// StartIndex resumes execution at ops[StartIndex].
	//
	// This allows deterministic continuation/retry tests after partial failures.
	StartIndex int

	// FaultProfile injects deterministic failures at matching execution points.
	FaultProfile FaultProfile
}

func (o OperationApplyOptions) validate(operationCount int) error {
	if o.StartIndex < 0 {
		return ErrOperationStartIndexNegative
	}
	if o.StartIndex > operationCount {
		return ErrOperationStartIndexOutOfRange
	}

	if err := o.FaultProfile.validate(); err != nil {
		return fmt.Errorf("invalid operation apply fault profile: %w", err)
	}

	return nil
}

// ApplyOperations executes operations in strict order and fails fast.
//
// Not safe for concurrent use against the same basePath: operations mutate the
// shared filesystem subtree and the fault injector backing OperationApplyOptions
// keeps per-rule trigger state without locking.
func ApplyOperations(basePath string, ops []Operation) error {
	return ApplyOperationsWithOptions(basePath, ops, OperationApplyOptions{})
}

// ApplyOperationsWithOptions executes operations in strict order and fails fast.
//
// Carries the same single-goroutine contract as ApplyOperations.
func ApplyOperationsWithOptions(basePath string, ops []Operation, opts OperationApplyOptions) error {
	if strings.TrimSpace(basePath) == "" {
		return ErrBasePathEmpty
	}
	if err := opts.validate(len(ops)); err != nil {
		return err
	}
	if opts.StartIndex == len(ops) {
		return nil
	}

	baseInfo, err := os.Lstat(basePath)
	if err != nil {
		return fmt.Errorf("failed to inspect base path `%s`: %w", basePath, err)
	}
	if baseInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("base path `%s`: %w", basePath, ErrBasePathSymlink)
	}
	if !baseInfo.IsDir() {
		return fmt.Errorf("base path `%s` is not a directory", basePath)
	}

	execCtx, err := newExecutionContext(opts.FaultProfile)
	if err != nil {
		return err
	}

	for i := opts.StartIndex; i < len(ops); i++ {
		op := ops[i]

		normalized, normalizeErr := normalizeOperation(op)
		if normalizeErr != nil {
			return newOperationApplyError(i, op, ops, fmt.Errorf("invalid operation: %w", normalizeErr))
		}

		if err := execCtx.before(FaultScopeMutation, i, normalized.Kind.String(), normalized.Path); err != nil {
			return newOperationApplyError(i, normalized, ops, err)
		}

		if err := applyOperation(basePath, normalized); err != nil {
			return newOperationApplyError(i, normalized, ops, err)
		}
	}

	return nil
}

type mutationNodeType uint8

const (
	mutationNodeTypeFile mutationNodeType = iota + 1
	mutationNodeTypeDir
	mutationNodeTypeSymlink
)

type mutationNode struct {
	typeID mutationNodeType
	size   int64
	xattrs map[string]struct{}
}

type mutationState struct {
	nodes map[string]mutationNode

	nameSeq int

	// Lazily-rebuilt sorted indexes over nodes. Invalidated by
	// invalidatePathIndexes() whenever the node map mutates so each
	// planOperation attempt can reuse the same scan across all kinds.
	indexAll     []string
	indexFiles   []string
	indexDirs    []string
	indexXAttrs  []string
	indexesValid bool
}

func (state *mutationState) invalidatePathIndexes() {
	state.indexesValid = false
	state.indexAll = nil
	state.indexFiles = nil
	state.indexDirs = nil
	state.indexXAttrs = nil
}

func (state *mutationState) rebuildPathIndexes() {
	if state.indexesValid {
		return
	}

	all := make([]string, 0, len(state.nodes))
	files := make([]string, 0, len(state.nodes))
	dirs := make([]string, 0, len(state.nodes))
	xattrs := make([]string, 0, len(state.nodes))

	for path, node := range state.nodes {
		all = append(all, path)

		switch node.typeID {
		case mutationNodeTypeFile:
			files = append(files, path)
		case mutationNodeTypeDir:
			dirs = append(dirs, path)
		}

		if len(node.xattrs) > 0 {
			xattrs = append(xattrs, path)
		}
	}

	sort.Strings(all)
	sort.Strings(files)
	sort.Strings(dirs)
	sort.Strings(xattrs)

	state.indexAll = all
	state.indexFiles = files
	state.indexDirs = dirs
	state.indexXAttrs = xattrs
	state.indexesValid = true
}

func scanMutationState(basePath string) (*mutationState, error) {
	baseInfo, err := os.Stat(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect baseline path `%s`: %w", basePath, err)
	}
	if !baseInfo.IsDir() {
		return nil, fmt.Errorf("baseline path `%s` is not a directory", basePath)
	}

	state := &mutationState{
		nodes: map[string]mutationNode{
			"/": {
				typeID: mutationNodeTypeDir,
			},
		},
		nameSeq: 1,
	}

	err = filepath.Walk(basePath, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("failed to access path `%s`: %w", path, walkErr)
		}

		nodePath, err := canonicalMutationPath(basePath, path)
		if err != nil {
			return err
		}
		if nodePath == "/" {
			return nil
		}

		node := mutationNode{}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			node.typeID = mutationNodeTypeSymlink
		case info.IsDir():
			node.typeID = mutationNodeTypeDir
		case info.Mode().IsRegular():
			node.typeID = mutationNodeTypeFile
			node.size = info.Size()
		default:
			return nil
		}

		xattrs, err := scanPathXAttrSet(path)
		if err != nil {
			return err
		}
		node.xattrs = xattrs

		state.nodes[nodePath] = node

		return nil
	})
	if err != nil {
		return nil, err
	}

	return state, nil
}

func canonicalMutationPath(basePath, path string) (string, error) {
	relPath, err := filepath.Rel(basePath, path)
	if err != nil {
		return "", fmt.Errorf("failed to determine relative path for `%s`: %w", path, err)
	}

	if relPath == "." {
		return "/", nil
	}

	return "/" + filepath.ToSlash(relPath), nil
}

func (state *mutationState) planNextOperation(
	r *rand.Rand,
	allowedKinds []OperationKind,
	maxDataSize int,
) (Operation, error) {
	start := r.Intn(len(allowedKinds))
	var lastPreconditionErr error

	for i := 0; i < len(allowedKinds); i++ {
		kind := allowedKinds[(start+i)%len(allowedKinds)]
		op, err := state.planOperation(kind, r, maxDataSize)
		if err == nil {
			return op, nil
		}

		if errors.Is(err, errOperationPreconditions) {
			lastPreconditionErr = fmt.Errorf("%s: %w", kind, err)
			continue
		}

		return Operation{}, fmt.Errorf("%s: %w", kind, err)
	}

	if lastPreconditionErr != nil {
		return Operation{}, lastPreconditionErr
	}

	return Operation{}, errOperationPreconditions
}

func (state *mutationState) planOperation(kind OperationKind, r *rand.Rand, maxDataSize int) (Operation, error) {
	switch kind {
	case OperationKindCreateFile:
		path, err := state.planUniquePath(r)
		if err != nil {
			return Operation{}, err
		}

		return Operation{
			Kind: kind,
			Path: path,
			Mode: randomFileMode(r),
			Data: randomBytes(r, randomDataLength(r, maxDataSize)),
		}, nil

	case OperationKindCreateDir:
		path, err := state.planUniquePath(r)
		if err != nil {
			return Operation{}, err
		}

		return Operation{
			Kind: kind,
			Path: path,
			Mode: randomDirMode(r),
		}, nil

	case OperationKindCreateSymlink:
		targets := state.sortedPaths(false)
		if len(targets) == 0 {
			return Operation{}, errOperationPreconditions
		}

		path, err := state.planUniquePath(r)
		if err != nil {
			return Operation{}, err
		}

		target := targets[r.Intn(len(targets))]
		relTargetFS, err := filepath.Rel(filepath.FromSlash(gopath.Dir(path)), filepath.FromSlash(target))
		if err != nil {
			return Operation{}, fmt.Errorf("failed to derive relative symlink target: %w", err)
		}
		relTarget := filepath.ToSlash(relTargetFS)

		return Operation{
			Kind:       kind,
			Path:       path,
			LinkTarget: relTarget,
		}, nil

	case OperationKindCreateHardlink:
		files := state.sortedFilePaths()
		if len(files) == 0 {
			return Operation{}, errOperationPreconditions
		}

		path, err := state.planUniquePath(r)
		if err != nil {
			return Operation{}, err
		}

		return Operation{
			Kind:       kind,
			Path:       path,
			SourcePath: files[r.Intn(len(files))],
		}, nil

	case OperationKindDelete:
		paths := state.sortedPaths(false)
		if len(paths) == 0 {
			return Operation{}, errOperationPreconditions
		}

		return Operation{
			Kind: kind,
			Path: paths[r.Intn(len(paths))],
		}, nil

	case OperationKindRename:
		sources := state.sortedPaths(false)
		if len(sources) == 0 {
			return Operation{}, errOperationPreconditions
		}

		source := sources[r.Intn(len(sources))]
		sourceNode := state.nodes[source]

		for attempt := 1; attempt <= maxMutationPathCollisionRetries; attempt++ {
			destination, err := state.planUniquePath(r)
			if err != nil {
				return Operation{}, err
			}

			if sourceNode.typeID == mutationNodeTypeDir && isChildMutationPath(destination, source) {
				continue
			}

			return Operation{
				Kind:        kind,
				Path:        source,
				Destination: destination,
			}, nil
		}

		return Operation{}, fmt.Errorf("rename destination: %w", ErrMutationPathCollisionExhausted)

	case OperationKindChmod:
		paths := state.sortedPaths(false)
		if len(paths) == 0 {
			return Operation{}, errOperationPreconditions
		}

		target := paths[r.Intn(len(paths))]
		mode := randomFileMode(r)
		if state.nodes[target].typeID == mutationNodeTypeDir {
			mode = randomDirMode(r)
		}

		return Operation{
			Kind: kind,
			Path: target,
			Mode: mode,
		}, nil

	case OperationKindChown:
		paths := state.sortedPaths(false)
		if len(paths) == 0 {
			return Operation{}, errOperationPreconditions
		}
		uid := os.Getuid()
		gid := os.Getgid()
		if uid < 0 || gid < 0 {
			return Operation{}, errOperationPreconditions
		}

		return Operation{
			Kind: kind,
			Path: paths[r.Intn(len(paths))],
			UID:  uid,
			GID:  gid,
		}, nil

	case OperationKindTruncate:
		files := state.sortedFilePaths()
		if len(files) == 0 {
			return Operation{}, errOperationPreconditions
		}

		filePath := files[r.Intn(len(files))]
		currentSize := state.nodes[filePath].size
		maxSize := int64(maxDataSize)
		if currentSize > maxSize {
			maxSize = currentSize
		}

		return Operation{
			Kind: kind,
			Path: filePath,
			Size: int64(r.Intn(int(maxSize) + 1)),
		}, nil

	case OperationKindAppend:
		files := state.sortedFilePaths()
		if len(files) == 0 {
			return Operation{}, errOperationPreconditions
		}

		return Operation{
			Kind: kind,
			Path: files[r.Intn(len(files))],
			Data: randomBytes(r, randomDataLength(r, maxDataSize)),
		}, nil

	case OperationKindOverwriteRange:
		files := state.sortedFilePaths()
		if len(files) == 0 {
			return Operation{}, errOperationPreconditions
		}

		filePath := files[r.Intn(len(files))]
		fileSize := state.nodes[filePath].size
		if fileSize <= 0 {
			return Operation{}, errOperationPreconditions
		}

		offset := int64(r.Intn(int(fileSize)))
		remaining := fileSize - offset
		if remaining <= 0 {
			return Operation{}, errOperationPreconditions
		}

		maxLen := maxDataSize
		if int64(maxLen) > remaining {
			maxLen = int(remaining)
		}

		return Operation{
			Kind:   kind,
			Path:   filePath,
			Offset: offset,
			Data:   randomBytes(r, randomDataLength(r, maxLen)),
		}, nil

	case OperationKindSetXAttr:
		paths := state.sortedPaths(true)
		if len(paths) == 0 {
			return Operation{}, errOperationPreconditions
		}

		target := paths[r.Intn(len(paths))]
		node := state.nodes[target]

		name := fmt.Sprintf("user.mut_%03d", r.Intn(1000))
		if node.xattrs != nil {
			for attempt := 0; attempt < maxMutationPathCollisionRetries; attempt++ {
				candidate := fmt.Sprintf("user.mut_%03d", r.Intn(1000))
				if _, exists := node.xattrs[candidate]; exists {
					continue
				}

				name = candidate
				break
			}
		}

		return Operation{
			Kind:       kind,
			Path:       target,
			XAttrName:  name,
			XAttrValue: randomBytes(r, randomDataLength(r, maxDataSize)),
		}, nil

	case OperationKindRemoveXAttr:
		targets := state.sortedPathsWithXAttrs(true)
		if len(targets) == 0 {
			return Operation{}, errOperationPreconditions
		}

		target := targets[r.Intn(len(targets))]
		names := state.sortedNodeXAttrs(target)
		if len(names) == 0 {
			return Operation{}, errOperationPreconditions
		}

		return Operation{
			Kind:      kind,
			Path:      target,
			XAttrName: names[r.Intn(len(names))],
		}, nil

	default:
		return Operation{}, fmt.Errorf("%w: %s", ErrUnsupportedOperation, kind)
	}
}

func (state *mutationState) sortedPaths(includeRoot bool) []string {
	state.rebuildPathIndexes()
	if includeRoot {
		return state.indexAll
	}

	if len(state.indexAll) > 0 && state.indexAll[0] == "/" {
		return state.indexAll[1:]
	}

	return state.indexAll
}

func (state *mutationState) sortedDirPaths() []string {
	state.rebuildPathIndexes()

	return state.indexDirs
}

func (state *mutationState) sortedFilePaths() []string {
	state.rebuildPathIndexes()

	return state.indexFiles
}

func (state *mutationState) sortedPathsWithXAttrs(includeRoot bool) []string {
	state.rebuildPathIndexes()
	if includeRoot {
		return state.indexXAttrs
	}

	if len(state.indexXAttrs) > 0 && state.indexXAttrs[0] == "/" {
		return state.indexXAttrs[1:]
	}

	return state.indexXAttrs
}

func (state *mutationState) sortedNodeXAttrs(path string) []string {
	node, ok := state.nodes[path]
	if !ok || len(node.xattrs) == 0 {
		return nil
	}

	names := make([]string, 0, len(node.xattrs))
	for name := range node.xattrs {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

func (state *mutationState) planUniquePath(r *rand.Rand) (string, error) {
	dirs := state.sortedDirPaths()
	if len(dirs) == 0 {
		return "", fmt.Errorf("plan unique path: %w", errOperationPreconditions)
	}

	for attempt := 1; attempt <= maxMutationPathCollisionRetries; attempt++ {
		parent := dirs[r.Intn(len(dirs))]
		candidate := gopath.Clean(gopath.Join(parent, state.nextMutationName(r)))
		if candidate == "/" {
			continue
		}

		if _, exists := state.nodes[candidate]; exists {
			continue
		}

		return candidate, nil
	}

	return "", ErrMutationPathCollisionExhausted
}

func (state *mutationState) nextMutationName(r *rand.Rand) string {
	name := fmt.Sprintf("mut_%04d_%02x", state.nameSeq, r.Intn(256))
	state.nameSeq++

	return name
}

func (state *mutationState) applyVirtualOperation(op Operation) error {
	// Any successful operation may invalidate the path indexes. Even read-only
	// kinds (chmod/chown) cost effectively nothing to invalidate, so always
	// drop the cache here and let the next planNextOperation rebuild on demand.
	state.invalidatePathIndexes()

	switch op.Kind {
	case OperationKindCreateFile:
		if _, exists := state.nodes[op.Path]; exists {
			return fmt.Errorf("create-file path `%s` already exists", op.Path)
		}
		if !state.hasParentDirectory(op.Path) {
			return fmt.Errorf("create-file parent directory missing for `%s`", op.Path)
		}
		state.nodes[op.Path] = mutationNode{typeID: mutationNodeTypeFile, size: int64(len(op.Data)), xattrs: make(map[string]struct{})}

	case OperationKindCreateDir:
		if _, exists := state.nodes[op.Path]; exists {
			return fmt.Errorf("create-dir path `%s` already exists", op.Path)
		}
		if !state.hasParentDirectory(op.Path) {
			return fmt.Errorf("create-dir parent directory missing for `%s`", op.Path)
		}
		state.nodes[op.Path] = mutationNode{typeID: mutationNodeTypeDir, xattrs: make(map[string]struct{})}

	case OperationKindCreateSymlink:
		if _, exists := state.nodes[op.Path]; exists {
			return fmt.Errorf("create-symlink path `%s` already exists", op.Path)
		}
		if !state.hasParentDirectory(op.Path) {
			return fmt.Errorf("create-symlink parent directory missing for `%s`", op.Path)
		}
		state.nodes[op.Path] = mutationNode{typeID: mutationNodeTypeSymlink, xattrs: make(map[string]struct{})}

	case OperationKindCreateHardlink:
		source, exists := state.nodes[op.SourcePath]
		if !exists {
			return fmt.Errorf("create-hardlink source `%s` does not exist", op.SourcePath)
		}
		if source.typeID != mutationNodeTypeFile {
			return fmt.Errorf("create-hardlink source `%s` is not a regular file", op.SourcePath)
		}
		if _, exists := state.nodes[op.Path]; exists {
			return fmt.Errorf("create-hardlink path `%s` already exists", op.Path)
		}
		if !state.hasParentDirectory(op.Path) {
			return fmt.Errorf("create-hardlink parent directory missing for `%s`", op.Path)
		}
		nextNode := mutationNode{typeID: mutationNodeTypeFile, size: source.size, xattrs: make(map[string]struct{}, len(source.xattrs))}
		for name := range source.xattrs {
			nextNode.xattrs[name] = struct{}{}
		}
		state.nodes[op.Path] = nextNode

	case OperationKindDelete:
		node, exists := state.nodes[op.Path]
		if !exists {
			return fmt.Errorf("delete path `%s` does not exist", op.Path)
		}
		if node.typeID == mutationNodeTypeDir {
			for path := range state.nodes {
				if path == op.Path || isChildMutationPath(path, op.Path) {
					delete(state.nodes, path)
				}
			}
			return nil
		}
		delete(state.nodes, op.Path)

	case OperationKindRename:
		node, exists := state.nodes[op.Path]
		if !exists {
			return fmt.Errorf("rename source path `%s` does not exist", op.Path)
		}
		if _, exists := state.nodes[op.Destination]; exists {
			return fmt.Errorf("rename destination path `%s` already exists", op.Destination)
		}
		if !state.hasParentDirectory(op.Destination) {
			return fmt.Errorf("rename destination parent directory missing for `%s`", op.Destination)
		}
		if node.typeID == mutationNodeTypeDir && isChildMutationPath(op.Destination, op.Path) {
			return fmt.Errorf("rename destination `%s` is inside source directory `%s`", op.Destination, op.Path)
		}

		if node.typeID != mutationNodeTypeDir {
			delete(state.nodes, op.Path)
			state.nodes[op.Destination] = node
			return nil
		}

		updates := make(map[string]mutationNode)
		toDelete := make([]string, 0)
		for path, childNode := range state.nodes {
			if path != op.Path && !isChildMutationPath(path, op.Path) {
				continue
			}

			suffix := strings.TrimPrefix(path, op.Path)
			targetPath := gopath.Clean(op.Destination + suffix)
			updates[targetPath] = childNode
			toDelete = append(toDelete, path)
		}

		for _, path := range toDelete {
			delete(state.nodes, path)
		}
		for path, childNode := range updates {
			state.nodes[path] = childNode
		}

	case OperationKindChmod:
		if _, exists := state.nodes[op.Path]; !exists {
			return fmt.Errorf("chmod path `%s` does not exist", op.Path)
		}

	case OperationKindChown:
		if _, exists := state.nodes[op.Path]; !exists {
			return fmt.Errorf("chown path `%s` does not exist", op.Path)
		}

	case OperationKindTruncate:
		node, exists := state.nodes[op.Path]
		if !exists {
			return fmt.Errorf("truncate path `%s` does not exist", op.Path)
		}
		if node.typeID != mutationNodeTypeFile {
			return fmt.Errorf("truncate path `%s` is not a regular file", op.Path)
		}
		node.size = op.Size
		state.nodes[op.Path] = node

	case OperationKindAppend:
		node, exists := state.nodes[op.Path]
		if !exists {
			return fmt.Errorf("append path `%s` does not exist", op.Path)
		}
		if node.typeID != mutationNodeTypeFile {
			return fmt.Errorf("append path `%s` is not a regular file", op.Path)
		}
		node.size += int64(len(op.Data))
		state.nodes[op.Path] = node

	case OperationKindOverwriteRange:
		node, exists := state.nodes[op.Path]
		if !exists {
			return fmt.Errorf("overwrite-range path `%s` does not exist", op.Path)
		}
		if node.typeID != mutationNodeTypeFile {
			return fmt.Errorf("overwrite-range path `%s` is not a regular file", op.Path)
		}
		end := op.Offset + int64(len(op.Data))
		if end > node.size {
			node.size = end
			state.nodes[op.Path] = node
		}

	case OperationKindSetXAttr, OperationKindRemoveXAttr:
		node, exists := state.nodes[op.Path]
		if !exists {
			return fmt.Errorf("%s path `%s` does not exist", op.Kind, op.Path)
		}

		if _, err := validateXAttrName(op.XAttrName); err != nil {
			return fmt.Errorf("%s xattr name `%s`: %w", op.Kind, op.XAttrName, err)
		}

		if node.xattrs == nil {
			node.xattrs = make(map[string]struct{})
		}

		if op.Kind == OperationKindSetXAttr {
			node.xattrs[op.XAttrName] = struct{}{}
			state.nodes[op.Path] = node
			return nil
		}

		if _, has := node.xattrs[op.XAttrName]; !has {
			return fmt.Errorf("remove-xattr name `%s` does not exist on `%s`", op.XAttrName, op.Path)
		}

		delete(node.xattrs, op.XAttrName)
		state.nodes[op.Path] = node

	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedOperation, op.Kind)
	}

	return nil
}

func (state *mutationState) hasParentDirectory(path string) bool {
	parent := gopath.Dir(path)
	if parent == "." {
		parent = "/"
	}

	node, exists := state.nodes[parent]
	if !exists {
		return false
	}

	return node.typeID == mutationNodeTypeDir
}

func isChildMutationPath(path, parent string) bool {
	if parent == "/" {
		return path != "/"
	}

	prefix := parent + "/"

	return strings.HasPrefix(path, prefix)
}

func randomDataLength(r *rand.Rand, max int) int {
	if max <= 1 {
		return 1
	}

	return r.Intn(max) + 1
}

func randomBytes(r *rand.Rand, size int) []byte {
	data := make([]byte, size)
	_, _ = io.ReadFull(r, data)

	return data
}

func randomFileMode(r *rand.Rand) uint32 {
	return uint32(0o600 + r.Intn(0o200))
}

func randomDirMode(r *rand.Rand) uint32 {
	return uint32(0o700 + r.Intn(0o100))
}

func applyOperation(basePath string, op Operation) error {
	path, err := operationPathToFS(basePath, op.Path)
	if err != nil {
		return fmt.Errorf("invalid operation path `%s`: %w", op.Path, err)
	}
	if err := ensureNoSymlinkParents(basePath, path); err != nil {
		return fmt.Errorf("operation path `%s`: %w", op.Path, err)
	}

	switch op.Kind {
	case OperationKindCreateFile:
		if err := ensureParentDirectory(path); err != nil {
			return fmt.Errorf("create-file parent check failed for `%s`: %w", op.Path, err)
		}
		if exists, err := pathExists(path); err != nil {
			return fmt.Errorf("create-file path check failed for `%s`: %w", op.Path, err)
		} else if exists {
			return fmt.Errorf("create-file path `%s` already exists", op.Path)
		}

		if err := os.WriteFile(path, op.Data, fs.FileMode(op.Mode)); err != nil {
			return fmt.Errorf("failed to write file `%s`: %w", op.Path, err)
		}

	case OperationKindCreateDir:
		if err := ensureParentDirectory(path); err != nil {
			return fmt.Errorf("create-dir parent check failed for `%s`: %w", op.Path, err)
		}
		if exists, err := pathExists(path); err != nil {
			return fmt.Errorf("create-dir path check failed for `%s`: %w", op.Path, err)
		} else if exists {
			return fmt.Errorf("create-dir path `%s` already exists", op.Path)
		}

		if err := os.Mkdir(path, fs.FileMode(op.Mode)); err != nil {
			return fmt.Errorf("failed to create directory `%s`: %w", op.Path, err)
		}

	case OperationKindCreateSymlink:
		if err := ensureParentDirectory(path); err != nil {
			return fmt.Errorf("create-symlink parent check failed for `%s`: %w", op.Path, err)
		}
		if exists, err := pathExists(path); err != nil {
			return fmt.Errorf("create-symlink path check failed for `%s`: %w", op.Path, err)
		} else if exists {
			return fmt.Errorf("create-symlink path `%s` already exists", op.Path)
		}

		if err := os.Symlink(op.LinkTarget, path); err != nil {
			return fmt.Errorf("failed to create symlink `%s` -> `%s`: %w", op.Path, op.LinkTarget, err)
		}

	case OperationKindCreateHardlink:
		source, err := operationPathToFS(basePath, op.SourcePath)
		if err != nil {
			return fmt.Errorf("invalid hardlink source path `%s`: %w", op.SourcePath, err)
		}
		if err := ensureNoSymlinkParents(basePath, source); err != nil {
			return fmt.Errorf("hardlink source path `%s`: %w", op.SourcePath, err)
		}
		if err := ensureParentDirectory(path); err != nil {
			return fmt.Errorf("create-hardlink parent check failed for `%s`: %w", op.Path, err)
		}
		if exists, err := pathExists(path); err != nil {
			return fmt.Errorf("create-hardlink path check failed for `%s`: %w", op.Path, err)
		} else if exists {
			return fmt.Errorf("create-hardlink path `%s` already exists", op.Path)
		}

		sourceInfo, err := os.Lstat(source)
		if err != nil {
			return fmt.Errorf("failed to inspect hardlink source `%s`: %w", op.SourcePath, err)
		}
		if !sourceInfo.Mode().IsRegular() {
			return fmt.Errorf("hardlink source `%s` is not a regular file", op.SourcePath)
		}

		if err := os.Link(source, path); err != nil {
			return fmt.Errorf("failed to create hardlink `%s` -> `%s`: %w", op.Path, op.SourcePath, err)
		}

	case OperationKindDelete:
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("failed to inspect delete path `%s`: %w", op.Path, err)
		}

		if info.IsDir() {
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("failed to remove directory `%s`: %w", op.Path, err)
			}
			return nil
		}

		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to remove path `%s`: %w", op.Path, err)
		}

	case OperationKindRename:
		destination, err := operationPathToFS(basePath, op.Destination)
		if err != nil {
			return fmt.Errorf("invalid rename destination path `%s`: %w", op.Destination, err)
		}
		if err := ensureNoSymlinkParents(basePath, destination); err != nil {
			return fmt.Errorf("rename destination path `%s`: %w", op.Destination, err)
		}
		sourceInfo, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("failed to inspect rename source `%s`: %w", op.Path, err)
		}
		if sourceInfo.IsDir() && isChildMutationPath(op.Destination, op.Path) {
			return fmt.Errorf("rename destination `%s` is inside source directory `%s`", op.Destination, op.Path)
		}
		if err := ensureParentDirectory(destination); err != nil {
			return fmt.Errorf("rename destination parent check failed for `%s`: %w", op.Destination, err)
		}
		if exists, err := pathExists(destination); err != nil {
			return fmt.Errorf("rename destination check failed for `%s`: %w", op.Destination, err)
		} else if exists {
			return fmt.Errorf("rename destination `%s` already exists", op.Destination)
		}

		if err := os.Rename(path, destination); err != nil {
			return fmt.Errorf("failed to rename `%s` -> `%s`: %w", op.Path, op.Destination, err)
		}

	case OperationKindChmod:
		if _, err := os.Lstat(path); err != nil {
			return fmt.Errorf("failed to inspect chmod path `%s`: %w", op.Path, err)
		}

		if err := os.Chmod(path, fs.FileMode(op.Mode)); err != nil {
			return fmt.Errorf("failed to chmod `%s` to %#o: %w", op.Path, op.Mode, err)
		}

	case OperationKindChown:
		if _, err := os.Lstat(path); err != nil {
			return fmt.Errorf("failed to inspect chown path `%s`: %w", op.Path, err)
		}

		if err := os.Lchown(path, op.UID, op.GID); err != nil {
			return fmt.Errorf("failed to chown `%s` to uid=%d gid=%d: %w", op.Path, op.UID, op.GID, err)
		}

	case OperationKindTruncate:
		if _, err := ensureRegularFile(path); err != nil {
			return fmt.Errorf("truncate `%s`: %w", op.Path, err)
		}

		if err := os.Truncate(path, op.Size); err != nil {
			return fmt.Errorf("failed to truncate `%s` to %d bytes: %w", op.Path, op.Size, err)
		}

	case OperationKindAppend:
		if _, err := ensureRegularFile(path); err != nil {
			return fmt.Errorf("append `%s`: %w", op.Path, err)
		}

		if err := appendToOperationFile(path, op.Path, op.Data); err != nil {
			return err
		}

	case OperationKindOverwriteRange:
		info, err := ensureRegularFile(path)
		if err != nil {
			return fmt.Errorf("overwrite-range `%s`: %w", op.Path, err)
		}
		if op.Offset > info.Size() {
			return fmt.Errorf(
				"overwrite-range offset %d exceeds file size %d for `%s`",
				op.Offset,
				info.Size(),
				op.Path,
			)
		}

		if err := overwriteOperationFile(path, op.Path, op.Offset, op.Data); err != nil {
			return err
		}

	case OperationKindSetXAttr:
		if _, err := os.Lstat(path); err != nil {
			return fmt.Errorf("failed to inspect set-xattr path `%s`: %w", op.Path, err)
		}

		if err := setPathXAttr(path, op.XAttrName, op.XAttrValue); err != nil {
			return fmt.Errorf("set-xattr `%s` on `%s`: %w", op.XAttrName, op.Path, err)
		}

	case OperationKindRemoveXAttr:
		if _, err := os.Lstat(path); err != nil {
			return fmt.Errorf("failed to inspect remove-xattr path `%s`: %w", op.Path, err)
		}

		if err := removePathXAttr(path, op.XAttrName); err != nil {
			return fmt.Errorf("remove-xattr `%s` on `%s`: %w", op.XAttrName, op.Path, err)
		}

	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedOperation, op.Kind)
	}

	return nil
}

func appendToOperationFile(fsPath, opPath string, data []byte) (err error) {
	f, err := os.OpenFile(fsPath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return fmt.Errorf("failed to open append target `%s`: %w", opPath, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to finalize append to `%s`: %w", opPath, closeErr)
		}
	}()

	nWritten, err := f.Write(data)
	if err != nil {
		return fmt.Errorf("failed to append to `%s`: %w", opPath, err)
	}
	if nWritten != len(data) {
		return fmt.Errorf("append to `%s` wrote %d bytes, expected %d", opPath, nWritten, len(data))
	}

	return nil
}

func overwriteOperationFile(fsPath, opPath string, offset int64, data []byte) (err error) {
	f, err := os.OpenFile(fsPath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open overwrite target `%s`: %w", opPath, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to finalize overwrite of `%s`: %w", opPath, closeErr)
		}
	}()

	nWritten, err := f.WriteAt(data, offset)
	if err != nil {
		return fmt.Errorf("failed to overwrite `%s` at offset %d: %w", opPath, offset, err)
	}
	if nWritten != len(data) {
		return fmt.Errorf("overwrite-range on `%s` wrote %d bytes, expected %d", opPath, nWritten, len(data))
	}

	return nil
}

func operationPathToFS(basePath, opPath string) (string, error) {
	cleanBase := filepath.Clean(basePath)

	if opPath == "/" {
		return cleanBase, nil
	}

	relPath := strings.TrimPrefix(opPath, "/")
	if relPath == "" {
		return "", fmt.Errorf("%w: empty relative path", ErrOperationPathEscapesBase)
	}

	fullPath := filepath.Join(cleanBase, filepath.FromSlash(relPath))
	if err := ensurePathWithinBase(cleanBase, fullPath); err != nil {
		return "", err
	}

	return fullPath, nil
}

func ensurePathWithinBase(basePath, path string) error {
	rel, err := filepath.Rel(basePath, path)
	if err != nil {
		return fmt.Errorf("%w: failed to derive relative path for `%s`: %v", ErrOperationPathEscapesBase, path, err)
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%w: `%s`", ErrOperationPathEscapesBase, path)
	}

	return nil
}

func ensureNoSymlinkParents(basePath, path string) error {
	cleanBase := filepath.Clean(basePath)
	cleanPath := filepath.Clean(path)

	if err := ensurePathWithinBase(cleanBase, cleanPath); err != nil {
		return err
	}
	if cleanPath == cleanBase {
		return nil
	}

	parent := filepath.Dir(cleanPath)
	if parent == cleanPath {
		return nil
	}

	relParent, err := filepath.Rel(cleanBase, parent)
	if err != nil {
		return fmt.Errorf("failed to derive parent path for `%s`: %w", cleanPath, err)
	}
	if relParent == "." {
		return nil
	}

	current := cleanBase
	for _, component := range strings.Split(relParent, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}

		current = filepath.Join(current, component)

		info, err := os.Lstat(current)
		if err != nil {
			return fmt.Errorf("failed to inspect parent component `%s`: %w", current, err)
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: `%s`", ErrOperationPathSymlinkParent, current)
		}
		if !info.IsDir() {
			return fmt.Errorf("parent component `%s` is not a directory", current)
		}
	}

	return nil
}

func ensureParentDirectory(path string) error {
	parent := filepath.Dir(path)
	parentInfo, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("failed to inspect parent `%s`: %w", parent, err)
	}
	if !parentInfo.IsDir() {
		return fmt.Errorf("parent `%s` is not a directory", parent)
	}

	return nil
}

func ensureRegularFile(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect path `%s`: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("path `%s` is not a regular file", path)
	}

	return info, nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, err
}

func newOperationApplyError(index int, op Operation, ops []Operation, err error) error {
	// Capture the source ops without serializing yet. ReplaySpec/Error will
	// trigger the (potentially large) JSON encoding only when the spec is
	// actually inspected.
	return &OperationApplyError{
		Index:     index,
		Operation: op,
		Err:       err,
		specOps:   ops,
	}
}

func operationPathLabel(op Operation) string {
	switch op.Kind {
	case OperationKindRename:
		return fmt.Sprintf("%s->%s", op.Path, op.Destination)
	case OperationKindCreateHardlink:
		return fmt.Sprintf("%s<- %s", op.Path, op.SourcePath)
	default:
		return op.Path
	}
}
