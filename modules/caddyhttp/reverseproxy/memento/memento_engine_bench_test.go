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
	"math/rand"
	"testing"
)

// BenchmarkRemoveBucket measures the performance of RemoveBucket operations
func BenchmarkRemoveBucket(b *testing.B) {
	const initialNodes = 1000

	b.Run("RemoveBucket_Sequential", func(b *testing.B) {
		engine := NewMementoEngine(initialNodes)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			size := engine.Size()
			if size <= 0 {
				break
			}
			// Remove a random bucket
			bucket := rand.Intn(size)
			engine.RemoveBucket(bucket)
		}
	})

	b.Run("RemoveBucket_WithLookups", func(b *testing.B) {
		engine := NewMementoEngine(initialNodes)
		const numKeys = 1000
		keys := make([]string, numKeys)
		for i := 0; i < numKeys; i++ {
			keys[i] = string(rune(i))
		}

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			size := engine.Size()
			if size <= 0 {
				break
			}
			// Remove a random bucket
			bucket := rand.Intn(size)
			engine.RemoveBucket(bucket)

			// Perform some lookups after removal
			for j := 0; j < 10; j++ {
				key := keys[rand.Intn(numKeys)]
				_ = engine.GetBucket(key)
			}
		}
	})
}

// BenchmarkRemoveNode measures the performance of RemoveNode in ConsistentEngine
func BenchmarkRemoveNode(b *testing.B) {
	const initialNodes = 1000

	b.Run("RemoveNode_ConsistentEngine", func(b *testing.B) {
		engine := NewConsistentEngine()

		// Populate with nodes
		for i := 0; i < initialNodes; i++ {
			engine.AddNode(string(rune(i)))
		}

		nodeIDs := engine.GetTopology()
		b.ResetTimer()

		for i := 0; i < b.N && i < len(nodeIDs); i++ {
			engine.RemoveNode(nodeIDs[i])
		}
	})

	b.Run("RemoveNode_WithLookups", func(b *testing.B) {
		engine := NewConsistentEngine()

		// Populate with nodes
		for i := 0; i < initialNodes; i++ {
			engine.AddNode(string(rune(i)))
		}

		const numKeys = 1000
		keys := make([]string, numKeys)
		for i := 0; i < numKeys; i++ {
			keys[i] = string(rune(i + 10000))
		}

		nodeIDs := engine.GetTopology()
		b.ResetTimer()

		for i := 0; i < b.N && i < len(nodeIDs); i++ {
			engine.RemoveNode(nodeIDs[i])

			// Perform some lookups after removal
			for j := 0; j < 10; j++ {
				key := keys[rand.Intn(numKeys)]
				_ = engine.GetBucket(key)
			}
		}
	})
}

// BenchmarkMementoSizeAccess measures the overhead of Size() calls
func BenchmarkMementoSizeAccess(b *testing.B) {
	memento := NewMemento()

	// Populate with some removals
	for i := 0; i < 100; i++ {
		memento.Remember(i, 100-i, -1)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = memento.Size()
	}
}
