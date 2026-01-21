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

// WeightedIndirection represents a many-to-one mapping between buckets (int)
// and a node ID (string), where the number of buckets per node is determined by its weight.
//
// This structure is responsible for all the logic of managing weighted distribution,
// including adding, removing, and re-weighting nodes and their associated buckets.
//
// It is NOT thread-safe by itself and must be protected by an external lock.
type WeightedIndirection struct {
	// bucketOwner maps each bucket ID to its owning physical node ID.
	bucketOwner map[int]string // bucketId -> physicalNodeId

	// nodeBuckets maps each physical node ID to a list of bucket IDs it owns.
	nodeBuckets map[string][]int // physicalNodeId -> [bucketId, ...]

	// nodeBucketPos maps each bucket ID to its index within the node's bucket list.
	// This allows for O(1) removal using the swap-and-pop technique.
	nodeBucketPos map[int]int // bucketId -> index in nodeBuckets

	// weight stores the configured weight for each node.
	weight map[string]int // physicalNodeId -> configured weight
}

// NewWeightedIndirection creates a new, empty weighted indirection mapping.
func NewWeightedIndirection() *WeightedIndirection {
	return &WeightedIndirection{
		bucketOwner:   make(map[int]string),
		nodeBuckets:   make(map[string][]int),
		nodeBucketPos: make(map[int]int),
		weight:        make(map[string]int),
	}
}

// AttachBucket assigns a bucket to a node, updating all internal mappings.
func (w *WeightedIndirection) AttachBucket(bucketID int, nodeID string) {
	w.bucketOwner[bucketID] = nodeID
	w.nodeBucketPos[bucketID] = len(w.nodeBuckets[nodeID])
	w.nodeBuckets[nodeID] = append(w.nodeBuckets[nodeID], bucketID)
}

// DetachBucket removes a bucket's assignment from a node in O(1).
// It uses the swap-and-pop technique to avoid re-slicing the bucket list.
func (w *WeightedIndirection) DetachBucket(bucketID int) {
	nodeID := w.bucketOwner[bucketID]
	idx := w.nodeBucketPos[bucketID]

	// Swap-and-pop: move the last bucket into the position of the one being removed.
	buckets := w.nodeBuckets[nodeID]
	lastBucket := buckets[len(buckets)-1]
	buckets[idx] = lastBucket
	w.nodeBucketPos[lastBucket] = idx

	// Shrink the slice.
	w.nodeBuckets[nodeID] = buckets[:len(buckets)-1]

	// Clean up the mappings for the removed bucket.
	delete(w.bucketOwner, bucketID)
	delete(w.nodeBucketPos, bucketID)
}

// GetNodeID returns the node ID that owns the given bucket.
func (w *WeightedIndirection) GetNodeID(bucketID int) (string, bool) {
	nodeID, ok := w.bucketOwner[bucketID]
	return nodeID, ok
}

// GetBucketsForNode returns a copy of the list of buckets owned by a node.
func (w *WeightedIndirection) GetBucketsForNode(nodeID string) []int {
	// Return a copy to prevent external modification.
	return append([]int{}, w.nodeBuckets[nodeID]...)
}

// GetWeight returns the weight of a given node.
func (w *WeightedIndirection) GetWeight(nodeID string) (int, bool) {
	weight, ok := w.weight[nodeID]
	return weight, ok
}

// HasNode checks if a node exists in the indirection layer.
func (w *WeightedIndirection) HasNode(nodeID string) bool {
	_, exists := w.weight[nodeID]
	return exists
}

// InitNode initializes the data structures for a new node.
func (w *WeightedIndirection) InitNode(nodeID string, weight int) {
	w.weight[nodeID] = weight
	w.nodeBuckets[nodeID] = make([]int, 0, weight)
}

// RemoveNode completely removes a node and all its associated data.
func (w *WeightedIndirection) RemoveNode(nodeID string) {
	// The actual buckets must be removed from the MementoEngine by the caller.
	// This method just cleans up the indirection's internal state.
	bucketsToRemove := w.nodeBuckets[nodeID]
	for _, bucketID := range bucketsToRemove {
		delete(w.bucketOwner, bucketID)
		delete(w.nodeBucketPos, bucketID)
	}
	delete(w.nodeBuckets, nodeID)
	delete(w.weight, nodeID)
}

// UpdateWeight updates the stored weight for a node.
func (w *WeightedIndirection) UpdateWeight(nodeID string, newWeight int) {
	w.weight[nodeID] = newWeight
}
