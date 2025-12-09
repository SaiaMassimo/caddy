# Memento Implementation Benchmark Documentation

This document describes the benchmark tests used to compare the two Memento implementations: **RWMutex** and **Lock-Free**. These benchmarks evaluate performance characteristics under different workload patterns to identify the strengths of each implementation.

## Overview

The Memento data structure is used to track removed buckets in a consistent hashing system. Two implementations are available:

1. **RWMutex Version**: Uses read-write mutexes for thread safety
2. **Lock-Free Version**: Uses atomic operations and lock-free algorithms

## Benchmark Test Scenarios

### 1. Concurrent Reads Only

**Test**: `BenchmarkMemento_RWMutex_ConcurrentReads` vs `BenchmarkMemento_LockFree_ConcurrentReads`

**What It Tests**:
- Pure read performance under high concurrency
- No write operations during the benchmark
- Pre-populated with 100 entries
- All goroutines perform `Replacer()` lookups

**Results Analysis**:
- **Lock-Free**: ~1.022 ns/op (extremely fast, no locking overhead)
- **RWMutex**: ~49.90 ns/op (read lock acquisition overhead)
- **Winner**: Lock-Free (49x faster)

**Key Insight**: When only reads are performed, lock-free implementation eliminates all locking overhead, providing near-direct memory access performance.

---

### 2. Concurrent Reads and Writes (10% Writes)

**Test**: `BenchmarkMemento_RWMutex_ConcurrentReadsAndWrites` vs `BenchmarkMemento_LockFree_ConcurrentReadsAndWrites`

**What It Tests**:
- Mixed workload: 90% reads, 10% writes
- Simulates realistic load balancing scenario with occasional node changes
- Pre-populated with 50 entries
- Each goroutine alternates between read and write operations

**Results Analysis**:
- **Lock-Free**: ~36.35 ns/op, 48 B/op, 0 allocs/op
- **RWMutex**: ~74.11 ns/op, 13 B/op, 0 allocs/op
- **Winner**: Lock-Free (2x faster)

**Key Insight**: With the optimized lock-free implementation (no copy-on-write for Remember/Restore):
- Lock-Free now performs direct in-place modifications (O(1) operations)
- Pointer assignments are atomic in Go, making writes lock-free and efficient
- Lock-Free is faster even with 10% writes because it avoids read lock overhead
- RWMutex still has read lock acquisition overhead even during writes

---

### 3. Realistic Load Balancing (Rare Writes)

**Test**: `BenchmarkMemento_RWMutex_RealisticLoadBalancing` vs `BenchmarkMemento_LockFree_RealisticLoadBalancing`

**What It Tests**:
- Production-like scenario: many readers, very rare writers
- 2 writer goroutines that write every 10ms (simulating health check events)
- All benchmark goroutines perform reads
- Pre-populated with 100 entries

**Results Analysis**:
- **Lock-Free**: ~1.047 ns/op (readers unaffected by rare writes)
- **RWMutex**: ~53.57 ns/op (read lock overhead even when no writes occur)
- **Winner**: Lock-Free (51x faster)

**Key Insight**: In production scenarios where node changes are infrequent (health checks every few seconds), the lock-free version provides optimal read performance because:
- Readers never block (no locks)
- Rare writes don't impact read performance
- No read lock acquisition overhead

---

### 4. Resize Stress (Frequent Resizes)

**Test**: `BenchmarkMemento_RWMutex_ResizeStress` vs `BenchmarkMemento_LockFree_ResizeStress`

**What It Tests**:
- Worst-case scenario: continuous resize operations
- One goroutine continuously adds entries (triggering resizes)
- All benchmark goroutines perform reads during resizes
- Tests behavior under maximum contention

**Results Analysis**:
- **Lock-Free**: ~2.950 ns/op (readers unaffected by resizes)
- **RWMutex**: ~142.8 ns/op, 59 B/op, 1 alloc/op (readers blocked during resize)
- **Winner**: Lock-Free (48x faster)

**Key Insight**: During resize operations:
- **Lock-Free**: Readers continue at full speed (no blocking)
- **RWMutex**: All readers are blocked by the write lock during resize, causing significant latency spikes

