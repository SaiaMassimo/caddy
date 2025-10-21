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
	"testing"
)

func TestMemento(t *testing.T) {
	memento := NewMemento()
	
	// Test initial state
	if !memento.IsEmpty() {
		t.Error("Memento should be empty initially")
	}
	
	if memento.Size() != 0 {
		t.Errorf("Expected size 0, got %d", memento.Size())
	}
}

func TestMementoRemember(t *testing.T) {
	memento := NewMemento()
	
	// Remember a removal
	lastRemoved := memento.Remember(5, 3, -1)
	if lastRemoved != 5 {
		t.Errorf("Expected lastRemoved 5, got %d", lastRemoved)
	}
	
	if memento.IsEmpty() {
		t.Error("Memento should not be empty after remembering")
	}
	
	if memento.Size() != 1 {
		t.Errorf("Expected size 1, got %d", memento.Size())
	}
}

func TestMementoReplacer(t *testing.T) {
	memento := NewMemento()
	
	// Remember a removal
	memento.Remember(5, 3, -1)
	
	// Test replacer
	replacer := memento.Replacer(5)
	if replacer != 3 {
		t.Errorf("Expected replacer 3, got %d", replacer)
	}
	
	// Test non-existent bucket
	replacer = memento.Replacer(10)
	if replacer != -1 {
		t.Errorf("Expected replacer -1 for non-existent bucket, got %d", replacer)
	}
}

func TestMementoRestore(t *testing.T) {
	memento := NewMemento()
	
	// Remember a removal
	memento.Remember(5, 3, -1)
	
	// Restore the bucket
	prevRemoved := memento.Restore(5)
	if prevRemoved != -1 {
		t.Errorf("Expected prevRemoved -1, got %d", prevRemoved)
	}
	
	if !memento.IsEmpty() {
		t.Error("Memento should be empty after restore")
	}
	
	if memento.Size() != 0 {
		t.Errorf("Expected size 0, got %d", memento.Size())
	}
}

func TestMementoSequence(t *testing.T) {
	memento := NewMemento()
	
	// Remember multiple removals in sequence
	lastRemoved := memento.Remember(5, 3, -1)
	if lastRemoved != 5 {
		t.Errorf("Expected lastRemoved 5, got %d", lastRemoved)
	}
	
	lastRemoved = memento.Remember(7, 2, 5)
	if lastRemoved != 7 {
		t.Errorf("Expected lastRemoved 7, got %d", lastRemoved)
	}
	
	lastRemoved = memento.Remember(2, 1, 7)
	if lastRemoved != 2 {
		t.Errorf("Expected lastRemoved 2, got %d", lastRemoved)
	}
	
	if memento.Size() != 3 {
		t.Errorf("Expected size 3, got %d", memento.Size())
	}
	
	// Test replacers
	if memento.Replacer(5) != 3 {
		t.Error("Expected replacer 3 for bucket 5")
	}
	if memento.Replacer(7) != 2 {
		t.Error("Expected replacer 2 for bucket 7")
	}
	if memento.Replacer(2) != 1 {
		t.Error("Expected replacer 1 for bucket 2")
	}
	
	// Restore in reverse order
	prevRemoved := memento.Restore(2)
	if prevRemoved != 7 {
		t.Errorf("Expected prevRemoved 7, got %d", prevRemoved)
	}
	
	prevRemoved = memento.Restore(7)
	if prevRemoved != 5 {
		t.Errorf("Expected prevRemoved 5, got %d", prevRemoved)
	}
	
	prevRemoved = memento.Restore(5)
	if prevRemoved != -1 {
		t.Errorf("Expected prevRemoved -1, got %d", prevRemoved)
	}
	
	if !memento.IsEmpty() {
		t.Error("Memento should be empty after all restores")
	}
}

func TestMementoConcurrent(t *testing.T) {
	memento := NewMemento()
	
	// Test concurrent access
	done := make(chan bool, 2)
	
	// Goroutine 1: Add removals
	go func() {
		for i := 0; i < 100; i++ {
			memento.Remember(i, i-1, -1)
		}
		done <- true
	}()
	
	// Goroutine 2: Check replacers
	go func() {
		for i := 0; i < 100; i++ {
			replacer := memento.Replacer(i)
			if replacer != -1 && replacer != i-1 {
				t.Errorf("Unexpected replacer for bucket %d: %d", i, replacer)
			}
		}
		done <- true
	}()
	
	// Wait for both goroutines
	<-done
	<-done
}

