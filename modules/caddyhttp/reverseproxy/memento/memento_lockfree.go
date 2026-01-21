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
	"fmt"
	"sync/atomic"
)

// MementoLockFree represents the memento replacement set lookup table
// using copy-on-write and atomic.Value for lock-free reads.
// It tracks which buckets have been removed and their replacements.
//
// This version allows lock-free reads even during resize operations,
// at the cost of copying the entire table on every write operation.
//
// Author: Massimo Saia
type MementoLockFree struct {
	// Stores the information about the removed buckets
	// Using atomic.Value for lock-free reads during resize
	table atomic.Value // stores []*Entry

	// The number of removed buckets (using atomic operations)
	size int64

	// The minimum size of the memento table
	minTableSize int

	// The maximum size of the memento table
	maxTableSize int
}

// NewMementoLockFree creates a new MementoLockFree instance
func NewMementoLockFree() *MementoLockFree {
	m := &MementoLockFree{
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
// Note: This operation is lock-free - pointer assignments are atomic in Go.
func (m *MementoLockFree) Remember(bucket, replacer, prevRemoved int) int {
	entry := &Entry{
		bucket:      bucket,
		replacer:    replacer,
		prevRemoved: prevRemoved,
		next:        nil,
	}

	// Lock-free: add entry directly to the table
	// Pointer assignments are atomic in Go, so this is safe
	table := m.getTable()
	m.add(entry, table)
	newSize := atomic.AddInt64(&m.size, 1)
	tableLen := len(table)

	// Check if resize is needed
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
func (m *MementoLockFree) Replacer(bucket int) int {
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
// Note: This operation is lock-free - pointer assignments are atomic in Go.
func (m *MementoLockFree) Restore(bucket int) int {
	if m.isEmpty() {
		return bucket + 1
	}

	// Lock-free: remove entry directly from the table
	// Pointer assignments are atomic in Go, so this is safe
	table := m.getTable()
	entry := m.remove(bucket, table)
	if entry == nil {
		return bucket + 1
	}

	prevRemoved := entry.prevRemoved
	newSize := atomic.AddInt64(&m.size, -1)
	tableLen := len(table)

	// Check if resize is needed
	if int(newSize) <= m.capacityForSize(tableLen)>>2 {
		m.resizeTable(tableLen >> 1)
	}

	return prevRemoved
}

// IsEmpty returns true if the replacement set is empty
// Note: This operation is lock-free using atomic operations.
func (m *MementoLockFree) IsEmpty() bool {
	return atomic.LoadInt64(&m.size) <= 0
}

// Size returns the size of the replacement set
// Note: This operation is lock-free using atomic operations.
func (m *MementoLockFree) Size() int {
	return int(atomic.LoadInt64(&m.size))
}

// Capacity returns the size of the lookup table used to implement the replacement set.
// We want to keep a load factor of 0.75 to have an average access time of O(1).
// For this reason, the declared capacity is 75% of the actual capacity.
// Note: This operation is lock-free using atomic.Value.
func (m *MementoLockFree) Capacity() int {
	table := m.getTable()
	return m.capacityForSize(len(table))
}

// isEmpty returns true if the replacement set is empty (internal use, no locking)
func (m *MementoLockFree) isEmpty() bool {
	return atomic.LoadInt64(&m.size) <= 0
}

// capacityForSize returns the capacity for a given table size (internal use)
func (m *MementoLockFree) capacityForSize(tableSize int) int {
	return (tableSize >> 2) * 3
}

// getTable returns the current table (lock-free read)
func (m *MementoLockFree) getTable() []*Entry {
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
func (m *MementoLockFree) add(entry *Entry, table []*Entry) {
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
func (m *MementoLockFree) get(bucket int, table []*Entry) *Entry {
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
// Pointer assignments are atomic in Go, so this is safe for lock-free operations
func (m *MementoLockFree) remove(bucket int, table []*Entry) *Entry {
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
// This operation is lock-free: reads can continue using the old table
// until the new table is atomically swapped in.
func (m *MementoLockFree) resizeTable(newTableSize int) {
	oldTable := m.getTable()
	oldTableSize := len(oldTable)

	// Check conditions
	if newTableSize < oldTableSize && oldTableSize <= m.minTableSize {
		return
	}

	if newTableSize > oldTableSize && oldTableSize >= m.maxTableSize {
		return
	}

	// Double-check: if table size already matches, another thread did the resize
	if len(oldTable) == newTableSize {
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
	// If another thread already resized, this will overwrite it, but that's ok
	// as the new table contains all entries from the old one
	m.table.Store(newTable)
}

// String returns a string representation of the MementoLockFree
// Note: This operation is lock-free using atomic operations.
func (m *MementoLockFree) String() string {
	table := m.getTable()
	return fmt.Sprintf("MementoLockFree{size=%d, capacity=%d, table_size=%d}",
		atomic.LoadInt64(&m.size), m.capacityForSize(len(table)), len(table))
}