The lock-free version's ability to read from old tables during resize provides superior performance.

---

### 5. MementoEngine Integration

**Test**: `BenchmarkMementoEngine_RWMutex` vs `BenchmarkMementoEngine_LockFree`

**What It Tests**:
- End-to-end performance of MementoEngine using each implementation
- Pre-populated with 50 removed buckets
- Concurrent `GetBucket()` operations that traverse the Memento structure
- Real-world usage pattern

**Results Analysis**:
- **Lock-Free**: ~57.75 ns/op, 24 B/op, 1 alloc/op
- **RWMutex**: ~138.4 ns/op, 24 B/op, 1 alloc/op
- **Winner**: Lock-Free (2.4x faster)

**Key Insight**: In real-world usage with MementoEngine:
- Lock-free provides better overall performance
- Both have similar memory characteristics
- Lock-free's read performance advantage translates to better end-to-end performance

---

## Summary of Results

| Scenario | RWMutex | Lock-Free | Winner | Speedup |
|----------|---------|-----------|--------|---------|
| Concurrent Reads Only | 49.90 ns/op | 1.022 ns/op | Lock-Free | 49x |
| Reads + Writes (10%) | 74.11 ns/op | 36.35 ns/op | Lock-Free | 2.0x |
| Realistic (Rare Writes) | 53.57 ns/op | 1.047 ns/op | Lock-Free | 51x |
| Resize Stress | 142.8 ns/op | 2.950 ns/op | Lock-Free | 48x |
| MementoEngine | 138.4 ns/op | 57.75 ns/op | Lock-Free | 2.4x |

## Key Findings

### Lock-Free Strengths

1. **Optimal Read Performance**: Near-direct memory access speed when reads dominate (49-51x faster)
2. **Fast Write Performance**: Direct in-place modifications (O(1)) - now faster than RWMutex even with 10% writes
3. **No Reader Blocking**: Readers never wait, even during writes or resizes
4. **Production Scenarios**: Best choice for all load balancing scenarios
5. **Resize Resilience**: Maintains read performance during table resizing
6. **Minimal Memory Overhead**: Only resize operations use copy-on-write, writes are O(1)

### RWMutex Strengths

1. **Memory Efficiency**: Slightly lower memory allocation overhead during writes
2. **Predictable Behavior**: Simpler implementation, easier to reason about
3. **Lower Memory Pressure**: No copy-on-write overhead for resize operations

## Recommendations

### Use Lock-Free When:
- **Any workload** (now faster even with 10% writes)
- **Production load balancers** (optimal for all scenarios)
- **Low-latency requirements** for lookups
- **High concurrency** operations
- **Read-heavy workloads** (≥90% reads) - best performance

### Use RWMutex When:
- **Memory-constrained environments** (slightly lower memory overhead)
- **Simpler debugging** requirements
- **Legacy compatibility** (if already using RWMutex version)

## Test Environment

- **CPU**: Intel(R) Xeon(R) Silver 4208 CPU @ 2.10GHz
- **Architecture**: amd64
- **OS**: linux
- **Benchmark Duration**: 3 seconds per test
- **Concurrency**: Parallel execution across multiple goroutines

## Conclusion

The benchmark results demonstrate that **Lock-Free is now the optimal choice for all production load balancing scenarios**:
- **Faster in all scenarios**: Even with 10% writes, Lock-Free is 2x faster than RWMutex
- **Optimal read performance**: 49-51x faster for read-heavy workloads
- **No reader blocking**: Readers never wait, even during resize operations
- **Minimal memory overhead**: With optimized implementation, only resize operations use copy-on-write

**Key Improvement**: The optimized Lock-Free implementation now performs direct in-place modifications for `Remember()` and `Restore()` operations (O(1) instead of O(n)), making it faster than RWMutex even with moderate write ratios.

The **RWMutex version** is now primarily useful for:
- Memory-constrained environments (slightly lower memory overhead)
- Simpler debugging requirements
- Legacy compatibility

**Recommendation**: For new deployments, use the Lock-Free implementation as it provides superior performance across all workload patterns.

