package randfiletree

import (
	"math/rand"
	"time"
)

// TimestampGenerator denotes a generator function for timestamps.
type TimestampGenerator func(r *rand.Rand) time.Time

// TimestampGeneratorConstant returns a fixed timestamp generator.
func TimestampGeneratorConstant(ts time.Time) TimestampGenerator {
	return func(r *rand.Rand) time.Time {
		return ts
	}
}

type metadataConfig struct {
	hasOwnership bool
	uid          int
	gid          int

	hasTimestamps bool
	atime         time.Time
	mtime         time.Time
}

func (g *Generator) resolveMetadata(r *rand.Rand) metadataConfig {
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

	return metadata
}
