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
)

// Indirection represents a one-to-one mapping between a node ID (string)
// and a bucket (int). This structure ensures that there is only
// one node per bucket and vice versa.
//
// This class performs all the consistency checks:
// - The node ID cannot be empty
// - The bucket must be non-negative
// - Two nodes cannot be mapped to the same bucket
// - Two buckets cannot be mapped to the same node
//
// Thread-safe: Uses sync.Map for concurrent access without external locks.
type Indirection struct {
	// Maps each node ID to the related bucket (thread-safe)
	nodeToBucket sync.Map // map[string]int

	// Maps each bucket to the related node ID (thread-safe)
	bucketToNode sync.Map // map[int]string
}

// NewIndirection creates a new indirection with the given initial capacity
// Note: initialCapacity is ignored when using sync.Map (it doesn't support pre-allocation)
func NewIndirection(initialCapacity int) *Indirection {
	return &Indirection{
		nodeToBucket: sync.Map{},
		bucketToNode: sync.Map{},
	}
}

// Put adds a new mapping between the given node ID and bucket.
// It performs consistency checks:
// - The node ID cannot be empty
// - The bucket must be non-negative
// - Two nodes cannot be mapped to the same bucket
// - Two buckets cannot be mapped to the same node
func (ind *Indirection) Put(nodeID string, bucket int) error {
	if nodeID == "" {
		return fmt.Errorf("node ID cannot be empty")
	}
	if bucket < 0 {
		return fmt.Errorf("bucket must be non-negative, got %d", bucket)
	}

	// Check for duplicate node
	if existingBucket, exists := ind.nodeToBucket.Load(nodeID); exists {
		return fmt.Errorf("duplicated node %s (already mapped to bucket %d)", nodeID, existingBucket)
	}

	// Check for duplicate bucket
	if existingNode, exists := ind.bucketToNode.Load(bucket); exists {
		return fmt.Errorf("duplicated bucket %d (already mapped to node %s)", bucket, existingNode)
	}

	// Store both mappings atomically
	ind.nodeToBucket.Store(nodeID, bucket)
	ind.bucketToNode.Store(bucket, nodeID)
	return nil
}

// GetBucket returns the bucket mapped to the given node ID.
// Returns an error if the node ID is empty or the mapping does not exist.
func (ind *Indirection) GetBucket(nodeID string) (int, error) {
	if nodeID == "" {
		return -1, fmt.Errorf("node ID cannot be empty")
	}

	bucket, exists := ind.nodeToBucket.Load(nodeID)
	if !exists {
		return -1, fmt.Errorf("node %s is not mapped to any bucket", nodeID)
	}

	return bucket.(int), nil
}

// GetNodeID returns the node ID mapped to the given bucket.
// Returns an error if the bucket is negative or the mapping does not exist.
func (ind *Indirection) GetNodeID(bucket int) (string, error) {
	if bucket < 0 {
		return "", fmt.Errorf("bucket must be non-negative, got %d", bucket)
	}

	nodeID, exists := ind.bucketToNode.Load(bucket)
	if !exists {
		return "", fmt.Errorf("bucket %d is not mapped to any node", bucket)
	}

	return nodeID.(string), nil
}

// HasBucket checks if the given bucket exists in the indirection
func (ind *Indirection) HasBucket(bucket int) bool {
	_, exists := ind.bucketToNode.Load(bucket)
	return exists
}

// HasNode checks if the given node ID exists in the indirection
func (ind *Indirection) HasNode(nodeID string) bool {
	_, exists := ind.nodeToBucket.Load(nodeID)
	return exists
}

// RemoveNode removes the mapping related to the given node ID.
// Returns the bucket that was mapped to the node, or an error if the node doesn't exist.
func (ind *Indirection) RemoveNode(nodeID string) (int, error) {
	bucket, err := ind.GetBucket(nodeID)
	if err != nil {
		return -1, err
	}

	// Remove both mappings atomically
	ind.nodeToBucket.Delete(nodeID)
	ind.bucketToNode.Delete(bucket)
	return bucket, nil
}

// RemoveBucket removes the mapping related to the given bucket.
// Returns the node ID that was mapped to the bucket, or an error if the bucket doesn't exist.
func (ind *Indirection) RemoveBucket(bucket int) (string, error) {
	nodeID, err := ind.GetNodeID(bucket)
	if err != nil {
		return "", err
	}

	// Remove both mappings atomically
	ind.nodeToBucket.Delete(nodeID)
	ind.bucketToNode.Delete(bucket)
	return nodeID, nil
}

// Size returns the number of mappings in the indirection
// Note: This is approximate for sync.Map (it may not be exact under concurrent modifications)
func (ind *Indirection) Size() int {
	count := 0
	ind.nodeToBucket.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// GetAllBuckets returns all buckets currently in the indirection
func (ind *Indirection) GetAllBuckets() []int {
	buckets := make([]int, 0)
	ind.bucketToNode.Range(func(key, _ interface{}) bool {
		buckets = append(buckets, key.(int))
		return true
	})
	return buckets
}

// GetAllNodeIDs returns all node IDs currently in the indirection
func (ind *Indirection) GetAllNodeIDs() []string {
	nodeIDs := make([]string, 0)
	ind.nodeToBucket.Range(func(key, _ interface{}) bool {
		nodeIDs = append(nodeIDs, key.(string))
		return true
	})
	return nodeIDs
}