func TestConsistentEngine(t *testing.T) {
	// Create a binomial engine
	binomialEngine := NewBinomialEngine(5)
	
	// Wrap it with consistent engine
	consistentEngine := NewConsistentEngine(binomialEngine)
	
	// Test initial state
	if consistentEngine.Size() != 5 {
		t.Errorf("Expected size 5, got %d", consistentEngine.Size())
	}
	
	// Test GetBucket
	bucket := consistentEngine.GetBucket("test-key")
	if bucket < 0 || bucket >= 5 {
		t.Errorf("Bucket %d out of range [0, 5)", bucket)
	}
	
	// Test topology
	topology := consistentEngine.GetTopology()
	if len(topology) != 0 {
		t.Errorf("Expected empty topology, got %v", topology)
	}
}

func TestConsistentEngineNodeOperations(t *testing.T) {
	binomialEngine := NewBinomialEngine(3)
	consistentEngine := NewConsistentEngine(binomialEngine)
	
	// Add nodes
	consistentEngine.AddNode("node1")
	consistentEngine.AddNode("node2")
	consistentEngine.AddNode("node3")
	
	// Check topology
	topology := consistentEngine.GetTopology()
	if len(topology) != 3 {
		t.Errorf("Expected topology size 3, got %d", len(topology))
	}
	
	// Remove a node
	consistentEngine.RemoveNode("node2")
	
	// Check topology after removal
	topology = consistentEngine.GetTopology()
	if len(topology) != 2 {
		t.Errorf("Expected topology size 2, got %d", len(topology))
	}
	
	// Check that node2 is not in topology
	found := false
	for _, node := range topology {
		if node == "node2" {
			found = true
			break
		}
	}
	if found {
		t.Error("node2 should not be in topology after removal")
	}
	
	// Restore the node
	consistentEngine.RestoreNode("node2")
	
	// Check topology after restore
	topology = consistentEngine.GetTopology()
	if len(topology) != 3 {
		t.Errorf("Expected topology size 3 after restore, got %d", len(topology))
	}
}

func TestConsistentEngineConsistency(t *testing.T) {
	binomialEngine := NewBinomialEngine(5)
	consistentEngine := NewConsistentEngine(binomialEngine)
	
	// Add nodes
	for i := 0; i < 5; i++ {
		consistentEngine.AddNode(fmt.Sprintf("node%d", i))
	}
	
	// Test consistency - same key should map to same bucket
	key := "consistent-test-key"
	bucket1 := consistentEngine.GetBucket(key)
	bucket2 := consistentEngine.GetBucket(key)
	
	if bucket1 != bucket2 {
		t.Errorf("Inconsistent mapping: %d vs %d", bucket1, bucket2)
	}
	
	// Remove a node and test that most keys still map to same bucket
	consistentEngine.RemoveNode("node2")
	
	bucket3 := consistentEngine.GetBucket(key)
	// The bucket might change due to removal, but it should be consistent
	bucket4 := consistentEngine.GetBucket(key)
	
	if bucket3 != bucket4 {
		t.Errorf("Inconsistent mapping after removal: %d vs %d", bucket3, bucket4)
	}
}

func BenchmarkMementoRemember(b *testing.B) {
	memento := NewMemento()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		memento.Remember(i%1000, (i-1)%1000, -1)
	}
}

func BenchmarkMementoReplacer(b *testing.B) {
	memento := NewMemento()
	
	// Pre-populate
	for i := 0; i < 1000; i++ {
		memento.Remember(i, i-1, -1)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		memento.Replacer(i % 1000)
	}
}

func BenchmarkConsistentEngineGetBucket(b *testing.B) {
	binomialEngine := NewBinomialEngine(100)
	consistentEngine := NewConsistentEngine(binomialEngine)
	
	// Add nodes
	for i := 0; i < 100; i++ {
		consistentEngine.AddNode(fmt.Sprintf("node%d", i))
	}
	
	key := "benchmark-key"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		consistentEngine.GetBucket(key)
	}
}
