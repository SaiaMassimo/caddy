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

package reverseproxy

import (
	"fmt"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy/memento"
)

// ConsistentEngine wraps MementoEngine to provide a load balancing policy
// that can handle arbitrary node removals and additions.
// It manages node topology (node IDs as strings) and delegates
// all hashing logic to MementoEngine.
type ConsistentEngine struct {
	// MementoEngine handles all hashing logic and arbitrary node removal
	engine *memento.MementoEngine

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
		engine:      memento.NewMementoEngine(0),
		indirection: NewIndirection(0),
	}
}

// NewConsistentEngineWithType creates a new consistent engine with MementoEngine
// using the specified implementation type (lockFree=true for Lock-Free, false for RWMutex)
func NewConsistentEngineWithType(lockFree bool) *ConsistentEngine {
	return &ConsistentEngine{
		engine:      memento.NewMementoEngineWithType(0, lockFree),
		indirection: NewIndirection(0),
	}
}

// GetBucket returns the bucket index for a given key.
func (ce *ConsistentEngine) GetBucket(key string) *Upstream {
	bucket := ce.engine.GetBucket(key)

	upstream, err := ce.indirection.GetNodeID(bucket)
	if err == nil {
		return upstream
	}

	return nil
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

func (ce *ConsistentEngine) AddNode(upstream *Upstream) error {
	// Check if node already exists
	if ce.indirection.HasNode(upstream) {
		return nil // Node already present
	}

	// Add to MementoEngine first
	bucket := ce.engine.AddBucket()

	// Map node ID to bucket index using indirection
	// If mapping fails, we need to rollback the bucket addition
	if err := ce.indirection.Put(upstream, bucket); err != nil {
		// Rollback: remove the bucket from engine
		ce.engine.RemoveBucket(bucket)
		return fmt.Errorf("failed to add node %s: %w", upstream.String(), err)
	}

	return nil
}

// RemoveNode removes a node from the topology.
//
// NOTE: This method is NOT thread-safe. The caller must hold an appropriate lock
// (typically MementoSelection.mu with Lock() for writes).

func (ce *ConsistentEngine) RemoveNode(upstream *Upstream) error {
	// Get the bucket for this node from indirection
	bucket, err := ce.indirection.GetBucket(upstream)
	if err != nil {
		return err // Node not found
	}

	// Remove from indirection first (following Java implementation order)
	// In Java: indirection.remove(node) returns the bucket, then engine.removeBucket(bucket)
	if _, err := ce.indirection.RemoveNode(upstream); err != nil {
		return fmt.Errorf("failed to remove node %s from indirection: %w", upstream.String(), err)
	}

	// Remove from MementoEngine
	ce.engine.RemoveBucket(bucket)

	return nil
}

// RestoreNode restores a previously removed node
func (ce *ConsistentEngine) RestoreNode(upstream *Upstream) {
	// AddNode already handles restoring previously removed nodes
	// because MementoEngine tracks the last removed bucket
	ce.AddNode(upstream)
}

// GetTopology returns the current topology (list of node IDs).
//
// NOTE: This method is NOT thread-safe. The caller must hold an appropriate lock
// (typically MementoSelection.mu with RLock() for reads).
func (ce *ConsistentEngine) GetTopology() []*Upstream {
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
		"binomial_size": ce.engine.BinomialArraySize(),
		"memento_size":  ce.engine.MementoSize(),
		"memento_empty": ce.engine.MementoEmpty(),
		"last_removed":  ce.engine.LastRemoved(),
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

func (ce *ConsistentEngine) GetNodeID(bucket int) *Upstream {
	upstream, err := ce.indirection.GetNodeID(bucket)
	if err != nil {
		return nil // Return nil if bucket doesn't exist
	}
	return upstream
}
