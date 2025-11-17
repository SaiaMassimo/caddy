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

package reverseproxy

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
)

// BenchmarkMementoVsRendezvous_LookupPerformance measures the average lookup time
// for Memento vs Rendezvous hashing with progressive node removals.
//
// Setup:
// - Initial cluster of 100 nodes
// - Progressive removals: {0, 5, 10, 20, 50}
// - Same list of keys for all runs and both implementations
// - 1,000,000 random keys with fixed seed (1337) for reproducibility
// - Warm-up before measurement (100k lookups)
//
// Metric: Average time per lookup in ns/op
func BenchmarkMementoVsRendezvous_LookupPerformance(b *testing.B) {
	const (
		totalNodes    = 100
		numKeys       = 1000000
		warmupLookups = 100000
		seed          = 1337
	)

	removalCounts := []int{0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 99}

	// Generate fixed set of keys with deterministic seed
	keys := generateKeys(numKeys, seed)

	// Setup context and policies
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	mementoPolicy := MementoSelection{Field: "ip"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}

	rendezvousPolicy := IPHashSelection{}

	// Create the full pool once
	pool := make([]*Upstream, totalNodes)
	for i := 0; i < totalNodes; i++ {
		pool[i] = &Upstream{
			Host: new(Host),
			Dial: fmt.Sprintf("localhost:%d", 8080+i),
		}
		pool[i].setHealthy(true)
	}

	// Benchmark each scenario
	for _, numRemoved := range removalCounts {
		if numRemoved >= totalNodes {
			continue
		}

		// Setup Memento: initialize topology and remove nodes
		mementoPolicy.PopulateInitialTopology(pool)
		for i := 0; i < numRemoved; i++ {
			mementoPolicy.handleUnhealthyEvent(context.Background(), caddy.Event{
				Data: map[string]any{"host": pool[i].String()},
			})
		}

		// Setup Rendezvous: mark nodes as unavailable
		for i := 0; i < numRemoved; i++ {
			pool[i].setHealthy(false)
		}

		// Benchmark Memento
		b.Run(fmt.Sprintf("Memento_100Nodes_%dRemoved", numRemoved), func(b *testing.B) {
			// Warm-up
			for i := 0; i < warmupLookups; i++ {
				key := keys[i%len(keys)]
				req, _ := http.NewRequest("GET", "/", nil)
				req.RemoteAddr = key
				_ = mementoPolicy.Select(pool, req, nil)
			}

			// Actual benchmark
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := keys[i%len(keys)]
				req, _ := http.NewRequest("GET", "/", nil)
				req.RemoteAddr = key
				_ = mementoPolicy.Select(pool, req, nil)
			}
		})

		// Benchmark Rendezvous
		b.Run(fmt.Sprintf("Rendezvous_100Nodes_%dUnavailable", numRemoved), func(b *testing.B) {
			// Warm-up
			for i := 0; i < warmupLookups; i++ {
				key := keys[i%len(keys)]
				req, _ := http.NewRequest("GET", "/", nil)
				req.RemoteAddr = key
				_ = rendezvousPolicy.Select(pool, req, nil)
			}

			// Actual benchmark
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := keys[i%len(keys)]
				req, _ := http.NewRequest("GET", "/", nil)
				req.RemoteAddr = key
				_ = rendezvousPolicy.Select(pool, req, nil)
			}
		})

		// Restore nodes for next iteration
		for i := 0; i < numRemoved; i++ {
			pool[i].setHealthy(true)
		}
	}
}

// generateKeys generates a fixed set of keys using a deterministic RNG
// with the given seed. Returns a slice of IP addresses as strings.
func generateKeys(count int, seed int64) []string {
	rng := rand.New(rand.NewSource(seed))
	keys := make([]string, count)

	for i := 0; i < count; i++ {
		// Generate random IP address
		octet1 := rng.Intn(256)
		octet2 := rng.Intn(256)
		octet3 := rng.Intn(256)
		octet4 := rng.Intn(256)
		port := 80 + rng.Intn(20) // Ports 80-99

		keys[i] = fmt.Sprintf("%d.%d.%d.%d:%d", octet1, octet2, octet3, octet4, port)
	}

	return keys
}

