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
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
)

// BenchmarkConcurrentSelection_HighThroughput benchmarks concurrent selection
// with many different client IPs, simulating high-throughput production load.
// This tests how both Rendezvous and Memento perform under concurrent access
// with diverse keys (different client IPs).
func BenchmarkConcurrentSelection_HighThroughput(b *testing.B) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	poolSize := 50
	pool := make([]*Upstream, poolSize)
	for i := 0; i < poolSize; i++ {
		pool[i] = &Upstream{
			Host: new(Host),
			Dial: fmt.Sprintf("localhost:%d", 8080+i),
		}
		pool[i].setHealthy(true)
	}

	// Test both Memento variants
	for _, lockFree := range []bool{false, true} {
		versionName := "RWMutex_Optimized"
		if lockFree {
			versionName = "LockFree_CopyOnWrite"
		}

		mementoPolicy, err := newMementoSelectionWithType(ctx, lockFree)
		if err != nil {
			b.Fatalf("Provision error: %v", err)
		}
		mementoPolicy.PopulateInitialTopology(pool)

		benchmarkName := fmt.Sprintf("Memento_%s_Concurrent_%dUpstreams", versionName, poolSize)
		b.Run(benchmarkName, func(b *testing.B) {
			// Generate diverse client IPs to simulate real traffic
			// Use modulo to cycle through IPs but ensure diversity
			clientIPs := make([]string, 100)
			for i := 0; i < 100; i++ {
				clientIPs[i] = fmt.Sprintf("192.168.1.%d", i%254+1)
			}

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				counter := 0
				for pb.Next() {
					req, _ := http.NewRequest("GET", "/", nil)
					// Use diverse IPs to test different hash keys
					req.RemoteAddr = clientIPs[counter%len(clientIPs)] + ":80"
					mementoPolicy.Select(pool, req, nil)
					counter++
				}
			})
		})
	}

	// Test Rendezvous for comparison
	ipHashPolicy := IPHashSelection{}
	b.Run(fmt.Sprintf("Rendezvous_Concurrent_%dUpstreams", poolSize), func(b *testing.B) {
		clientIPs := make([]string, 100)
		for i := 0; i < 100; i++ {
			clientIPs[i] = fmt.Sprintf("192.168.1.%d", i%254+1)
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			counter := 0
			for pb.Next() {
				req, _ := http.NewRequest("GET", "/", nil)
				req.RemoteAddr = clientIPs[counter%len(clientIPs)] + ":80"
				ipHashPolicy.Select(pool, req, nil)
				counter++
			}
		})
	})
}

