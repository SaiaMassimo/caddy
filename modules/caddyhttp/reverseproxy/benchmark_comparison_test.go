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

// BenchmarkRendezvousVsBinomial compares Rendezvous Hashing vs BinomialHash
// in different scenarios to measure performance differences

func BenchmarkRendezvousVsBinomial_SameKey(b *testing.B) {
	// Test scenario: Same key repeated many times (cache-friendly)
	
	// Setup Rendezvous (IP Hash)
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()
	
	ipHashPolicy := IPHashSelection{}
	binomialPolicy := BinomialSelection{Field: "ip"}
	if err := binomialPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}

	pool := testPool()
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.0.0.1:80"

	b.Run("Rendezvous_IPHash_SameKey", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ipHashPolicy.Select(pool, req, nil)
		}
	})

	b.Run("Binomial_SameKey", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			binomialPolicy.Select(pool, req, nil)
		}
	})
}

func BenchmarkRendezvousVsBinomial_DifferentKeys(b *testing.B) {
	// Test scenario: Different keys each time (no cache benefit)
	
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()
	
	ipHashPolicy := IPHashSelection{}
	binomialPolicy := BinomialSelection{Field: "ip"}
	if err := binomialPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}

	pool := testPool()

	b.Run("Rendezvous_IPHash_DifferentKeys", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = fmt.Sprintf("172.0.0.%d:80", i%256)
			ipHashPolicy.Select(pool, req, nil)
		}
	})

	b.Run("Binomial_DifferentKeys", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = fmt.Sprintf("172.0.0.%d:80", i%256)
			binomialPolicy.Select(pool, req, nil)
		}
	})
}

func BenchmarkRendezvousVsBinomial_URIHash(b *testing.B) {
	// Test scenario: URI-based hashing comparison
	
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()
	
	uriHashPolicy := URIHashSelection{}
	binomialPolicy := BinomialSelection{Field: "uri"}
	if err := binomialPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}

	pool := testPool()

	b.Run("Rendezvous_URIHash_SameURI", func(b *testing.B) {
		req, _ := http.NewRequest("GET", "/test-endpoint", nil)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			uriHashPolicy.Select(pool, req, nil)
		}
	})

	b.Run("Binomial_URI_SameURI", func(b *testing.B) {
		req, _ := http.NewRequest("GET", "/test-endpoint", nil)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			binomialPolicy.Select(pool, req, nil)
		}
	})

	b.Run("Rendezvous_URIHash_DifferentURIs", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/endpoint-%d", i%1000), nil)
			uriHashPolicy.Select(pool, req, nil)
		}
	})

	b.Run("Binomial_URI_DifferentURIs", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req, _ := http.NewRequest("GET", fmt.Sprintf("/endpoint-%d", i%1000), nil)
			binomialPolicy.Select(pool, req, nil)
		}
	})
}

func BenchmarkRendezvousVsBinomial_DifferentPoolSizes(b *testing.B) {
	// Test scenario: Performance with different upstream pool sizes
	
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()
	
	binomialPolicy := BinomialSelection{Field: "ip"}
	if err := binomialPolicy.Provision(ctx); err != nil {
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

		b.Run(fmt.Sprintf("Binomial_PoolSize_%d", size), func(b *testing.B) {
			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = "172.0.0.1:80"
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				binomialPolicy.Select(pool, req, nil)
			}
		})
	}
}

