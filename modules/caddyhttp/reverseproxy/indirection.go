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
	"sync"
)

// Indirection represents a one-to-one mapping between an upstream
// and a bucket (int). This structure ensures that there is only
// one upstream per bucket and vice versa.
//
// This class performs all the consistency checks:
// - The upstream cannot be nil
// - The bucket must be non-negative
// - Two upstreams cannot be mapped to the same bucket
// - Two buckets cannot be mapped to the same upstream
//
// Thread-safe: Uses sync.Map for concurrent access without external locks.
type Indirection struct {
	// Maps each upstream to the related bucket (thread-safe)
	upstreamToBucket sync.Map // map[*Upstream]int

	// Maps each bucket to the related upstream (thread-safe)
	bucketToUpstream sync.Map // map[int]*Upstream
}

// NewIndirection creates a new indirection with the given initial capacity
// Note: initialCapacity is ignored when using sync.Map (it doesn't support pre-allocation)
func NewIndirection(initialCapacity int) *Indirection {
	return &Indirection{
		upstreamToBucket: sync.Map{},
		bucketToUpstream: sync.Map{},
	}
}

// Put adds a new mapping between the given upstream and bucket.
// It performs consistency checks:
// - The upstream cannot be nil
// - The bucket must be non-negative
// - Two upstreams cannot be mapped to the same bucket
// - Two buckets cannot be mapped to the same upstream
func (ind *Indirection) Put(upstream *Upstream, bucket int) error {
	if upstream == nil {
		return fmt.Errorf("upstream cannot be nil")
	}
	if bucket < 0 {
		return fmt.Errorf("bucket must be non-negative, got %d", bucket)
	}

	// Check for duplicate upstream
	if existingBucket, exists := ind.upstreamToBucket.Load(upstream); exists {
		return fmt.Errorf("duplicated upstream %s (already mapped to bucket %d)", upstream.String(), existingBucket)
	}

	// Check for duplicate bucket
	if existingUpstream, exists := ind.bucketToUpstream.Load(bucket); exists {
		return fmt.Errorf("duplicated bucket %d (already mapped to upstream %s)", bucket, existingUpstream.(*Upstream).String())
	}

	// Store both mappings atomically
	ind.upstreamToBucket.Store(upstream, bucket)
	ind.bucketToUpstream.Store(bucket, upstream)
	return nil
}

// GetBucket returns the bucket mapped to the given upstream.
// Returns an error if the upstream is nil or the mapping does not exist.
func (ind *Indirection) GetBucket(upstream *Upstream) (int, error) {
	if upstream == nil {
		return -1, fmt.Errorf("upstream cannot be nil")
	}

	bucket, exists := ind.upstreamToBucket.Load(upstream)
	if !exists {
		return -1, fmt.Errorf("upstream %s is not mapped to any bucket", upstream.String())
	}

	return bucket.(int), nil
}

// GetNodeID returns the upstream mapped to the given bucket.
// Returns an error if the bucket is negative or the mapping does not exist.
func (ind *Indirection) GetNodeID(bucket int) (*Upstream, error) {
	if bucket < 0 {
		return nil, fmt.Errorf("bucket must be non-negative, got %d", bucket)
	}

	upstream, exists := ind.bucketToUpstream.Load(bucket)
	if !exists {
		return nil, fmt.Errorf("bucket %d is not mapped to any upstream", bucket)
	}

	return upstream.(*Upstream), nil
}

// HasBucket checks if the given bucket exists in the indirection
func (ind *Indirection) HasBucket(bucket int) bool {
	_, exists := ind.bucketToUpstream.Load(bucket)
	return exists
}

// HasNode checks if the given upstream exists in the indirection
func (ind *Indirection) HasNode(upstream *Upstream) bool {
	_, exists := ind.upstreamToBucket.Load(upstream)
	return exists
}

// RemoveNode removes the mapping related to the given upstream.
// Returns the bucket that was mapped to the upstream, or an error if the upstream doesn't exist.
func (ind *Indirection) RemoveNode(upstream *Upstream) (int, error) {
	bucket, err := ind.GetBucket(upstream)
	if err != nil {
		return -1, err
	}

	// Remove both mappings atomically
	ind.upstreamToBucket.Delete(upstream)
	ind.bucketToUpstream.Delete(bucket)
	return bucket, nil
}

// RemoveBucket removes the mapping related to the given bucket.
// Returns the upstream that was mapped to the bucket, or an error if the bucket doesn't exist.
func (ind *Indirection) RemoveBucket(bucket int) (*Upstream, error) {
	upstream, err := ind.GetNodeID(bucket)
	if err != nil {
		return nil, err
	}

	// Remove both mappings atomically
	ind.upstreamToBucket.Delete(upstream)
	ind.bucketToUpstream.Delete(bucket)
	return upstream, nil
}

// Size returns the number of mappings in the indirection
// Note: This is approximate for sync.Map (it may not be exact under concurrent modifications)
func (ind *Indirection) Size() int {
	count := 0
	ind.upstreamToBucket.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// GetAllBuckets returns all buckets currently in the indirection
func (ind *Indirection) GetAllBuckets() []int {
	buckets := make([]int, 0)
	ind.bucketToUpstream.Range(func(key, _ interface{}) bool {
		buckets = append(buckets, key.(int))
		return true
	})
	return buckets
}

// GetAllNodeIDs returns all upstreams currently in the indirection
func (ind *Indirection) GetAllNodeIDs() []*Upstream {
	upstreams := make([]*Upstream, 0)
	ind.upstreamToBucket.Range(func(key, _ interface{}) bool {
		upstreams = append(upstreams, key.(*Upstream))
		return true
	})
	return upstreams
}