// BenchmarkConcurrentSelection_ProductionLike benchmarks concurrent selection
// with rare topology changes, simulating a production load balancer where:
// - Many concurrent requests (read operations)
// - Very rare node health check events (write operations)
// - Topology changes happen in separate goroutines (simulating health check system)
//
// Three scenarios are tested:
// 1. Balanced: 50% removals, 50% additions
// 2. Removal-heavy: 80% removals, 20% additions
// 3. Addition-heavy: 80% additions, 20% removals
func BenchmarkConcurrentSelection_ProductionLike(b *testing.B) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	poolSize := 100
	scenarios := []struct {
		name           string
		removalPercent int // Percentage of operations that are removals (rest are additions)
	}{
		{"Balanced_50Removals_50Additions", 50},
		{"RemovalHeavy_80Removals_20Additions", 80},
		{"AdditionHeavy_20Removals_80Additions", 20},
	}

	for _, scenario := range scenarios {
		pool := make([]*Upstream, poolSize)
		for i := 0; i < poolSize; i++ {
			pool[i] = &Upstream{
				Host: new(Host),
				Dial: fmt.Sprintf("localhost:%d", 8080+i),
			}
			pool[i].setHealthy(true)
		}

		// Test Memento Lock-Free version
		mementoPolicy, err := newMementoSelectionWithType(ctx, true) // lockFree=true
		if err != nil {
			b.Fatalf("Provision error: %v", err)
		}
		mementoPolicy.PopulateInitialTopology(pool)

		benchmarkName := fmt.Sprintf("Memento_LockFree_%s_%dUpstreams", scenario.name, poolSize)
		b.Run(benchmarkName, func(b *testing.B) {
			var wg sync.WaitGroup
			stop := make(chan bool)

			// Simulate health check system with configurable removal/addition ratio
			wg.Add(1)
			go func() {
				defer wg.Done()
				nodeIndex := 0
				removedNodes := make(map[int]bool) // Track which nodes are currently removed
				operationCount := 0

				for {
					select {
					case <-stop:
						return
					default:
						currentNode := nodeIndex % poolSize
						operationCount++

						// Determine if this operation should be a removal or addition based on percentage
						shouldRemove := (operationCount % 100) < scenario.removalPercent

						if shouldRemove {
							// Removal operation
							if !removedNodes[currentNode] {
								// Node is healthy, remove it (simulate failure)
								removedHost := pool[currentNode].String()
								mementoPolicy.handleUnhealthyEvent(context.Background(), caddy.Event{
									Data: map[string]any{"host": removedHost},
								})
								removedNodes[currentNode] = true
							} else {
								// Node already removed, skip (try next node)
								nodeIndex++
								currentNode = nodeIndex % poolSize
								if !removedNodes[currentNode] {
									removedHost := pool[currentNode].String()
									mementoPolicy.handleUnhealthyEvent(context.Background(), caddy.Event{
										Data: map[string]any{"host": removedHost},
									})
									removedNodes[currentNode] = true
								}
							}
						} else {
							// Addition operation
							if removedNodes[currentNode] {
								// Node was removed, restore it (simulate recovery)
								restoredHost := pool[currentNode].String()
								mementoPolicy.handleHealthyEvent(context.Background(), caddy.Event{
									Data: map[string]any{"host": restoredHost},
								})
								removedNodes[currentNode] = false
							} else {
								// Node already healthy, skip (try next node)
								nodeIndex++
								currentNode = nodeIndex % poolSize
								if removedNodes[currentNode] {
									restoredHost := pool[currentNode].String()
									mementoPolicy.handleHealthyEvent(context.Background(), caddy.Event{
										Data: map[string]any{"host": restoredHost},
									})
									removedNodes[currentNode] = false
								}
							}
						}

						time.Sleep(200 * time.Millisecond) // 200ms = 5 topology changes per second
						nodeIndex++
					}
				}
			}()

			// Generate diverse client IPs
			clientIPs := make([]string, 100)
			for i := 0; i < 100; i++ {
				clientIPs[i] = fmt.Sprintf("10.0.0.%d", i%254+1)
			}

			b.ResetTimer()
			// All benchmark goroutines do reads (typical request pattern)
			b.RunParallel(func(pb *testing.PB) {
				counter := 0
				for pb.Next() {
					req, _ := http.NewRequest("GET", "/", nil)
					req.RemoteAddr = clientIPs[counter%len(clientIPs)] + ":80"
					mementoPolicy.Select(pool, req, nil)
					counter++
				}
			})

			close(stop)
			wg.Wait()
		})

		// Test Rendezvous for comparison (no topology changes, just concurrent reads)
		ipHashPolicy := IPHashSelection{}
		b.Run(fmt.Sprintf("Rendezvous_%s_%dUpstreams", scenario.name, poolSize), func(b *testing.B) {
			clientIPs := make([]string, 100)
			for i := 0; i < 100; i++ {
				clientIPs[i] = fmt.Sprintf("10.0.0.%d", i%254+1)
			}

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				counter := 0
				for pb.Next() {
					req, _ := http.NewRequest("GET", "/", nil)
					req.RemoteAddr = clientIPs[counter%len(clientIPs)] + ":80"
					ipHashPolicy.Select(pool, req, nil)
					counter++
				}
			})
		})
	}
}

