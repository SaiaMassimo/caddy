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
	"sort"
)

// ConsistentEngine wraps MementoEngine to provide a load balancing policy
// that can handle arbitrary node removals and additions.
// It manages node topology (node IDs as strings) and delegates
// all hashing logic to MementoEngine.
type ConsistentEngine struct {
	// MementoEngine handles all hashing logic and arbitrary node removal
	engine *MementoEngine

	// Indirection: one-to-one mapping between node ID (string) and bucket (int)
	// This ensures consistency checks and proper mapping management
	indirection *Indirection

	// NOTE: Thread safety is handled at the MementoSelection level.
	// This engine is not thread-safe by itself and must be protected
	// by the caller's lock (typically MementoSelection.mu).
}

// NewConsistentEngine creates a new consistent engine with MementoEngine
func NewConsistentEngine() *ConsistentEngine {
	return &ConsistentEngine{
		engine:      NewMementoEngine(0),
		indirection: NewIndirection(0),
	}
}

// NewConsistentEngineWithType creates a new consistent engine with MementoEngine
// using the specified implementation type (lockFree=true for Lock-Free, false for RWMutex)
func NewConsistentEngineWithType(lockFree bool) *ConsistentEngine {
	return &ConsistentEngine{
		engine:      NewMementoEngineWithType(0, lockFree),
		indirection: NewIndirection(0),
	}
}

// GetBucket returns the bucket for a key.
// It ensures the returned bucket exists in the indirection.
// If the bucket returned by MementoEngine doesn't exist in the indirection,
// it follows the replacement chain until finding a valid bucket.
//
// NOTE: This method is NOT thread-safe. The caller must hold an appropriate lock
// (typically MementoSelection.mu with RLock() for reads or Lock() for writes).
func (ce *ConsistentEngine) GetBucket(key string) int {
	bucket := ce.engine.GetBucket(key)

	// Verify that the bucket exists in the indirection
	if ce.indirection.HasBucket(bucket) {
		return bucket
	}

	// The bucket doesn't exist in indirection - this can happen when
	// multiple removals cause remapping chains where the final bucket
	// was also removed from the topology.
	//
	// According to the Java implementation, when indirection.get(bucket)
	// fails (bucket doesn't exist), it throws an exception. But in our
	// case, we need to handle this gracefully.
	//
	// Since MementoEngine.GetBucket already handles remapping correctly,
	// if we get a bucket that doesn't exist in indirection, it means
	// there's a synchronization issue. We should not reach this point
	// in normal operation, but we handle it by finding a valid bucket
	// deterministically based on the key to maintain consistency.
	validBuckets := ce.indirection.GetAllBuckets()
	if len(validBuckets) == 0 {
		// No buckets in indirection - return the original bucket
		// This will cause GetNodeID to return an error, triggering fallback
		return bucket
	}

	// Sort buckets to ensure deterministic ordering
	// This is critical for consistency: same key -> same bucket
	sort.Ints(validBuckets)

	// Use a deterministic hash of the key to select a valid bucket
	// This ensures consistency: same key -> same bucket (from valid buckets)
	hash := hashString(key)
	selectedIndex := int(hash % uint64(len(validBuckets)))
	return validBuckets[selectedIndex]
}

// hashString computes a simple hash of a string
// This is used for deterministic bucket selection when the original bucket
// doesn't exist in the indirection
func hashString(s string) uint64 {
	var hash uint64 = 5381
	for i := 0; i < len(s); i++ {
		hash = ((hash << 5) + hash) + uint64(s[i])
	}
	return hash
}

// AddNode adds a new node to the topology.
//
// NOTE: This method is NOT thread-safe. The caller must hold an appropriate lock
// (typically MementoSelection.mu with Lock() for writes).
func (ce *ConsistentEngine) AddNode(nodeID string) error {
	// Check if node already exists
	if ce.indirection.HasNode(nodeID) {
		return nil // Node already present
	}

	// Add to MementoEngine first
	bucket := ce.engine.AddBucket()

	// Map node ID to bucket index using indirection
	// If mapping fails, we need to rollback the bucket addition
	if err := ce.indirection.Put(nodeID, bucket); err != nil {
		// Rollback: remove the bucket from engine
		ce.engine.RemoveBucket(bucket)
		return fmt.Errorf("failed to add node %s: %w", nodeID, err)
	}

	return nil
}

// RemoveNode removes a node from the topology.
//
// NOTE: This method is NOT thread-safe. The caller must hold an appropriate lock
// (typically MementoSelection.mu with Lock() for writes).
func (ce *ConsistentEngine) RemoveNode(nodeID string) error {
	// Get the bucket for this node from indirection
	bucket, err := ce.indirection.GetBucket(nodeID)
	if err != nil {
		return err // Node not found
	}

	// Remove from indirection first (following Java implementation order)
	// In Java: indirection.remove(node) returns the bucket, then engine.removeBucket(bucket)
	if _, err := ce.indirection.RemoveNode(nodeID); err != nil {
		return fmt.Errorf("failed to remove node %s from indirection: %w", nodeID, err)
	}

	// Remove from MementoEngine
	ce.engine.RemoveBucket(bucket)

	return nil
}

// RestoreNode restores a previously removed node
func (ce *ConsistentEngine) RestoreNode(nodeID string) {
	// AddNode already handles restoring previously removed nodes
	// because MementoEngine tracks the last removed bucket
	ce.AddNode(nodeID)
}

// GetTopology returns the current topology (list of node IDs).
//
// NOTE: This method is NOT thread-safe. The caller must hold an appropriate lock
// (typically MementoSelection.mu with RLock() for reads).
func (ce *ConsistentEngine) GetTopology() []string {
	return ce.indirection.GetAllNodeIDs()
}

// Size returns the current size of the working set.
//
// NOTE: This method is NOT thread-safe. The caller must hold an appropriate lock
// (typically MementoSelection.mu with RLock() for reads).
func (ce *ConsistentEngine) Size() int {
	return ce.engine.Size()
}

// GetMementoStats returns statistics about the engine.
//
// NOTE: This method is NOT thread-safe. The caller must hold an appropriate lock
// (typically MementoSelection.mu with RLock() for reads).
func (ce *ConsistentEngine) GetMementoStats() map[string]interface{} {
	return map[string]interface{}{
		"engine_size":   ce.engine.Size(),
		"binomial_size": ce.engine.binomialArraySize(),
		"memento_size":  ce.engine.memento.Size(),
		"memento_empty": ce.engine.memento.IsEmpty(),
		"last_removed":  ce.engine.lastRemoved,
		"topology_size": ce.indirection.Size(),
	}
}

// String returns a string representation of the consistent engine.
//
// NOTE: This method is NOT thread-safe. The caller must hold an appropriate lock
// (typically MementoSelection.mu with RLock() for reads).
func (ce *ConsistentEngine) String() string {
	return fmt.Sprintf("ConsistentEngine{engine=%s, topology_size=%d}",
		ce.engine.String(), ce.indirection.Size())
}

// GetNodeID returns the node ID for a given bucket index.
//
// NOTE: This method is NOT thread-safe. The caller must hold an appropriate lock
// (typically MementoSelection.mu with RLock() for reads).
func (ce *ConsistentEngine) GetNodeID(bucket int) string {
	nodeID, err := ce.indirection.GetNodeID(bucket)
	if err != nil {
		return "" // Return empty string if bucket doesn't exist
	}
	return nodeID
}
