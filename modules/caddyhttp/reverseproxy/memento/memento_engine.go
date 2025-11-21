// Copyright 2024 Massimo Coluzzi and The Caddy Authors
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
	"fmt"
	"hash/fnv"
)

// MementoInterface defines the interface for both Memento implementations
type MementoInterface interface {
	Remember(bucket, replacer, prevRemoved int) int
	Replacer(bucket int) int
	Restore(bucket int) int
	IsEmpty() bool
	Size() int
	Capacity() int
	String() string
}

// MementoEngine combines Memento with BinomialEngine to provide
// consistent hashing that supports arbitrary node removals.
//
// Author: Massimo Coluzzi
type MementoEngine struct {
	// The memory of the removed nodes (replacement set)
	// Can be either *Memento (RWMutex version) or *MementoLockFree (lock-free version)
	memento MementoInterface

	// The underlying binomial engine
	binomialEngine *BinomialEngine

	// The last removed bucket
	lastRemoved int
}

// NewMementoEngine creates a new MementoEngine with the given initial size
// Uses the RWMutex version (Memento) by default
func NewMementoEngine(size int) *MementoEngine {
	return NewMementoEngineWithType(size, false)
}

// NewMementoEngineWithType creates a new MementoEngine with the given initial size
// and allows choosing between RWMutex version (lockFree=false) or lock-free version (lockFree=true)
func NewMementoEngineWithType(size int, lockFree bool) *MementoEngine {
	engine := NewBinomialEngine(size)
	var memento MementoInterface
	if lockFree {
		memento = NewMementoLockFree()
	} else {
		memento = NewMemento()
	}
	return &MementoEngine{
		memento:        memento,
		binomialEngine: engine,
		lastRemoved:    size,
	}
}

// GetBucket returns the bucket where the given key should be mapped.
func (me *MementoEngine) GetBucket(key string) int {
	b := me.binomialEngine.GetBucket(key)

	/*
	 * We check if the bucket was removed, if not we are done.
	 * If the bucket was removed the replacing bucket is >= 0,
	 * otherwise it is -1.
	 */
	replacer := me.memento.Replacer(b)

	for replacer >= 0 {
		/*
		 * If the bucket was removed, we must re-hash and find
		 * a new bucket in the remaining slots. To know the
		 * remaining slots, we look at 'replacer' that also
		 * represents the size of the working set when the bucket
		 * was removed and get a new bucket in [0,replacer-1].
		 */
		h := me.hashWithSeed(key, b)
		b = int(h % uint64(replacer))

		/*
		 * If we hit a removed bucket we follow the replacements
		 * until we get a working bucket or a bucket in the range
		 * [0,replacer-1]
		 */
		r := me.memento.Replacer(b)
		for r >= 0 && r >= replacer {
			b = r
			r = me.memento.Replacer(b)
		}

		/* Finally we update the entry of the external loop. */
		replacer = r
	}

	return b
}

// AddBucket adds a bucket back to the working set
func (me *MementoEngine) AddBucket() int {
	bucket := me.lastRemoved

	me.lastRemoved = me.memento.Restore(bucket)

	// Only add to binomial engine if the bucket is beyond current size
	if me.binomialArraySize() <= bucket {
		me.binomialEngine.AddBucket()
	}

	return bucket
}

// RemoveBucket removes a bucket from the working set
func (me *MementoEngine) RemoveBucket(bucket int) int {
	// Calculate working size
	mementoSize := me.memento.Size()
	binomialSize := me.binomialEngine.Size()
	workingSize := binomialSize - mementoSize
	mementoEmpty := mementoSize == 0

	// If memento is empty and removing the last bucket, remove from binomial directly
	if mementoEmpty && bucket == binomialSize-1 {
		me.binomialEngine.RemoveBucket()
		me.lastRemoved = bucket
		return bucket
	}

	// Otherwise, remember the removal in memento
	me.lastRemoved = me.memento.Remember(
		bucket,
		workingSize,
		me.lastRemoved,
	)

	return bucket
}

// Size returns the size of the working set
func (me *MementoEngine) Size() int {
	return me.binomialEngine.Size() - me.memento.Size()
}

// BinomialArraySize returns the size of the underlying binomial engine
func (me *MementoEngine) binomialArraySize() int {
	return me.binomialEngine.Size()
}

// String returns a string representation of the MementoEngine
func (me *MementoEngine) String() string {
	return fmt.Sprintf("MementoEngine{memento=%s, binomialSize=%d, lastRemoved=%d, size=%d, bArraySize=%d}",
		me.memento.String(),
		me.binomialArraySize(),
		me.lastRemoved,
		me.Size(),
		me.binomialArraySize(),
	)
}

// hashWithSeed returns a hash of the key with the given seed
// This simulates the hashFunction.hash(key, seed) from Java
func (me *MementoEngine) hashWithSeed(key string, seed int) uint64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	h.Write([]byte{byte(seed), byte(seed >> 8), byte(seed >> 16), byte(seed >> 24)})
	return h.Sum64()
}