// BenchmarkConcurrentSelection_WithRemovedNodes benchmarks concurrent selection
// when some nodes are removed, simulating a production scenario where
// some upstreams are unhealthy but requests continue to arrive.
func BenchmarkConcurrentSelection_WithRemovedNodes(b *testing.B) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	poolSize := 50
	numRemoved := 5

	pool := make([]*Upstream, poolSize)
	for i := 0; i < poolSize; i++ {
		pool[i] = &Upstream{
			Host: new(Host),
			Dial: fmt.Sprintf("localhost:%d", 8080+i),
		}
		pool[i].setHealthy(true)
	}

	// Test both Memento variants
	for _, lockFree := range []bool{false, true} {
		versionName := "RWMutex_Optimized"
		if lockFree {
			versionName = "LockFree_CopyOnWrite"
		}

		mementoPolicy, err := newMementoSelectionWithType(ctx, lockFree)
		if err != nil {
			b.Fatalf("Provision error: %v", err)
		}
		mementoPolicy.PopulateInitialTopology(pool)

		// Remove nodes from Memento topology
		for i := 0; i < numRemoved; i++ {
			removedHost := pool[i].String()
			mementoPolicy.handleUnhealthyEvent(context.Background(), caddy.Event{
				Data: map[string]any{"host": removedHost},
			})
		}

		// Mark same nodes as unavailable for Rendezvous
		for i := 0; i < numRemoved; i++ {
			pool[i].setHealthy(false)
		}

		benchmarkName := fmt.Sprintf("Memento_%s_Concurrent_%dUpstreams_%dRemoved", versionName, poolSize, numRemoved)
		b.Run(benchmarkName, func(b *testing.B) {
			clientIPs := make([]string, 100)
			for i := 0; i < 100; i++ {
				clientIPs[i] = fmt.Sprintf("172.16.0.%d", i%254+1)
			}

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				counter := 0
				for pb.Next() {
					req, _ := http.NewRequest("GET", "/", nil)
					req.RemoteAddr = clientIPs[counter%len(clientIPs)] + ":80"
					mementoPolicy.Select(pool, req, nil)
					counter++
				}
			})
		})
	}

	// Test Rendezvous for comparison
	ipHashPolicy := IPHashSelection{}
	b.Run(fmt.Sprintf("Rendezvous_Concurrent_%dUpstreams_%dUnavailable", poolSize, numRemoved), func(b *testing.B) {
		clientIPs := make([]string, 100)
		for i := 0; i < 100; i++ {
			clientIPs[i] = fmt.Sprintf("172.16.0.%d", i%254+1)
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			counter := 0
			for pb.Next() {
				req, _ := http.NewRequest("GET", "/", nil)
				req.RemoteAddr = clientIPs[counter%len(clientIPs)] + ":80"
				ipHashPolicy.Select(pool, req, nil)
				counter++
			}
		})
	})
}

// BenchmarkConcurrentSelection_DifferentPoolSizes benchmarks concurrent selection
// across different pool sizes to see how concurrency affects performance scaling.
func BenchmarkConcurrentSelection_DifferentPoolSizes(b *testing.B) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	poolSizes := []int{10, 20, 50, 100}

	// Test both Memento variants
	lockFreeVariants := []bool{false, true}
	for _, lockFree := range lockFreeVariants {
		versionName := "RWMutex_Optimized"
		if lockFree {
			versionName = "LockFree_CopyOnWrite"
		}

		for _, poolSize := range poolSizes {
			mementoPolicy, err := newMementoSelectionWithType(ctx, lockFree)
			if err != nil {
				b.Fatalf("Provision error: %v", err)
			}

			pool := make([]*Upstream, poolSize)
			for i := 0; i < poolSize; i++ {
				pool[i] = &Upstream{
					Host: new(Host),
					Dial: fmt.Sprintf("localhost:%d", 8080+i),
				}
				pool[i].setHealthy(true)
			}
			mementoPolicy.PopulateInitialTopology(pool)

			benchmarkName := fmt.Sprintf("Memento_%s_Concurrent_PoolSize_%d", versionName, poolSize)
			b.Run(benchmarkName, func(b *testing.B) {
				clientIPs := make([]string, 100)
				for i := 0; i < 100; i++ {
					clientIPs[i] = fmt.Sprintf("203.0.113.%d", i%254+1)
				}

				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					counter := 0
					for pb.Next() {
						req, _ := http.NewRequest("GET", "/", nil)
						req.RemoteAddr = clientIPs[counter%len(clientIPs)] + ":80"
						mementoPolicy.Select(pool, req, nil)
						counter++
					}
				})
			})
		}
	}

	// Test Rendezvous for comparison
	ipHashPolicy := IPHashSelection{}
	for _, poolSize := range poolSizes {
		pool := make([]*Upstream, poolSize)
		for i := 0; i < poolSize; i++ {
			pool[i] = &Upstream{
				Host: new(Host),
				Dial: fmt.Sprintf("localhost:%d", 8080+i),
			}
			pool[i].setHealthy(true)
		}

		b.Run(fmt.Sprintf("Rendezvous_Concurrent_PoolSize_%d", poolSize), func(b *testing.B) {
			clientIPs := make([]string, 100)
			for i := 0; i < 100; i++ {
				clientIPs[i] = fmt.Sprintf("203.0.113.%d", i%254+1)
			}

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				counter := 0
				for pb.Next() {
					req, _ := http.NewRequest("GET", "/", nil)
					req.RemoteAddr = clientIPs[counter%len(clientIPs)] + ":80"
					ipHashPolicy.Select(pool, req, nil)
					counter++
				}
			})
		})
	}
}
