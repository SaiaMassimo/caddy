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
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/caddyserver/caddy/v2"
)

// Note: CSV writing is handled by the benchmark_to_csv.go script which parses the benchmark output
// The functions below are kept for potential future use but are currently not used

// runBenchmarkAndWriteCSV runs a benchmark function normally
// CSV writing is handled by the benchmark_to_csv.go script which parses the output
// This function is kept for API compatibility but just runs the benchmark normally
func runBenchmarkAndWriteCSV(b *testing.B, benchmarkName string, benchFunc func(*testing.B)) {
	// Just run the benchmark normally
	// CSV will be written by parsing the benchmark output using benchmark_to_csv.go
	b.Run(benchmarkName, benchFunc)
}

// CSV writing functions are not used - benchmark_to_csv.go handles CSV generation

// Benchmarks compare Rendezvous Hashing vs Memento Lock-Free in different scenarios
//
// Memento Lock-Free: Fully lock-free version using atomic operations for reads/writes
// and copy-on-write for resize operations

// newMementoSelectionWithType creates a MementoSelection with the specified implementation type
// lockFree=true for Lock-Free version (fully lock-free with atomic operations)
func newMementoSelectionWithType(ctx caddy.Context, lockFree bool) (*MementoSelection, error) {
	policy := MementoSelection{Field: "ip"}
	if err := policy.Provision(ctx); err != nil {
		return nil, err
	}
	// Replace the consistentEngine with the specified type
	policy.consistentEngine = NewConsistentEngineWithType(lockFree)
	return &policy, nil
}

// verifyMementoSelection verifies that the selected upstream is valid:
// 1. Not in the removed nodes list
// 2. Actually selected by Memento (not fallback) - verified by recalculating what Memento would select
func verifyMementoSelection(selected *Upstream, removedNodes map[string]bool, policy *MementoSelection, key string) error {
	if selected == nil {
		return fmt.Errorf("selected upstream is nil")
	}

	selectedHost := selected.String()

	// Check 1: Not in removed nodes
	if removedNodes[selectedHost] {
		return fmt.Errorf("selected node %s was removed and should not be returned", selectedHost)
	}

	// Check 2: Verify it's actually from Memento (not fallback)
	// Recalculate what Memento would select for the same key and verify it matches
	if policy.consistentEngine == nil {
		return fmt.Errorf("consistent engine is nil - using fallback")
	}
	engineSize := policy.consistentEngine.Size()
	if engineSize == 0 {
		return fmt.Errorf("consistent engine is not initialized (size=0) - using fallback")
	}

	// Get the upstream that Memento would select for this key
	expectedUpstream := policy.consistentEngine.GetBucket(key)
	if expectedUpstream == nil {
		return fmt.Errorf("GetBucket returned nil for key %s - using fallback", key)
	}

	// Verify the selected node matches what Memento would select
	if selectedHost != expectedUpstream.String() {
		return fmt.Errorf("selected node %s does not match Memento's expected node %s (key=%s) - likely using fallback",
			selectedHost, expectedUpstream.String(), key)
	}

	return nil
}

func BenchmarkRendezvousVsMemento_DifferentPoolSizes(b *testing.B) {
	// Test scenario: Performance with different upstream pool sizes
	// Compares Rendezvous Hashing vs Memento Lock-Free

	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	poolSizes := []int{3, 5, 10, 20, 50, 100}
	ipHashPolicy := IPHashSelection{}

	// Test Memento Lock-Free version
	b.Logf("Testing Memento version: Lock-Free (fully lock-free with atomic operations)")

	mementoPolicy, err := newMementoSelectionWithType(ctx, true) // lockFree=true
	if err != nil {
		b.Fatalf("Provision error: %v", err)
	}

	for _, size := range poolSizes {
		pool := make([]*Upstream, size)
		for i := 0; i < size; i++ {
			pool[i] = &Upstream{
				Host: new(Host),
				Dial: fmt.Sprintf("localhost:%d", 8080+i),
			}
			pool[i].setHealthy(true)
		}

		// Initialize the topology for memento to actually test memento, not the fallback
		mementoPolicy.PopulateInitialTopology(pool)

		benchmarkName := fmt.Sprintf("Memento_LockFree_PoolSize_%d", size)
		runBenchmarkAndWriteCSV(b, benchmarkName, func(b *testing.B) {
			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = "172.0.0.1:80"
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				mementoPolicy.Select(pool, req, nil)
			}
		})
	}

	// Test Rendezvous Hashing for comparison
	for _, size := range poolSizes {
		pool := make([]*Upstream, size)
		for i := 0; i < size; i++ {
			pool[i] = &Upstream{
				Host: new(Host),
				Dial: fmt.Sprintf("localhost:%d", 8080+i),
			}
			pool[i].setHealthy(true)
		}

		benchmarkName := fmt.Sprintf("Rendezvous_PoolSize_%d", size)
		runBenchmarkAndWriteCSV(b, benchmarkName, func(b *testing.B) {
			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = "172.0.0.1:80"
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ipHashPolicy.Select(pool, req, nil)
			}
		})
	}
}

