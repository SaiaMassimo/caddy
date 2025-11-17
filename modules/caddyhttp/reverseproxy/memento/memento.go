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
	"sync"
	"sync/atomic"
)

// Memento represents the memento replacement set lookup table.
// It tracks which buckets have been removed and their replacements.
//
// Author: Massimo Coluzzi
type Memento struct {
	// Stores the information about the removed buckets
	// Using atomic.Value for lock-free reads during resize
	table atomic.Value // stores []*Entry

	// The number of removed buckets (must be accessed with mu)
	size int64 // using int64 for atomic operations

	// The minimum size of the memento table
	minTableSize int

	// The maximum size of the memento table
	maxTableSize int

	// Mutex for thread safety - used for writes (add, remove, resize)
	// Reads can proceed lock-free using atomic.Value
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
	m := &Memento{
		size:         0,
		minTableSize: 1 << 4,  // 16
		maxTableSize: 1 << 30, // ~1 billion
	}
	m.table.Store(make([]*Entry, 1<<4)) // 16
	return m
}

// Remember remembers that the given bucket has been removed
// and that was replaced by the given replacer.
// This method also stores the last removed bucket
// (before the current one) to create the sequence of removals.
//
// Returns the value of the new last removed bucket
// Note: This operation requires a write lock for thread safety.
func (m *Memento) Remember(bucket, replacer, prevRemoved int) int {
	entry := &Entry{
		bucket:      bucket,
		replacer:    replacer,
		prevRemoved: prevRemoved,
		next:        nil,
	}

	m.mu.Lock()
	table := m.getTable()
	m.add(entry, table)
	newSize := atomic.AddInt64(&m.size, 1)
	m.mu.Unlock()

	// Check if resize is needed (outside the lock to avoid blocking reads)
	tableLen := len(table)
	if int(newSize) > m.capacityForSize(tableLen) {
		m.resizeTable(tableLen << 1)
	}

	return bucket
}

// Replacer returns the replacer of the bucket if it
// was removed, otherwise returns -1.
// The value returned by this method represents
// both the bucket that replaced the given one
// and the size of the working set after removing
// the given bucket.
// Note: This operation is lock-free using atomic.Value.
func (m *Memento) Replacer(bucket int) int {
	table := m.getTable()
	entry := m.get(bucket, table)
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
// Note: This operation requires a write lock for thread safety.
func (m *Memento) Restore(bucket int) int {
	if m.isEmpty() {
		return bucket + 1
	}

	m.mu.Lock()
	table := m.getTable()
	entry := m.remove(bucket, table)
	if entry == nil {
		m.mu.Unlock()
		return bucket + 1
	}

	newSize := atomic.AddInt64(&m.size, -1)
	tableLen := len(table)
	m.mu.Unlock()

	// Check if resize is needed (outside the lock to avoid blocking reads)
	if int(newSize) <= m.capacityForSize(tableLen)>>2 {
		m.resizeTable(tableLen >> 1)
	}

	return entry.prevRemoved
}

// IsEmpty returns true if the replacement set is empty
// Note: This operation is lock-free using atomic operations.
func (m *Memento) IsEmpty() bool {
	return atomic.LoadInt64(&m.size) <= 0
}

// Size returns the size of the replacement set
// Note: This operation is lock-free using atomic operations.
func (m *Memento) Size() int {
	return int(atomic.LoadInt64(&m.size))
}

// Capacity returns the size of the lookup table used to implement the replacement set.
// We want to keep a load factor of 0.75 to have an average access time of O(1).
// For this reason, the declared capacity is 75% of the actual capacity.
// Note: This operation is lock-free using atomic.Value.
func (m *Memento) Capacity() int {
	table := m.getTable()
	return m.capacityForSize(len(table))
}

// isEmpty returns true if the replacement set is empty (internal use, no locking)
func (m *Memento) isEmpty() bool {
	return atomic.LoadInt64(&m.size) <= 0
}

// capacityForSize returns the capacity for a given table size (internal use)
func (m *Memento) capacityForSize(tableSize int) int {
	return (tableSize >> 2) * 3
}

// getTable returns the current table (lock-free read)
func (m *Memento) getTable() []*Entry {
	table := m.table.Load()
	if table == nil {
		return make([]*Entry, 1<<4)
	}
	return table.([]*Entry)
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
// table parameter allows lock-free reads using the table snapshot
func (m *Memento) get(bucket int, table []*Entry) *Entry {
	// We use the same approach adopted by java.util.HashMap
	// to compute the index. It is proven to be efficient
	// in the majority of the cases.
	hash := bucket ^ (bucket >> 16)
	index := (len(table) - 1) & hash

	entry := table[index]
	for entry != nil {
		if entry.bucket == bucket {
			return entry
		}
		entry = entry.next
	}

	return nil
}

// remove removes the given bucket from the lookup table
// table parameter is the current table (must be called with lock held)
func (m *Memento) remove(bucket int, table []*Entry) *Entry {
	hash := bucket ^ (bucket >> 16)
	index := (len(table) - 1) & hash

	entry := table[index]
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
		table[index] = entry.next
	} else {
		prev.next = entry.next
	}

	entry.next = nil
	return entry
}

// resizeTable resizes the lookup table by creating a new table and cloning
// the entries in the old table into the new one.
// This operation uses a write lock, but reads can continue using the old table
// until the new table is atomically swapped in.
func (m *Memento) resizeTable(newTableSize int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldTable := m.getTable()
	oldTableSize := len(oldTable)

	// Double-check conditions after acquiring lock
	if newTableSize < oldTableSize && oldTableSize <= m.minTableSize {
		return
	}

	if newTableSize > oldTableSize && oldTableSize >= m.maxTableSize {
		return
	}

	// Create new table and clone entries
	newTable := make([]*Entry, newTableSize)
	for i := 0; i < oldTableSize; i++ {
		entry := oldTable[i]
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

	// Atomically swap the new table in
	// This allows concurrent reads to continue using the old table
	// until they finish, then they'll see the new table
	m.table.Store(newTable)
}

// String returns a string representation of the Memento
// Note: This operation is lock-free using atomic operations.
func (m *Memento) String() string {
	table := m.getTable()
	return fmt.Sprintf("Memento{size=%d, capacity=%d, table_size=%d}",
		atomic.LoadInt64(&m.size), m.capacityForSize(len(table)), len(table))
}