func BenchmarkRendezvousVsBinomial_RendezvousPoolSizes(b *testing.B) {
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

func BenchmarkRendezvousVsBinomial_HeaderHash(b *testing.B) {
	// Test scenario: Header-based hashing comparison
	
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()
	
	headerHashPolicy := HeaderHashSelection{Field: "User-Agent"}
	if err := headerHashPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}
	
	binomialPolicy := BinomialSelection{
		Field:       "header",
		HeaderField: "User-Agent",
	}
	if err := binomialPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}

	pool := testPool()

	b.Run("Rendezvous_HeaderHash_SameHeader", func(b *testing.B) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 Test Browser")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			headerHashPolicy.Select(pool, req, nil)
		}
	})

	b.Run("Binomial_Header_SameHeader", func(b *testing.B) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 Test Browser")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			binomialPolicy.Select(pool, req, nil)
		}
	})

	b.Run("Rendezvous_HeaderHash_DifferentHeaders", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req, _ := http.NewRequest("GET", "/", nil)
			req.Header.Set("User-Agent", fmt.Sprintf("Browser-%d", i%100))
			headerHashPolicy.Select(pool, req, nil)
		}
	})

	b.Run("Binomial_Header_DifferentHeaders", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req, _ := http.NewRequest("GET", "/", nil)
			req.Header.Set("User-Agent", fmt.Sprintf("Browser-%d", i%100))
			binomialPolicy.Select(pool, req, nil)
		}
	})
}

func BenchmarkRendezvousVsBinomial_WithUnavailableHosts(b *testing.B) {
	// Test scenario: Performance when some hosts are unavailable
	
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()
	
	ipHashPolicy := IPHashSelection{}
	binomialPolicy := BinomialSelection{Field: "ip"}
	if err := binomialPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}

	pool := testPool()
	
	// Mark some hosts as unavailable
	pool[1].setHealthy(false)
	pool[2].setHealthy(false)

	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.0.0.1:80"

	b.Run("Rendezvous_IPHash_WithUnavailableHosts", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ipHashPolicy.Select(pool, req, nil)
		}
	})

	b.Run("Binomial_WithUnavailableHosts", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			binomialPolicy.Select(pool, req, nil)
		}
	})
}

func BenchmarkRendezvousVsBinomial_MemoryAllocation(b *testing.B) {
	// Test scenario: Memory allocation patterns
	
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()
	
	ipHashPolicy := IPHashSelection{}
	binomialPolicy := BinomialSelection{Field: "ip"}
	if err := binomialPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}

	pool := testPool()

	b.Run("Rendezvous_IPHash_Memory", func(b *testing.B) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = "172.0.0.1:80"
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ipHashPolicy.Select(pool, req, nil)
		}
	})

	b.Run("Binomial_Memory", func(b *testing.B) {
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = "172.0.0.1:80"
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			binomialPolicy.Select(pool, req, nil)
		}
	})
}

func BenchmarkRendezvousVsBinomial_ConcurrentAccess(b *testing.B) {
	// Test scenario: Concurrent access patterns
	
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()
	
	ipHashPolicy := IPHashSelection{}
	binomialPolicy := BinomialSelection{Field: "ip"}
	if err := binomialPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}

	pool := testPool()

	b.Run("Rendezvous_IPHash_Concurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = "172.0.0.1:80"
			for pb.Next() {
				ipHashPolicy.Select(pool, req, nil)
			}
		})
	})

	b.Run("Binomial_Concurrent", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			req, _ := http.NewRequest("GET", "/", nil)
			req.RemoteAddr = "172.0.0.1:80"
			for pb.Next() {
				binomialPolicy.Select(pool, req, nil)
			}
		})
	})
}

func BenchmarkRendezvousVsBinomial_ConsistencyCheck(b *testing.B) {
	// Test scenario: Consistency of hash distribution
	
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()
	
	ipHashPolicy := IPHashSelection{}
	binomialPolicy := BinomialSelection{Field: "ip"}
	if err := binomialPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}

	pool := testPool()
	numKeys := 1000

	b.Run("Rendezvous_IPHash_Consistency", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for j := 0; j < numKeys; j++ {
				req, _ := http.NewRequest("GET", "/", nil)
				req.RemoteAddr = fmt.Sprintf("172.0.0.%d:80", j%256)
				ipHashPolicy.Select(pool, req, nil)
			}
		}
	})

	b.Run("Binomial_Consistency", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for j := 0; j < numKeys; j++ {
				req, _ := http.NewRequest("GET", "/", nil)
				req.RemoteAddr = fmt.Sprintf("172.0.0.%d:80", j%256)
				binomialPolicy.Select(pool, req, nil)
			}
		}
	})
}
