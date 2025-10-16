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
	"testing"
)

func TestBinomialEngine(t *testing.T) {
	// Test basic functionality
	engine := NewBinomialEngine(5)
	
	if engine.Size() != 5 {
		t.Errorf("Expected size 5, got %d", engine.Size())
	}

	// Test bucket assignment
	key := "test-key-123"
	bucket := engine.GetBucket(key)
	
	if bucket < 0 || bucket >= engine.Size() {
		t.Errorf("Bucket %d is out of range [0, %d)", bucket, engine.Size())
	}

	// Test consistency - same key should always map to same bucket
	bucket2 := engine.GetBucket(key)
	if bucket != bucket2 {
		t.Errorf("Inconsistent bucket assignment: %d vs %d", bucket, bucket2)
	}
}

func TestBinomialEngineSingleNode(t *testing.T) {
	engine := NewBinomialEngine(1)
	
	if engine.Size() != 1 {
		t.Errorf("Expected size 1, got %d", engine.Size())
	}

	// All keys should map to bucket 0
	for i := 0; i < 10; i++ {
		key := "test-key-" + string(rune(i))
		bucket := engine.GetBucket(key)
		if bucket != 0 {
			t.Errorf("Expected bucket 0 for single node, got %d", bucket)
		}
	}
}

func TestBinomialEngineAddBucket(t *testing.T) {
	engine := NewBinomialEngine(3)
	originalSize := engine.Size()

	// Add a bucket
	newBucket := engine.AddBucket()
	
	if engine.Size() != originalSize+1 {
		t.Errorf("Expected size %d, got %d", originalSize+1, engine.Size())
	}

	if newBucket != originalSize {
		t.Errorf("Expected new bucket %d, got %d", originalSize, newBucket)
	}

	// Test that the new bucket is valid
	if newBucket < 0 || newBucket >= engine.Size() {
		t.Errorf("New bucket %d is out of range [0, %d)", newBucket, engine.Size())
	}
}

func TestBinomialEngineRemoveBucket(t *testing.T) {
	engine := NewBinomialEngine(5)
	originalSize := engine.Size()

	// Remove a bucket
	newSize := engine.RemoveBucket(2)
	
	if engine.Size() != originalSize-1 {
		t.Errorf("Expected size %d, got %d", originalSize-1, engine.Size())
	}

	if newSize != originalSize-1 {
		t.Errorf("Expected returned size %d, got %d", originalSize-1, newSize)
	}
}

func TestBinomialEngineDistribution(t *testing.T) {
	engine := NewBinomialEngine(10)
	
	// Test distribution across buckets
	bucketCounts := make(map[int]int)
	numKeys := 1000
	
	for i := 0; i < numKeys; i++ {
		key := "test-key-" + string(rune(i))
		bucket := engine.GetBucket(key)
		bucketCounts[bucket]++
	}

	// Check that all buckets are used
	if len(bucketCounts) != engine.Size() {
		t.Errorf("Expected %d buckets to be used, got %d", engine.Size(), len(bucketCounts))
	}

	// Check distribution is reasonable (not perfectly uniform due to hash function)
	expectedPerBucket := numKeys / engine.Size()
	tolerance := expectedPerBucket / 2 // 50% tolerance
	
	for bucket, count := range bucketCounts {
		if count < expectedPerBucket-tolerance || count > expectedPerBucket+tolerance {
			t.Logf("Bucket %d has %d keys (expected ~%d)", bucket, count, expectedPerBucket)
		}
	}
}

func TestBinomialEngineConsistency(t *testing.T) {
	engine := NewBinomialEngine(8)
	
	// Test that same keys always map to same buckets
	keys := []string{"key1", "key2", "key3", "key4", "key5"}
	originalBuckets := make(map[string]int)
	
	for _, key := range keys {
		originalBuckets[key] = engine.GetBucket(key)
	}
	
	// Test multiple times
	for i := 0; i < 10; i++ {
		for _, key := range keys {
			bucket := engine.GetBucket(key)
			if bucket != originalBuckets[key] {
				t.Errorf("Inconsistent mapping for key %s: %d vs %d", key, bucket, originalBuckets[key])
			}
		}
	}
}

func TestBinomialEngineFilters(t *testing.T) {
	engine := NewBinomialEngine(7)
	
	enclosingFilter := engine.EnclosingTreeFilter()
	minorFilter := engine.MinorTreeFilter()
	
	// Enclosing filter should be larger than minor filter
	if enclosingFilter <= minorFilter {
		t.Errorf("Enclosing filter %d should be larger than minor filter %d", enclosingFilter, minorFilter)
	}
	
	// Filters should be powers of 2 minus 1
	if (enclosingFilter+1)&enclosingFilter != 0 {
		t.Errorf("Enclosing filter %d should be 2^n-1", enclosingFilter)
	}
	
	if (minorFilter+1)&minorFilter != 0 {
		t.Errorf("Minor filter %d should be 2^n-1", minorFilter)
	}
}

func BenchmarkBinomialEngineGetBucket(b *testing.B) {
	engine := NewBinomialEngine(100)
	key := "benchmark-key-12345"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.GetBucket(key)
	}
}

func BenchmarkBinomialEngineGetBucketDifferentKeys(b *testing.B) {
	engine := NewBinomialEngine(100)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "benchmark-key-" + string(rune(i))
		engine.GetBucket(key)
	}
}
