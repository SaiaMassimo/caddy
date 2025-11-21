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
	"sync"
	"testing"
	"time"
)

// BenchmarkMemento_RWMutex_ConcurrentReads benchmarks the RWMutex version
// with concurrent reads and occasional writes/resizes
func BenchmarkMemento_RWMutex_ConcurrentReads(b *testing.B) {
	m := NewMemento()

	// Pre-populate with some entries
	for i := 0; i < 100; i++ {
		m.Remember(i, i+1, i-1)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = m.Replacer(50)
		}
	})
}

// BenchmarkMemento_LockFree_ConcurrentReads benchmarks the lock-free version
// with concurrent reads and occasional writes/resizes
func BenchmarkMemento_LockFree_ConcurrentReads(b *testing.B) {
	m := NewMementoLockFree()

	// Pre-populate with some entries
	for i := 0; i < 100; i++ {
		m.Remember(i, i+1, i-1)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = m.Replacer(50)
		}
	})
}

// BenchmarkMemento_RWMutex_ConcurrentReadsAndWrites benchmarks the RWMutex version
// with concurrent reads and writes (10% writes, 90% reads - realistic for load balancing)
func BenchmarkMemento_RWMutex_ConcurrentReadsAndWrites(b *testing.B) {
	m := NewMemento()

	// Pre-populate with some entries
	for i := 0; i < 50; i++ {
		m.Remember(i, i+1, i-1)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			if counter%10 == 0 {
				// Write operation (10% writes - realistic for occasional node changes)
				m.Remember(100+counter, 101+counter, 99+counter)
			} else {
				// Read operation (90% reads - typical lookup pattern)
				_ = m.Replacer(50)
			}
			counter++
		}
	})
}

// BenchmarkMemento_LockFree_ConcurrentReadsAndWrites benchmarks the lock-free version
// with concurrent reads and writes (10% writes, 90% reads - realistic for load balancing)
func BenchmarkMemento_LockFree_ConcurrentReadsAndWrites(b *testing.B) {
	m := NewMementoLockFree()

	// Pre-populate with some entries
	for i := 0; i < 50; i++ {
		m.Remember(i, i+1, i-1)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			if counter%10 == 0 {
				// Write operation (10% writes - realistic for occasional node changes)
				m.Remember(100+counter, 101+counter, 99+counter)
			} else {
				// Read operation (90% reads - typical lookup pattern)
				_ = m.Replacer(50)
			}
			counter++
		}
	})
}

// BenchmarkMemento_RWMutex_RealisticLoadBalancing benchmarks the RWMutex version
// with a realistic load balancing scenario: many readers, rare writers
// This represents a production load balancer where node changes are very rare
// Writers run in separate goroutines to simulate realistic scenario
func BenchmarkMemento_RWMutex_RealisticLoadBalancing(b *testing.B) {
	m := NewMemento()

	// Pre-populate with some entries
	for i := 0; i < 100; i++ {
		m.Remember(i, i+1, i-1)
	}

	var wg sync.WaitGroup
	stop := make(chan bool)

	// Start a few writer goroutines (simulating rare node changes)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			counter := 0
			for {
				select {
				case <-stop:
					return
				default:
					// Very rare writes (simulate node health check events)
					m.Remember(1000+writerID*10000+counter, 1001+writerID*10000+counter, 999+writerID*10000+counter)
					counter++
					// Delay to make writes truly rare (simulate real scenario where node changes are infrequent)
					// In production, node health checks happen every few seconds, not continuously
					time.Sleep(10 * time.Millisecond)
				}
			}
		}(i)
	}

	b.ResetTimer()
	// All benchmark goroutines do reads (typical lookup pattern)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = m.Replacer(50)
		}
	})

	close(stop)
	wg.Wait()
}

// BenchmarkMemento_LockFree_RealisticLoadBalancing benchmarks the lock-free version
// with a realistic load balancing scenario: many readers, rare writers
// This represents a production load balancer where node changes are very rare
// Writers run in separate goroutines to simulate realistic scenario
func BenchmarkMemento_LockFree_RealisticLoadBalancing(b *testing.B) {
	m := NewMementoLockFree()

	// Pre-populate with some entries
	for i := 0; i < 100; i++ {
		m.Remember(i, i+1, i-1)
	}

	var wg sync.WaitGroup
	stop := make(chan bool)

	// Start a few writer goroutines (simulating rare node changes)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			counter := 0
			for {
				select {
				case <-stop:
					return
				default:
					// Very rare writes (simulate node health check events)
					m.Remember(1000+writerID*10000+counter, 1001+writerID*10000+counter, 999+writerID*10000+counter)
					counter++
					// Delay to make writes truly rare (simulate real scenario where node changes are infrequent)
					// In production, node health checks happen every few seconds, not continuously
					time.Sleep(10 * time.Millisecond)
				}
			}
		}(i)
	}

	b.ResetTimer()
	// All benchmark goroutines do reads (typical lookup pattern)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = m.Replacer(50)
		}
	})

	close(stop)
	wg.Wait()
}

// BenchmarkMemento_RWMutex_ResizeStress benchmarks the RWMutex version
// during frequent resize operations (worst case scenario)
func BenchmarkMemento_RWMutex_ResizeStress(b *testing.B) {
	m := NewMemento()

	var wg sync.WaitGroup
	stop := make(chan bool)

	// Start goroutine that continuously triggers resizes
	wg.Add(1)
	go func() {
		defer wg.Done()
		counter := 0
		for {
			select {
			case <-stop:
				return
			default:
				m.Remember(1000+counter, 1001+counter, 999+counter)
				counter++
			}
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = m.Replacer(50)
		}
	})

	close(stop)
	wg.Wait()
}

// BenchmarkMemento_LockFree_ResizeStress benchmarks the lock-free version
// during frequent resize operations (worst case scenario)
func BenchmarkMemento_LockFree_ResizeStress(b *testing.B) {
	m := NewMementoLockFree()

	var wg sync.WaitGroup
	stop := make(chan bool)

	// Start goroutine that continuously triggers resizes
	wg.Add(1)
	go func() {
		defer wg.Done()
		counter := 0
		for {
			select {
			case <-stop:
				return
			default:
				m.Remember(1000+counter, 1001+counter, 999+counter)
				counter++
			}
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = m.Replacer(50)
		}
	})

	close(stop)
	wg.Wait()
}

// BenchmarkMementoEngine_RWMutex benchmarks MementoEngine with RWMutex version
func BenchmarkMementoEngine_RWMutex(b *testing.B) {
	engine := NewMementoEngineWithType(100, false)

	// Pre-populate with removals
	for i := 0; i < 50; i++ {
		engine.RemoveBucket(i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			_ = engine.GetBucket(fmt.Sprintf("test-key-%d", counter))
			counter++
		}
	})
}

// BenchmarkMementoEngine_LockFree benchmarks MementoEngine with lock-free version
func BenchmarkMementoEngine_LockFree(b *testing.B) {
	engine := NewMementoEngineWithType(100, true)

	// Pre-populate with removals
	for i := 0; i < 50; i++ {
		engine.RemoveBucket(i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			_ = engine.GetBucket(fmt.Sprintf("test-key-%d", counter))
			counter++
		}
	})
}