// BenchmarkRendezvous_PoolSizes is now integrated into BenchmarkRendezvousVsMemento_DifferentPoolSizes
// This function is kept for backward compatibility but is now a no-op
func BenchmarkRendezvous_PoolSizes(b *testing.B) {
	// This benchmark is now integrated into BenchmarkRendezvousVsMemento_DifferentPoolSizes
	// Keeping this function for backward compatibility
	b.Skip("This benchmark is now part of BenchmarkRendezvousVsMemento_DifferentPoolSizes")
}

func BenchmarkMementoVsRendezvous_WithRemovedNodes(b *testing.B) {
	// Test scenario: Performance comparison with 5 nodes always removed
	// Tests how Memento handles topology changes vs Rendezvous filtering unavailable hosts
	// Compares Memento Lock-Free with Rendezvous

	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	ipHashPolicy := IPHashSelection{}

	poolSizes := []int{10, 20, 50, 100}
	const numRemovedNodes = 5

	// Test Memento Lock-Free version
	b.Logf("Testing Memento version: Lock-Free (fully lock-free with atomic operations)")

	for _, size := range poolSizes {
		// Skip if pool is too small
		if size <= numRemovedNodes {
			continue
		}

		// Create a fresh policy for each test to avoid state accumulation
		testPolicy, err := newMementoSelectionWithType(ctx, true) // lockFree=true
		if err != nil {
			b.Fatalf("Provision error: %v", err)
		}

		pool := make([]*Upstream, size)
		for i := 0; i < size; i++ {
			pool[i] = &Upstream{
				Host: new(Host),
				Dial: fmt.Sprintf("localhost:%d", 8080+i),
			}
			pool[i].setHealthy(true)
		}

		// Initialize Memento topology with all nodes
		testPolicy.PopulateInitialTopology(pool)

		// Remove 5 nodes from Memento topology using events
		removedNodes := make(map[string]bool)
		for i := 0; i < numRemovedNodes; i++ {
			removedHost := pool[i].String()
			removedNodes[removedHost] = true
			testPolicy.handleUnhealthyEvent(context.Background(), caddy.Event{
				Data: map[string]any{"host": removedHost},
			})
		}

		// Mark same 5 nodes as unavailable for Rendezvous
		for i := 0; i < numRemovedNodes; i++ {
			pool[i].setHealthy(false)
		}

		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = "172.0.0.1:80"
		key := "172.0.0.1" // Extract key from RemoteAddr

		// Verify correctness before benchmark (sample check)
		verificationSamples := 100
		if verificationSamples > b.N {
			verificationSamples = b.N
		}
		for i := 0; i < verificationSamples; i++ {
			selected := testPolicy.Select(pool, req, nil)
			if err := verifyMementoSelection(selected, removedNodes, testPolicy, key); err != nil {
				b.Fatalf("Verification failed at sample %d: %v", i, err)
			}
		}

		// Benchmark Memento with removed nodes
		// Note: With removed nodes, Memento must follow replacement chains during lookup,
		// which adds overhead. This should be visible in the benchmark results.
		benchmarkName := fmt.Sprintf("Memento_LockFree_%dNodes_%dRemoved", size, numRemovedNodes)
		runBenchmarkAndWriteCSV(b, benchmarkName, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				testPolicy.Select(pool, req, nil)
			}
		})
	}

	// Benchmark Rendezvous separately (only once, not per version)
	for _, size := range poolSizes {
		if size <= numRemovedNodes {
			continue
		}

		pool := make([]*Upstream, size)
		for i := 0; i < size; i++ {
			pool[i] = &Upstream{
				Host: new(Host),
				Dial: fmt.Sprintf("localhost:%d", 8080+i),
			}
			pool[i].setHealthy(true)
		}

		// Mark 5 nodes as unavailable for Rendezvous
		for i := 0; i < numRemovedNodes; i++ {
			pool[i].setHealthy(false)
		}

		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = "172.0.0.1:80"

		benchmarkName := fmt.Sprintf("Rendezvous_%dNodes_%dUnavailable", size, numRemovedNodes)
		runBenchmarkAndWriteCSV(b, benchmarkName, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ipHashPolicy.Select(pool, req, nil)
			}
		})
	}
}