// BenchmarkMementoVsRendezvous_ProgressiveAddition measures the average lookup time
// for Memento vs Rendezvous hashing with progressive node additions.
//
// Setup:
// - Start with 0 nodes
// - Progressive additions: {0, 50, 100, 150, ..., 1000} nodes
// - Same list of keys for all runs and both implementations
// - 1,000,000 random keys with fixed seed (1337) for reproducibility
// - Warm-up before measurement (100k lookups)
//
// Metric: Average time per lookup in ns/op
func BenchmarkMementoVsRendezvous_ProgressiveAddition(b *testing.B) {
	const (
		maxNodes      = 1000
		stepSize      = 50
		numKeys       = 1000000
		warmupLookups = 100000
		seed          = 1337
	)

	// Generate fixed set of keys with deterministic seed
	keys := generateKeys(numKeys, seed)

	// Setup context and policies
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	mementoPolicy := MementoSelection{Field: "ip"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}

	rendezvousPolicy := IPHashSelection{}

	// Create the maximum pool size once (we'll slice it as needed)
	pool := make([]*Upstream, maxNodes)
	for i := 0; i < maxNodes; i++ {
		pool[i] = &Upstream{
			Host: new(Host),
			Dial: fmt.Sprintf("localhost:%d", 8080+i),
		}
		pool[i].setHealthy(true)
	}

	// Benchmark each scenario with progressive additions
	for numNodes := 0; numNodes <= maxNodes; numNodes += stepSize {
		// Create slice of current pool size
		currentPool := pool[:numNodes]

		// Skip if no nodes
		if numNodes == 0 {
			continue
		}

		// Setup Memento: initialize topology with current nodes
		mementoPolicy.PopulateInitialTopology(currentPool)

		// Benchmark Memento
		b.Run(fmt.Sprintf("Memento_%dNodes", numNodes), func(b *testing.B) {
			// Warm-up
			for i := 0; i < warmupLookups; i++ {
				key := keys[i%len(keys)]
				req, _ := http.NewRequest("GET", "/", nil)
				req.RemoteAddr = key
				_ = mementoPolicy.Select(currentPool, req, nil)
			}

			// Actual benchmark
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := keys[i%len(keys)]
				req, _ := http.NewRequest("GET", "/", nil)
				req.RemoteAddr = key
				_ = mementoPolicy.Select(currentPool, req, nil)
			}
		})

		// Benchmark Rendezvous
		b.Run(fmt.Sprintf("Rendezvous_%dNodes", numNodes), func(b *testing.B) {
			// Warm-up
			for i := 0; i < warmupLookups; i++ {
				key := keys[i%len(keys)]
				req, _ := http.NewRequest("GET", "/", nil)
				req.RemoteAddr = key
				_ = rendezvousPolicy.Select(currentPool, req, nil)
			}

			// Actual benchmark
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := keys[i%len(keys)]
				req, _ := http.NewRequest("GET", "/", nil)
				req.RemoteAddr = key
				_ = rendezvousPolicy.Select(currentPool, req, nil)
			}
		})
	}
}

