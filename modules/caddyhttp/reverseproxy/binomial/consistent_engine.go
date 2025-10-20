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

// ConsistentEngine wraps any hashing engine with Memento to provide
// consistent hashing that is stable against random node removals.
type ConsistentEngine struct {
	// The underlying hashing engine (e.g., BinomialEngine)
	engine HashingEngine
	
	// Memento for tracking removed nodes and their replacements
	memento *Memento
	
	// Current topology state
	topology []string
	lastRemoved int
	
	// Thread safety
	mu sync.RWMutex
}

// HashingEngine defines the interface for any hashing engine
type HashingEngine interface {
	GetBucket(key string) int
	Size() int
	AddBucket() int
	RemoveBucket(bucket int) int
}

// NewConsistentEngine creates a new consistent engine wrapping the given engine
func NewConsistentEngine(engine HashingEngine) *ConsistentEngine {
	return &ConsistentEngine{
		engine:        engine,
		memento:       NewMemento(),
		topology:      make([]string, 0, engine.Size()),
		lastRemoved:   -1,
	}
}

// GetBucket returns the bucket for a key, using memento to handle removed nodes
func (ce *ConsistentEngine) GetBucket(key string) int {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	
	// Get the bucket from the underlying engine
	bucket := ce.engine.GetBucket(key)
	
	// Check if this bucket was removed and has a replacer
	replacer := ce.memento.Replacer(bucket)
	if replacer != -1 {
		return replacer
	}
	
	return bucket
}

// AddNode adds a new node to the topology
func (ce *ConsistentEngine) AddNode(nodeID string) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	
	// Add to underlying engine
	ce.engine.AddBucket()
	
	// Add to topology
	ce.topology = append(ce.topology, nodeID)
}

// RemoveNode removes a node from the topology
func (ce *ConsistentEngine) RemoveNode(nodeID string) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	
	// Find the node in topology
	nodeIndex := -1
	for i, node := range ce.topology {
		if node == nodeID {
			nodeIndex = i
			break
		}
	}
	
	if nodeIndex == -1 {
		return // Node not found
	}
	
	// Remove from underlying engine
	newSize := ce.engine.RemoveBucket(nodeIndex)
	
	// Remember the removal in memento
	// The replacer is the new size (which represents the last bucket)
	ce.lastRemoved = ce.memento.Remember(nodeIndex, newSize, ce.lastRemoved)
	
	// Remove from topology
	ce.topology = append(ce.topology[:nodeIndex], ce.topology[nodeIndex+1:]...)
}

// RestoreNode restores a previously removed node
func (ce *ConsistentEngine) RestoreNode(nodeID string) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	
	// Find the node in topology to determine its index
	nodeIndex := -1
	for i, node := range ce.topology {
		if node == nodeID {
			nodeIndex = i
			break
		}
	}
	
	if nodeIndex == -1 {
		// Node not found in current topology, add it at the end
		ce.engine.AddBucket()
		ce.topology = append(ce.topology, nodeID)
		return
	}
	
	// Restore from memento
	ce.lastRemoved = ce.memento.Restore(nodeIndex)
	
	// Add back to underlying engine
	ce.engine.AddBucket()
	
	// Add back to topology (insert at original position)
	ce.topology = append(ce.topology[:nodeIndex], 
		append([]string{nodeID}, ce.topology[nodeIndex:]...)...)
}

// GetTopology returns the current topology
func (ce *ConsistentEngine) GetTopology() []string {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	
	result := make([]string, len(ce.topology))
	copy(result, ce.topology)
	return result
}

// Size returns the current size of the working set
func (ce *ConsistentEngine) Size() int {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return ce.engine.Size()
}

// GetMementoStats returns statistics about the memento
func (ce *ConsistentEngine) GetMementoStats() map[string]interface{} {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	
	return map[string]interface{}{
		"memento_size":     ce.memento.Size(),
		"memento_capacity": ce.memento.capacity(),
		"memento_empty":    ce.memento.IsEmpty(),
		"topology_size":    len(ce.topology),
		"last_removed":     ce.lastRemoved,
	}
}

// String returns a string representation of the consistent engine
func (ce *ConsistentEngine) String() string {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	
	return fmt.Sprintf("ConsistentEngine{engine_size=%d, topology_size=%d, memento=%s}", 
		ce.engine.Size(), len(ce.topology), ce.memento.String())
}
