// Copyright 2024 Massimo Saia and The Caddy Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package memento

import (
	"math"
	"math/bits"
)

// BinomialEngine implements the BinomialHash algorithm as described in the paper:
// https://arxiv.org/pdf/2406.19836
//
// IMPORTANT: This implementation is not performing any consistency check
// to avoid the performance tests to be falsified.
//
// Author: Massimo Saia
type BinomialEngine struct {
	// Number of nodes in the cluster
	size int

	// This value is used to filter values in the range [0,enclosingTreeCapacity-1]
	// where the term enclosingTreeCapacity identifies the capacity of the smallest
	// binary tree capable of containing the cluster size.
	enclosingTreeFilter int

	// This value is used to filter values in the range [0,minorTreeCapacity-1]
	// where the term minorTreeCapacity identifies the capacity of the biggest
	// binary tree incapable of containing the cluster size.
	minorTreeFilter int
}

// NewBinomialEngine creates a new BinomialEngine with the given cluster size
func NewBinomialEngine(size int) *BinomialEngine {
	engine := &BinomialEngine{
		size: size,
	}

	highestOneBit := highestOneBit(size)
	if size > highestOneBit {
		highestOneBit = highestOneBit << 1
	}

	engine.enclosingTreeFilter = highestOneBit - 1
	engine.minorTreeFilter = engine.enclosingTreeFilter >> 1

	return engine
}

// GetBucket returns the index of the bucket where the given key should be mapped
func (be *BinomialEngine) GetBucket(key string) int {
	// If the cluster counts only one node we return such a node
	if be.size < 2 {
		return 0
	}

	// We get the hash of the provided key
	hash := be.hash(key)

	// We get a position within the enclosing tree based on the value of the key hash
	bucket := int(hash) & be.enclosingTreeFilter

	// We relocate the bucket randomly inside the same tree level
	bucket = be.relocateWithinLevel(bucket, hash)

	// If the final position is valid, we return it
	if bucket < be.size {
		return bucket
	}

	// Otherwise, we get a new random position in the enclosing tree
	// and return it if in the range [minorTreeFilter+1,size-1].
	// We repeat the operation up to 4 times to get a better balance
	h := hash
	for i := 0; i < 4; i++ {
		h = be.rehash(h, be.enclosingTreeFilter)
		bucket = int(h) & be.enclosingTreeFilter

		if bucket <= be.minorTreeFilter {
			break
		}

		if bucket < be.size {
			return bucket
		}
	}

	// Finally, if none of the previous operations succeed,
	// we remap the key in the range covered by the minor tree,
	// which is guaranteed valid
	bucket = int(hash) & be.minorTreeFilter
	return be.relocateWithinLevel(bucket, hash)
}

// AddBucket increases the cluster size by one and returns the new bucket index
func (be *BinomialEngine) AddBucket() int {
	newBucket := be.size

	be.size++
	if be.size == 1 {
		be.enclosingTreeFilter = 1
		be.minorTreeFilter = 0
	} else {
		highestOneBit := highestOneBit(be.size)
		if be.size > highestOneBit {
			highestOneBit = highestOneBit << 1
		}

		be.enclosingTreeFilter = highestOneBit - 1
		be.minorTreeFilter = be.enclosingTreeFilter >> 1
	}

	return newBucket
}

// RemoveBucket decreases the cluster size by one
func (be *BinomialEngine) RemoveBucket() int {
	be.size--

	if be.size <= be.minorTreeFilter {
		be.minorTreeFilter = be.minorTreeFilter >> 1
		be.enclosingTreeFilter = be.enclosingTreeFilter >> 1
	}

	return be.size
}

// Size returns the size of the cluster
func (be *BinomialEngine) Size() int {
	return be.size
}

// EnclosingTreeFilter returns the enclosingTreeFilter as described in the paper
func (be *BinomialEngine) EnclosingTreeFilter() int {
	return be.enclosingTreeFilter
}

// MinorTreeFilter returns the minorTreeFilter as described in the paper
func (be *BinomialEngine) MinorTreeFilter() int {
	return be.minorTreeFilter
}

// rehash is a linear congruential generator to create uniformly distributed values
func (be *BinomialEngine) rehash(value uint64, seed int) uint64 {
	hash := 2862933555777941757*uint64(value) + 1
	return (hash * hash * uint64(seed)) >> 32
}

// relocateWithinLevel returns a random position inside the same tree level of the provided bucket
func (be *BinomialEngine) relocateWithinLevel(bucket int, hash uint64) int {
	// If the bucket is 0 or 1, we are in the root of the tree.
	// Therefore, no relocation is needed
	if bucket < 2 {
		return bucket
	}

	levelBaseIndex := highestOneBit(bucket)
	levelFilter := levelBaseIndex - 1

	levelHash := be.rehash(hash, levelFilter)
	levelIndex := int(levelHash) & levelFilter

	return levelBaseIndex + levelIndex
}

// hash performs the hashing of the given string using MurmurHash3
func (be *BinomialEngine) hash(key string) uint64 {
	// Simple MurmurHash3 implementation for 32-bit hash
	// This is a simplified version - in production you might want to use
	// a proper MurmurHash3 library like github.com/spaolacci/murmur3
	bytes := []byte(key)

	// MurmurHash3 32-bit implementation
	const (
		c1 = 0xcc9e2d51
		c2 = 0x1b873593
		r1 = 15
		r2 = 13
		m  = 5
		n  = 0xe6546b64
	)

	h := uint32(0) // seed = 0
	length := len(bytes)

	// Process 4-byte chunks
	for i := 0; i < length-3; i += 4 {
		k := uint32(bytes[i]) | uint32(bytes[i+1])<<8 | uint32(bytes[i+2])<<16 | uint32(bytes[i+3])<<24
		k *= c1
		k = bits.RotateLeft32(k, r1)
		k *= c2

		h ^= k
		h = bits.RotateLeft32(h, r2)
		h = h*m + n
	}

	// Handle remaining bytes
	if length%4 != 0 {
		var k uint32
		switch length % 4 {
		case 3:
			k ^= uint32(bytes[length-3]) << 16
			fallthrough
		case 2:
			k ^= uint32(bytes[length-2]) << 8
			fallthrough
		case 1:
			k ^= uint32(bytes[length-1])
			k *= c1
			k = bits.RotateLeft32(k, r1)
			k *= c2
			h ^= k
		}
	}

	h ^= uint32(length)
	h ^= h >> 16
	h *= 0x85ebca6b
	h ^= h >> 13
	h *= 0xc2b2ae35
	h ^= h >> 16

	return uint64(math.Abs(float64(int32(h))))
}

// highestOneBit returns the highest one bit in the integer (equivalent to Java's Integer.highestOneBit)
func highestOneBit(i int) int {
	if i <= 0 {
		return 0
	}
	return 1 << (bits.Len(uint(i)) - 1)
}