// BenchmarkMementoVsRendezvous_ConcurrentLookupAndTopologyUpdates measures
// performance under concurrent load with simultaneous lookups and topology updates.
//
// Scenario:
// - Initial cluster of 100 nodes
// - Concurrent goroutines performing lookups (read operations)
// - Concurrent goroutines performing topology updates (add/remove nodes - write operations)
// - Memento uses RWMutex (readers can proceed concurrently, writers block)
// - Rendezvous has no locking (always lock-free)
//
// This benchmark shows how RWMutex contention affects Memento vs Rendezvous
// when topology changes happen frequently during high lookup load.
func BenchmarkMementoVsRendezvous_ConcurrentLookupAndTopologyUpdates(b *testing.B) {
	const (
		totalNodes       = 100
		numKeys          = 1000000
		lookupGoroutines = 10 // Number of goroutines doing lookups
		updateGoroutines = 2  // Number of goroutines doing topology updates
		seed             = 1337
	)

	// Generate fixed set of keys with deterministic seed
	keys := generateKeys(numKeys, seed)

	// Setup context and policies
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	mementoPolicy := MementoSelection{Field: "ip"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}

	rendezvousPolicy := IPHashSelection{}

	// Create the full pool
	pool := make([]*Upstream, totalNodes)
	for i := 0; i < totalNodes; i++ {
		pool[i] = &Upstream{
			Host: new(Host),
			Dial: fmt.Sprintf("localhost:%d", 8080+i),
		}
		pool[i].setHealthy(true)
	}

	// Initialize Memento topology
	mementoPolicy.PopulateInitialTopology(pool)

	// Benchmark Memento with concurrent lookups and topology updates
	b.Run("Memento_ConcurrentLookupAndUpdates", func(b *testing.B) {
		stopUpdates := int32(0)

		// Start update goroutines (add/remove nodes continuously)
		var wgUpdates sync.WaitGroup
		for i := 0; i < updateGoroutines; i++ {
			wgUpdates.Add(1)
			go func(goroutineID int) {
				defer wgUpdates.Done()
				rng := rand.New(rand.NewSource(seed + int64(goroutineID+lookupGoroutines)))
				for atomic.LoadInt32(&stopUpdates) == 0 {
					// Alternate between add and remove
					nodeIdx := rng.Intn(totalNodes)
					if rng.Intn(2) == 0 {
						// Remove a random node
						mementoPolicy.handleUnhealthyEvent(context.Background(), caddy.Event{
							Data: map[string]any{"host": pool[nodeIdx].String()},
						})
						pool[nodeIdx].setHealthy(false)
					} else {
						// Restore a random node
						mementoPolicy.handleHealthyEvent(context.Background(), caddy.Event{
							Data: map[string]any{"host": pool[nodeIdx].String()},
						})
						pool[nodeIdx].setHealthy(true)
					}
					// Small delay to avoid overwhelming the system
					time.Sleep(time.Microsecond * time.Duration(rng.Intn(10)))
				}
			}(i)
		}

		// Run lookup benchmark with concurrent goroutines
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			rng := rand.New(rand.NewSource(seed))
			for pb.Next() {
				key := keys[rng.Intn(len(keys))]
				req, _ := http.NewRequest("GET", "/", nil)
				req.RemoteAddr = key
				_ = mementoPolicy.Select(pool, req, nil)
			}
		})
		b.StopTimer()

		// Stop update goroutines
		atomic.StoreInt32(&stopUpdates, 1)
		wgUpdates.Wait()
	})

	// Reset pool state for Rendezvous
	for i := 0; i < totalNodes; i++ {
		pool[i].setHealthy(true)
	}

	// Benchmark Rendezvous with concurrent lookups and topology updates
	b.Run("Rendezvous_ConcurrentLookupAndUpdates", func(b *testing.B) {
		stopUpdates := int32(0)

		// Start update goroutines (mark nodes as healthy/unhealthy continuously)
		var wgUpdates sync.WaitGroup
		for i := 0; i < updateGoroutines; i++ {
			wgUpdates.Add(1)
			go func(goroutineID int) {
				defer wgUpdates.Done()
				rng := rand.New(rand.NewSource(seed + int64(goroutineID+lookupGoroutines)))
				for atomic.LoadInt32(&stopUpdates) == 0 {
					// Alternate between mark unhealthy and healthy
					nodeIdx := rng.Intn(totalNodes)
					if rng.Intn(2) == 0 {
						pool[nodeIdx].setHealthy(false)
					} else {
						pool[nodeIdx].setHealthy(true)
					}
					// Small delay to avoid overwhelming the system
					time.Sleep(time.Microsecond * time.Duration(rng.Intn(10)))
				}
			}(i)
		}

		// Run lookup benchmark with concurrent goroutines
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			rng := rand.New(rand.NewSource(seed))
			for pb.Next() {
				key := keys[rng.Intn(len(keys))]
				req, _ := http.NewRequest("GET", "/", nil)
				req.RemoteAddr = key
				_ = rendezvousPolicy.Select(pool, req, nil)
			}
		})
		b.StopTimer()

		// Stop update goroutines
		atomic.StoreInt32(&stopUpdates, 1)
		wgUpdates.Wait()
	})
}
