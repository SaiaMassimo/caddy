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

package binomial

import (
	"fmt"
	"sync"
)

// Memento represents the memento replacement set lookup table.
// It allows any consistent hashing engine to be stable against random node removals.
//
// Author: Massimo Coluzzi
type Memento struct {
	// The minimum size of the memento table
	minTableSize int
	
	// The maximum size of the memento table
	maxTableSize int
	
	// Stores the information about the removed buckets
	table []*Entry
	
	// The number of removed buckets
	size int
	
	// Mutex for thread safety
	mu sync.RWMutex
}

// Entry represents an entry in the lookup table
type Entry struct {
	// The removed bucket
	bucket int
	
	// Represents the bucket that will replace the current one.
	// This value also represents the size of the working set
	// after the removal of the current bucket.
	replacer int
	
	// Keep track of the bucket removed before the current one
	prevRemoved int
	
	// Used if multiple entries have the same hashcode (chaining)
	next *Entry
}

// NewMemento creates a new Memento instance
func NewMemento() *Memento {
	return &Memento{
		minTableSize: 1 << 4,  // 16
		maxTableSize: 1 << 30, // ~1 billion
		table:        make([]*Entry, 1<<4),
		size:         0,
	}
}

// Remember remembers that the given bucket has been removed
// and that was replaced by the given replacer.
// This method also stores the last removed bucket
// (before the current one) to create the sequence of removals.
//
// Returns the value of the new last removed bucket
func (m *Memento) Remember(bucket, replacer, prevRemoved int) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	entry := &Entry{
		bucket:      bucket,
		replacer:    replacer,
		prevRemoved: prevRemoved,
		next:        nil,
	}
	
	m.add(entry, m.table)
	m.size++
	
	if m.size > m.capacity() {
		m.resizeTable(len(m.table) << 1)
	}
	
	return bucket
}

// Replacer returns the replacer of the bucket if it
// was removed, otherwise returns -1.
// The value returned by this method represents
// both the bucket that replaced the given one
// and the size of the working set after removing
// the given bucket.
func (m *Memento) Replacer(bucket int) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	entry := m.get(bucket)
	if entry != nil {
		return entry.replacer
	}
	return -1
}

// Restore restores the given bucket by removing it
// from the memory.
// If the memory is empty the last removed bucket
// becomes the given bucket + 1.
//
// Returns the new last removed bucket
func (m *Memento) Restore(bucket int) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.isEmpty() {
		return bucket + 1
	}
	
	entry := m.remove(bucket)
	if entry == nil {
		return bucket + 1
	}
	
	m.size--
	
	if m.size <= m.capacity()>>2 {
		m.resizeTable(len(m.table) >> 1)
	}
	
	return entry.prevRemoved
}

// IsEmpty returns true if the replacement set is empty
func (m *Memento) IsEmpty() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.size <= 0
}

// Size returns the size of the replacement set
func (m *Memento) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.size
}

// Capacity returns the size of the lookup table used to implement the replacement set.
// We want to keep a load factor of 0.75 to have an average access time of O(1).
// For this reason, the declared capacity is 75% of the actual capacity.
func (m *Memento) capacity() int {
	return (len(m.table) >> 2) * 3
}

// isEmpty returns true if the replacement set is empty (internal use, no locking)
func (m *Memento) isEmpty() bool {
	return m.size <= 0
}

// add adds a new entry to the given table.
// This method is used to add entries to the lookup table
// during common operations and to add entries to the new
// lookup table during resize.
// We assume the algorithm to be used properly.
// Therefore, we do not handle the case of the same entry
// being added twice.
func (m *Memento) add(entry *Entry, table []*Entry) {
	// We use the same approach adopted by java.util.HashMap
	// to compute the index. It is proven to be efficient
	// in the majority of the cases.
	bucket := entry.bucket
	hash := bucket ^ (bucket >> 16)
	index := (len(table) - 1) & hash
	
	entry.next = table[index]
	table[index] = entry
}

// get returns the entry related to the given bucket if any
func (m *Memento) get(bucket int) *Entry {
	// We use the same approach adopted by java.util.HashMap
	// to compute the index. It is proven to be efficient
	// in the majority of the cases.
	hash := bucket ^ (bucket >> 16)
	index := (len(m.table) - 1) & hash
	
	entry := m.table[index]
	for entry != nil {
		if entry.bucket == bucket {
			return entry
		}
		entry = entry.next
	}
	
	return nil
}

// remove removes the given bucket from the lookup table
func (m *Memento) remove(bucket int) *Entry {
	hash := bucket ^ (bucket >> 16)
	index := (len(m.table) - 1) & hash
	
	entry := m.table[index]
	if entry == nil {
		return nil
	}
	
	var prev *Entry
	for entry != nil && entry.bucket != bucket {
		prev = entry
		entry = entry.next
	}
	
	if entry == nil {
		return nil
	}
	
	if prev == nil {
		m.table[index] = entry.next
	} else {
		prev.next = entry.next
	}
	
	entry.next = nil
	return entry
}

// resizeTable resizes the lookup table by creating a new table and cloning
// the entries in the old table into the new one
func (m *Memento) resizeTable(newTableSize int) {
	if newTableSize < len(m.table) && len(m.table) <= m.minTableSize {
		return
	}
	
	if newTableSize > len(m.table) && len(m.table) >= m.maxTableSize {
		return
	}
	
	newTable := make([]*Entry, newTableSize)
	for i := 0; i < len(m.table); i++ {
		entry := m.table[i]
		for entry != nil {
			newEntry := &Entry{
				bucket:      entry.bucket,
				replacer:    entry.replacer,
				prevRemoved: entry.prevRemoved,
				next:        nil,
			}
			m.add(newEntry, newTable)
			entry = entry.next
		}
	}
	
	m.table = newTable
}

// String returns a string representation of the Memento
func (m *Memento) String() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return fmt.Sprintf("Memento{size=%d, capacity=%d, table_size=%d}", 
		m.size, m.capacity(), len(m.table))
}
