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

// WeightedIndirection represents a many-to-one mapping between buckets (int)
// and an upstream, where the number of buckets per upstream is determined by its weight.
//
// This structure is responsible for all the logic of managing weighted distribution,
// including adding, removing, and re-weighting upstreams and their associated buckets.
//
// It is NOT thread-safe by itself and must be protected by an external lock.
type WeightedIndirection struct {
	// bucketOwner maps each bucket ID to its owning upstream.
	bucketOwner map[int]*Upstream // bucketId -> upstream

	// nodeBuckets maps each upstream to a list of bucket IDs it owns.
	nodeBuckets map[*Upstream][]int // upstream -> [bucketId, ...]

	// nodeBucketPos maps each bucket ID to its index within the upstream's bucket list.
	// This allows for O(1) removal using the swap-and-pop technique.
	nodeBucketPos map[int]int // bucketId -> index in nodeBuckets

	// weight stores the configured weight for each upstream.
	weight map[*Upstream]int // upstream -> configured weight
}

// NewWeightedIndirection creates a new, empty weighted indirection mapping.
func NewWeightedIndirection() *WeightedIndirection {
	return &WeightedIndirection{
		bucketOwner:   make(map[int]*Upstream),
		nodeBuckets:   make(map[*Upstream][]int),
		nodeBucketPos: make(map[int]int),
		weight:        make(map[*Upstream]int),
	}
}

// AttachBucket assigns a bucket to an upstream, updating all internal mappings.
func (w *WeightedIndirection) AttachBucket(bucketID int, upstream *Upstream) {
	w.bucketOwner[bucketID] = upstream
	w.nodeBucketPos[bucketID] = len(w.nodeBuckets[upstream])
	w.nodeBuckets[upstream] = append(w.nodeBuckets[upstream], bucketID)
}

// DetachBucket removes a bucket's assignment from a node in O(1).
// It uses the swap-and-pop technique to avoid re-slicing the bucket list.
func (w *WeightedIndirection) DetachBucket(bucketID int) {
	upstream := w.bucketOwner[bucketID]
	idx := w.nodeBucketPos[bucketID]

	// Swap-and-pop: move the last bucket into the position of the one being removed.
	buckets := w.nodeBuckets[upstream]
	lastBucket := buckets[len(buckets)-1]
	buckets[idx] = lastBucket
	w.nodeBucketPos[lastBucket] = idx

	// Shrink the slice.
	w.nodeBuckets[upstream] = buckets[:len(buckets)-1]

	// Clean up the mappings for the removed bucket.
	delete(w.bucketOwner, bucketID)
	delete(w.nodeBucketPos, bucketID)
}

// GetNodeID returns the upstream that owns the given bucket.
func (w *WeightedIndirection) GetNodeID(bucketID int) (*Upstream, bool) {
	upstream, ok := w.bucketOwner[bucketID]
	return upstream, ok
}

// GetBucketsForNode returns a copy of the list of buckets owned by an upstream.
func (w *WeightedIndirection) GetBucketsForNode(upstream *Upstream) []int {
	// Return a copy to prevent external modification.
	return append([]int{}, w.nodeBuckets[upstream]...)
}

// GetWeight returns the weight of a given upstream.
func (w *WeightedIndirection) GetWeight(upstream *Upstream) (int, bool) {
	weight, ok := w.weight[upstream]
	return weight, ok
}

// HasNode checks if an upstream exists in the indirection layer.
func (w *WeightedIndirection) HasNode(upstream *Upstream) bool {
	_, exists := w.weight[upstream]
	return exists
}

// InitNode initializes the data structures for a new upstream.
func (w *WeightedIndirection) InitNode(upstream *Upstream, weight int) {
	w.weight[upstream] = weight
	w.nodeBuckets[upstream] = make([]int, 0, weight)
}

// RemoveNode completely removes an upstream and all its associated data.
func (w *WeightedIndirection) RemoveNode(upstream *Upstream) {
	// The actual buckets must be removed from the MementoEngine by the caller.
	// This method just cleans up the indirection's internal state.
	bucketsToRemove := w.nodeBuckets[upstream]
	for _, bucketID := range bucketsToRemove {
		delete(w.bucketOwner, bucketID)
		delete(w.nodeBucketPos, bucketID)
	}
	delete(w.nodeBuckets, upstream)
	delete(w.weight, upstream)
}

// UpdateWeight updates the stored weight for an upstream.
func (w *WeightedIndirection) UpdateWeight(upstream *Upstream, newWeight int) {
	w.weight[upstream] = newWeight
}
