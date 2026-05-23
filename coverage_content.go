package randfiletree

import (
	"fmt"
	"hash/fnv"
)

// coverageContentClass enumerates the content strategies exercised for regular
// files in the deterministic coverage scenario.
type coverageContentClass uint8

const (
	coverageContentClassPlain coverageContentClass = iota
	coverageContentClassDenseRandom
	coverageContentClassSparseHoles
	coverageContentClassRepeatedBlocks
	coverageContentClassPartialRangeOverwrite
)

func (c coverageContentClass) String() string {
	switch c {
	case coverageContentClassPlain:
		return "plain"
	case coverageContentClassDenseRandom:
		return "dense-random"
	case coverageContentClassSparseHoles:
		return "sparse-holes"
	case coverageContentClassRepeatedBlocks:
		return "repeated-blocks"
	case coverageContentClassPartialRangeOverwrite:
		return "partial-range-overwrite"
	default:
		return fmt.Sprintf("unknown(%d)", c)
	}
}

var coverageContentClassAll = []coverageContentClass{
	coverageContentClassPlain,
	coverageContentClassDenseRandom,
	coverageContentClassSparseHoles,
	coverageContentClassRepeatedBlocks,
	coverageContentClassPartialRangeOverwrite,
}

// coverageSizeClass enumerates the file-size dimensions. The numeric values
// are chosen to straddle the 64 KiB content-write chunk boundary and to
// produce one multi-MiB sample.
type coverageSizeClass uint8

const (
	coverageSizeClassZero coverageSizeClass = iota
	coverageSizeClassOne
	coverageSizeClassBelowChunk
	coverageSizeClassExactChunk
	coverageSizeClassAboveChunk
	coverageSizeClassMultiMiB
)

func (c coverageSizeClass) String() string {
	switch c {
	case coverageSizeClassZero:
		return "size-0"
	case coverageSizeClassOne:
		return "size-1"
	case coverageSizeClassBelowChunk:
		return "size-below-chunk"
	case coverageSizeClassExactChunk:
		return "size-exact-chunk"
	case coverageSizeClassAboveChunk:
		return "size-above-chunk"
	case coverageSizeClassMultiMiB:
		return "size-multi-mib"
	default:
		return fmt.Sprintf("unknown(%d)", c)
	}
}

var coverageSizeClassAll = []coverageSizeClass{
	coverageSizeClassZero,
	coverageSizeClassOne,
	coverageSizeClassBelowChunk,
	coverageSizeClassExactChunk,
	coverageSizeClassAboveChunk,
	coverageSizeClassMultiMiB,
}

func coverageSizeBytes(class coverageSizeClass) int64 {
	chunk := int64(defaultContentWriteChunkSize)
	switch class {
	case coverageSizeClassZero:
		return 0
	case coverageSizeClassOne:
		return 1
	case coverageSizeClassBelowChunk:
		return chunk - 1
	case coverageSizeClassExactChunk:
		return chunk
	case coverageSizeClassAboveChunk:
		return chunk + 1
	case coverageSizeClassMultiMiB:
		return (1 << 20) + 7
	default:
		return chunk
	}
}

// coverageDeterministicSeed mixes a label and the cell id into an int63 seed
// used by content patterns. The result is reproducible across runs.
func coverageDeterministicSeed(label string, cellID uint64) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(label))
	_, _ = h.Write([]byte{0})
	var seedBytes [8]byte
	for i := 0; i < 8; i++ {
		seedBytes[i] = byte(cellID >> uint(8*i))
	}
	_, _ = h.Write(seedBytes[:])

	return int64(h.Sum64() & 0x7FFF_FFFF_FFFF_FFFF)
}

// coveragePlainData returns deterministic body bytes for the plain content
// class. Output depends only on (size, cellID).
func coveragePlainData(size int64, cellID uint64) []byte {
	if size <= 0 {
		return nil
	}

	data := make([]byte, size)
	stream := uint64(coverageDeterministicSeed("plain", cellID))
	for i := range data {
		stream = stream*6364136223846793005 + 1442695040888963407
		data[i] = byte(stream >> 56)
	}

	return data
}

