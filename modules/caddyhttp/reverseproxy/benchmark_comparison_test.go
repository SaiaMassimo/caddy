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
	"testing"

	"github.com/caddyserver/caddy/v2"
)

// Benchmarks compare Rendezvous Hashing vs Memento in different scenarios


func BenchmarkRendezvousVsMemento_DifferentPoolSizes(b *testing.B) {
	// Test scenario: Performance with different upstream pool sizes
	
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()
	
    mementoPolicy := MementoSelection{Field: "ip"}
    if err := mementoPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}

	poolSizes := []int{3, 5, 10, 20, 50, 100}
	
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

        b.Run(fmt.Sprintf("Memento_PoolSize_%d", size), func(b *testing.B) {
			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = "172.0.0.1:80"
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
                mementoPolicy.Select(pool, req, nil)
			}
		})
	}
}

func BenchmarkRendezvous_PoolSizes(b *testing.B) {
	// Test scenario: Rendezvous Hashing performance with different upstream pool sizes
	
	ipHashPolicy := IPHashSelection{}

	poolSizes := []int{3, 5, 10, 20, 50, 100}
	
	for _, size := range poolSizes {
		pool := make([]*Upstream, size)
		for i := 0; i < size; i++ {
			pool[i] = &Upstream{
				Host: new(Host),
				Dial: fmt.Sprintf("localhost:%d", 8080+i),
			}
			pool[i].setHealthy(true)
		}

		b.Run(fmt.Sprintf("Rendezvous_PoolSize_%d", size), func(b *testing.B) {
			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = "172.0.0.1:80"
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ipHashPolicy.Select(pool, req, nil)
			}
		})
	}
}

func BenchmarkMementoVsRendezvous_WithRemovedNodes(b *testing.B) {
	// Test scenario: Performance comparison with 5 nodes always removed
	// Tests how Memento handles topology changes vs Rendezvous filtering unavailable hosts
	
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()
	
	mementoPolicy := MementoSelection{Field: "ip"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}
	
	ipHashPolicy := IPHashSelection{}
	
	poolSizes := []int{10, 20, 50, 100}
	const numRemovedNodes = 5
	
	for _, size := range poolSizes {
		// Skip if pool is too small
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
		
		// Initialize Memento topology with all nodes
		mementoPolicy.PopulateInitialTopology(pool)
		
		// Remove 5 nodes from Memento topology using events
		for i := 0; i < numRemovedNodes; i++ {
			mementoPolicy.handleUnhealthyEvent(context.Background(), caddy.Event{
				Data: map[string]any{"host": pool[i].String()},
			})
		}
		
		// Mark same 5 nodes as unavailable for Rendezvous
		for i := 0; i < numRemovedNodes; i++ {
			pool[i].setHealthy(false)
		}
		
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = "172.0.0.1:80"
		
		// Benchmark Memento with removed nodes
		b.Run(fmt.Sprintf("Memento_%dNodes_%dRemoved", size, numRemovedNodes), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				mementoPolicy.Select(pool, req, nil)
			}
		})
		
		// Benchmark Rendezvous with unavailable hosts
		b.Run(fmt.Sprintf("Rendezvous_%dNodes_%dUnavailable", size, numRemovedNodes), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ipHashPolicy.Select(pool, req, nil)
			}
		})
	}
}

func BenchmarkMementoVsRendezvous_100Nodes_ProgressiveRemoval(b *testing.B) {
	// Test scenario: Fixed 100 nodes, progressively remove 5, 10, 20, 50 nodes
	// Shows how performance degrades as more nodes are removed
	
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()
	
	mementoPolicy := MementoSelection{Field: "ip"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}
	
	ipHashPolicy := IPHashSelection{}
	
	const totalNodes = 100
	removalCounts := []int{0, 5, 10, 20, 50}
	
	// Create the full pool once
	pool := make([]*Upstream, totalNodes)
	for i := 0; i < totalNodes; i++ {
		pool[i] = &Upstream{
			Host: new(Host),
			Dial: fmt.Sprintf("localhost:%d", 8080+i),
		}
		pool[i].setHealthy(true)
	}
	
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.0.0.1:80"
	
	for _, numRemoved := range removalCounts {
		// Skip if removing too many nodes
		if numRemoved >= totalNodes {
			continue
		}
		
		// Initialize Memento topology with all nodes
		mementoPolicy.PopulateInitialTopology(pool)
		
		// Remove nodes from Memento topology using events
		for i := 0; i < numRemoved; i++ {
			mementoPolicy.handleUnhealthyEvent(context.Background(), caddy.Event{
				Data: map[string]any{"host": pool[i].String()},
			})
		}
		
		// Mark same nodes as unavailable for Rendezvous
		for i := 0; i < numRemoved; i++ {
			pool[i].setHealthy(false)
		}
		
		// Benchmark Memento with progressive removal
		b.Run(fmt.Sprintf("Memento_100Nodes_%dRemoved", numRemoved), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				mementoPolicy.Select(pool, req, nil)
			}
		})
		
		// Benchmark Rendezvous with progressive removal
		b.Run(fmt.Sprintf("Rendezvous_100Nodes_%dUnavailable", numRemoved), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ipHashPolicy.Select(pool, req, nil)
			}
		})
		
		// Restore nodes for next iteration
		for i := 0; i < numRemoved; i++ {
			pool[i].setHealthy(true)
		}
	}
}