func BenchmarkMementoVsRendezvous_100Nodes_ProgressiveRemoval(b *testing.B) {
	// Test scenario: Fixed 100 nodes, progressively remove 0, 1, 10, 20, 50 nodes
	// Shows how performance degrades as more nodes are removed
	// Compares Memento Lock-Free with Rendezvous
	// Uses 10 different IP addresses to get average performance across different keys

	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	ipHashPolicy := IPHashSelection{}

	const totalNodes = 100
	removalCounts := []int{0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 99}

	// Use 10 different IP addresses to test distribution and get average performance
	testIPs := []string{
		"172.0.0.1",
		"172.0.0.2",
		"172.0.0.3",
		"172.0.0.4",
		"172.0.0.5",
		"10.0.0.1",
		"10.0.0.2",
		"10.0.0.3",
		"192.168.1.1",
		"192.168.1.2",
	}

	// Test Memento Lock-Free version
	b.Logf("Testing Memento version: Lock-Free (fully lock-free with atomic operations)")

	// Create the full pool once
	pool := make([]*Upstream, totalNodes)
	for i := 0; i < totalNodes; i++ {
		pool[i] = &Upstream{
			Host: new(Host),
			Dial: fmt.Sprintf("localhost:%d", 8080+i),
		}
		pool[i].setHealthy(true)
	}

	for _, numRemoved := range removalCounts {
		// Skip if removing too many nodes
		if numRemoved >= totalNodes {
			continue
		}

		// Create a fresh policy for each test to avoid state accumulation
		// This ensures each test starts with a clean state
		testPolicy, err := newMementoSelectionWithType(ctx, true) // lockFree=true
		if err != nil {
			b.Fatalf("Provision error: %v", err)
		}

		// Initialize Memento topology with all nodes
		testPolicy.PopulateInitialTopology(pool)

		// Remove nodes from Memento topology using events
		removedNodes := make(map[string]bool)
		for i := 0; i < numRemoved; i++ {
			removedHost := pool[i].String()
			removedNodes[removedHost] = true
			testPolicy.handleUnhealthyEvent(context.Background(), caddy.Event{
				Data: map[string]any{"host": removedHost},
			})
		}

		// Verify correctness before benchmark (sample check with all IPs)
		// Verify each IP to ensure correctness across different keys
		verificationSamplesPerIP := 10
		for _, testIP := range testIPs {
			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = testIP + ":80"
			key := testIP

			for i := 0; i < verificationSamplesPerIP; i++ {
				selected := testPolicy.Select(pool, req, nil)
				if err := verifyMementoSelection(selected, removedNodes, testPolicy, key); err != nil {
					b.Fatalf("Verification failed for IP %s at sample %d (removed=%d): %v", testIP, i, numRemoved, err)
				}
			}
		}

		// Benchmark Memento with progressive removal
		// Use all 10 IPs in rotation to get average performance across different keys
		// Note: With more removals, Memento must follow longer replacement chains,
		// which should increase lookup time. This benchmark should show that cost.
		benchmarkName := fmt.Sprintf("Memento_LockFree_100Nodes_%dRemoved", numRemoved)
		runBenchmarkAndWriteCSV(b, benchmarkName, func(b *testing.B) {
			// Create requests for all IPs
			requests := make([]*http.Request, len(testIPs))
			keys := make([]string, len(testIPs))
			for i, testIP := range testIPs {
				req, _ := http.NewRequest("GET", "/", nil)
				req.RemoteAddr = testIP + ":80"
				requests[i] = req
				keys[i] = testIP
			}

			b.ResetTimer()
			// Rotate through all IPs to get average performance
			for i := 0; i < b.N; i++ {
				ipIndex := i % len(testIPs)
				testPolicy.Select(pool, requests[ipIndex], nil)
			}
		})
	}

	// Benchmark Rendezvous separately (only once, not per version)
	// Use the same 10 IPs for consistency
	pool = make([]*Upstream, totalNodes)
	for i := 0; i < totalNodes; i++ {
		pool[i] = &Upstream{
			Host: new(Host),
			Dial: fmt.Sprintf("localhost:%d", 8080+i),
		}
		pool[i].setHealthy(true)
	}

	for _, numRemoved := range removalCounts {
		if numRemoved >= totalNodes {
			continue
		}

		// Mark nodes as unavailable for Rendezvous
		for i := 0; i < numRemoved; i++ {
			pool[i].setHealthy(false)
		}

		// Benchmark Rendezvous with progressive removal
		// Use all 10 IPs in rotation to get average performance across different keys
		benchmarkName := fmt.Sprintf("Rendezvous_100Nodes_%dUnavailable", numRemoved)
		runBenchmarkAndWriteCSV(b, benchmarkName, func(b *testing.B) {
			// Create requests for all IPs
			requests := make([]*http.Request, len(testIPs))
			for i, testIP := range testIPs {
				req, _ := http.NewRequest("GET", "/", nil)
				req.RemoteAddr = testIP + ":80"
				requests[i] = req
			}

			b.ResetTimer()
			// Rotate through all IPs to get average performance
			for i := 0; i < b.N; i++ {
				ipIndex := i % len(testIPs)
				ipHashPolicy.Select(pool, requests[ipIndex], nil)
			}
		})

		// Restore nodes for next iteration
		for i := 0; i < numRemoved; i++ {
			pool[i].setHealthy(true)
		}
	}
}
