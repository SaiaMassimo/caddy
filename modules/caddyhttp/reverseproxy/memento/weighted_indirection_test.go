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
	"reflect"
	"sort"
	"testing"
)

func TestWeightedIndirection_AttachAndGet(t *testing.T) {
	wi := NewWeightedIndirection()
	wi.InitNode("node1", 2)

	wi.AttachBucket(10, "node1")
	wi.AttachBucket(20, "node1")

	if node, ok := wi.GetNodeID(10); !ok || node != "node1" {
		t.Errorf("Expected bucket 10 to be on node1, got %s (found: %v)", node, ok)
	}
	if node, ok := wi.GetNodeID(20); !ok || node != "node1" {
		t.Errorf("Expected bucket 20 to be on node1, got %s (found: %v)", node, ok)
	}

	buckets := wi.GetBucketsForNode("node1")
	sort.Ints(buckets)
	expectedBuckets := []int{10, 20}
	if !reflect.DeepEqual(buckets, expectedBuckets) {
		t.Errorf("Expected buckets %v for node1, got %v", expectedBuckets, buckets)
	}
}

func TestWeightedIndirection_DetachBucket(t *testing.T) {
	wi := NewWeightedIndirection()
	wi.InitNode("node1", 3)
	wi.AttachBucket(10, "node1")
	wi.AttachBucket(20, "node1")
	wi.AttachBucket(30, "node1")

	// Detach the middle bucket
	wi.DetachBucket(20)

	if _, ok := wi.GetNodeID(20); ok {
		t.Errorf("Expected bucket 20 to be detached, but it still exists")
	}

	buckets := wi.GetBucketsForNode("node1")
	sort.Ints(buckets)
	expectedBuckets := []int{10, 30}
	if !reflect.DeepEqual(buckets, expectedBuckets) {
		t.Errorf("Expected buckets %v after detaching, got %v", expectedBuckets, buckets)
	}

	// Check if swap-and-pop worked correctly
	if node, ok := wi.GetNodeID(30); !ok || node != "node1" {
		t.Errorf("Expected bucket 30 to still be on node1 after swap, but it was not")
	}
}

func TestWeightedIndirection_RemoveNode(t *testing.T) {
	wi := NewWeightedIndirection()
	wi.InitNode("node1", 2)
	wi.AttachBucket(10, "node1")
	wi.AttachBucket(20, "node1")

	wi.RemoveNode("node1")

	if wi.HasNode("node1") {
		t.Errorf("Expected node1 to be removed, but it still exists")
	}
	if _, ok := wi.GetNodeID(10); ok {
		t.Errorf("Expected bucket 10's mapping to be removed, but it still exists")
	}
	if _, ok := wi.GetNodeID(20); ok {
		t.Errorf("Expected bucket 20's mapping to be removed, but it still exists")
	}
	if len(wi.GetBucketsForNode("node1")) != 0 {
		t.Errorf("Expected no buckets for removed node1")
	}
}

func TestWeightedIndirection_UpdateWeight(t *testing.T) {
	wi := NewWeightedIndirection()
	wi.InitNode("node1", 5)

	wi.UpdateWeight("node1", 10)
	if w, _ := wi.GetWeight("node1"); w != 10 {
		t.Errorf("Expected weight to be updated to 10, got %d", w)
	}
}