// coverageContentFor builds the plannedFileContent payload for the (class,
// size, cellID) tuple. For the plain class, returns a zero-value plannedFileContent
// (i.e. pattern == 0) so the caller knows to populate entry.data instead.
//
// The plain class is signaled by pattern == 0; the caller is responsible for
// generating the byte body via coveragePlainData.
func coverageContentFor(class coverageContentClass, sizeClass coverageSizeClass, cellID uint64) (plannedFileContent, error) {
	size := coverageSizeBytes(sizeClass)

	switch class {
	case coverageContentClassPlain:
		return plannedFileContent{}, nil

	case coverageContentClassDenseRandom:
		return plannedFileContent{
			pattern:              ContentPatternDenseRandom,
			logicalSize:          size,
			expectedWrittenBytes: size,
			expectedSparse:       false,
			seed:                 coverageDeterministicSeed("dense-random", cellID),
		}, nil

	case coverageContentClassSparseHoles:
		content := plannedFileContent{
			pattern:     ContentPatternSparseHoles,
			logicalSize: size,
		}
		if size == 0 {
			return content, nil
		}

		length := coverageSparseSegmentLength(size, cellID)
		offset := coverageDeterministicOffset(size, length, cellID)

		content.sparseExtents = []plannedContentRange{{
			offset: offset,
			length: length,
			seed:   coverageDeterministicSeed("sparse-extent", cellID),
		}}
		content.expectedWrittenBytes = length
		content.expectedSparse = size > length

		return content, nil

	case coverageContentClassRepeatedBlocks:
		block := coverageRepeatedBlock(size, cellID)
		return plannedFileContent{
			pattern:              ContentPatternRepeatedBlocks,
			logicalSize:          size,
			expectedWrittenBytes: size,
			expectedSparse:       false,
			repeatedBlock:        block,
		}, nil

	case coverageContentClassPartialRangeOverwrite:
		content := plannedFileContent{
			pattern:              ContentPatternPartialRangeOverwrite,
			logicalSize:          size,
			expectedWrittenBytes: size,
			expectedSparse:       false,
			seed:                 coverageDeterministicSeed("partial-base", cellID),
		}
		if size == 0 {
			return content, nil
		}

		updates := make([]plannedContentRange, 0, defaultPartialOverwriteCount)
		for i := 0; i < defaultPartialOverwriteCount; i++ {
			length := coveragePartialSegmentLength(size, cellID, i)
			offset := coverageDeterministicOffset(size, length, cellID+uint64(i)+1)
			updates = append(updates, plannedContentRange{
				offset: offset,
				length: length,
				seed:   coverageDeterministicSeed(fmt.Sprintf("partial-extent-%d", i), cellID),
			})
			content.expectedWrittenBytes += length
		}

		content.overwriteExtents = updates
		return content, nil

	default:
		return plannedFileContent{}, fmt.Errorf("unsupported coverage content class %d", class)
	}
}

func coverageSparseSegmentLength(size int64, cellID uint64) int64 {
	if size <= 0 {
		return 0
	}
	if size == 1 {
		return 1
	}

	maxLength := size - 1
	if maxLength > defaultPatternSegmentMaxLength {
		maxLength = defaultPatternSegmentMaxLength
	}

	stream := uint64(coverageDeterministicSeed("sparse-length", cellID))
	pick := int64(stream%uint64(maxLength)) + 1
	if pick > maxLength {
		pick = maxLength
	}

	return pick
}

func coveragePartialSegmentLength(size int64, cellID uint64, index int) int64 {
	if size <= 0 {
		return 0
	}

	maxLength := size
	if maxLength > defaultPatternSegmentMaxLength {
		maxLength = defaultPatternSegmentMaxLength
	}

	stream := uint64(coverageDeterministicSeed(fmt.Sprintf("partial-length-%d", index), cellID))
	pick := int64(stream%uint64(maxLength)) + 1
	if pick > maxLength {
		pick = maxLength
	}
	if pick > size {
		pick = size
	}

	return pick
}

func coverageDeterministicOffset(size, length int64, cellID uint64) int64 {
	if size <= 0 || length <= 0 || length >= size {
		return 0
	}

	remaining := size - length
	if remaining <= 0 {
		return 0
	}

	stream := uint64(coverageDeterministicSeed("offset", cellID))
	return int64(stream % uint64(remaining+1))
}

func coverageRepeatedBlock(size int64, cellID uint64) []byte {
	blockLen := defaultContentWriteChunkSize
	if size > 0 && size < int64(blockLen) {
		blockLen = int(size)
	}
	if blockLen == 0 {
		blockLen = 1
	}

	stream := uint64(coverageDeterministicSeed("repeated-block", cellID))
	block := make([]byte, blockLen)
	for i := range block {
		stream = stream*6364136223846793005 + 1442695040888963407
		block[i] = byte(stream >> 56)
	}

	return block
}
