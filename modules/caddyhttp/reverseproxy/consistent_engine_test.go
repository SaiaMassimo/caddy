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
	"fmt"
	"math"
	"math/rand"
	"testing"
)

func makeUpstream(id string) *Upstream {
	up := &Upstream{Host: new(Host), Dial: id}
	up.setHealthy(true)
	return up
}

func TestConsistentEngine(t *testing.T) {
	consistentEngine := NewConsistentEngine()

	if consistentEngine.Size() != 0 {
		t.Errorf("Expected size 0, got %d", consistentEngine.Size())
	}

	upstreams := make([]*Upstream, 0, 5)
	for i := 0; i < 5; i++ {
		up := makeUpstream(fmt.Sprintf("node%d", i))
		upstreams = append(upstreams, up)
		consistentEngine.AddNode(up)
	}

	if consistentEngine.Size() != 5 {
		t.Errorf("Expected size 5, got %d", consistentEngine.Size())
	}

	upstream := consistentEngine.GetBucket("test-key")
	if upstream == nil {
		t.Fatal("Expected non-nil upstream for key")
	}
	found := false
	for _, up := range upstreams {
		if up == upstream {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Upstream %s not found in initial upstream list", upstream.String())
	}

	topology := consistentEngine.GetTopology()
	if len(topology) != 5 {
		t.Errorf("Expected topology size 5, got %d", len(topology))
	}
}

func TestConsistentEngineNodeOperations(t *testing.T) {
	consistentEngine := NewConsistentEngine()

	up1 := makeUpstream("node1")
	up2 := makeUpstream("node2")
	up3 := makeUpstream("node3")

	consistentEngine.AddNode(up1)
	consistentEngine.AddNode(up2)
	consistentEngine.AddNode(up3)

	topology := consistentEngine.GetTopology()
	if len(topology) != 3 {
		t.Errorf("Expected topology size 3, got %d", len(topology))
	}

	consistentEngine.RemoveNode(up2)

	topology = consistentEngine.GetTopology()
	if len(topology) != 2 {
		t.Errorf("Expected topology size 2, got %d", len(topology))
	}

	found := false
	for _, up := range topology {
		if up == up2 {
			found = true
			break
		}
	}
	if found {
		t.Error("node2 should not be in topology after removal")
	}

	consistentEngine.RestoreNode(up2)

	topology = consistentEngine.GetTopology()
	if len(topology) != 3 {
		t.Errorf("Expected topology size 3 after restore, got %d", len(topology))
	}
}

func TestConsistentEngineConsistency(t *testing.T) {
	consistentEngine := NewConsistentEngine()

	upstreams := make([]*Upstream, 0, 5)
	for i := 0; i < 5; i++ {
		up := makeUpstream(fmt.Sprintf("node%d", i))
		upstreams = append(upstreams, up)
		consistentEngine.AddNode(up)
	}

	key := "consistent-test-key"
	upstream1 := consistentEngine.GetBucket(key)
	upstream2 := consistentEngine.GetBucket(key)

	if upstream1 == nil || upstream2 == nil {
		t.Fatal("Expected non-nil upstream for key")
	}
	if upstream1 != upstream2 {
		t.Errorf("Inconsistent mapping: %s vs %s", upstream1.String(), upstream2.String())
	}

	consistentEngine.RemoveNode(upstreams[2])

	upstream3 := consistentEngine.GetBucket(key)
	upstream4 := consistentEngine.GetBucket(key)

	if upstream3 == nil || upstream4 == nil {
		t.Fatal("Expected non-nil upstream for key after removal")
	}
	if upstream3 != upstream4 {
		t.Errorf("Inconsistent mapping after removal: %s vs %s", upstream3.String(), upstream4.String())
	}
}

func TestConsistentEngineLoadBalancing(t *testing.T) {
	const N = 50
	const K = 100000

	consistentEngine := NewConsistentEngine()
	upstreams := make([]*Upstream, 0, N)

	for i := 0; i < N; i++ {
		up := makeUpstream(fmt.Sprintf("node%d", i))
		upstreams = append(upstreams, up)
		if err := consistentEngine.AddNode(up); err != nil {
			t.Fatalf("Failed to add node %s: %v", up.Dial, err)
		}
	}

	if consistentEngine.Size() != N {
		t.Fatalf("Expected engine size %d, got %d", N, consistentEngine.Size())
	}

	nodeCounts := make(map[string]int, N)
	for i := 0; i < K; i++ {
		key := fmt.Sprintf("consistent-key-%d", i)
		up := consistentEngine.GetBucket(key)
		if up == nil {
			t.Fatalf("Invalid upstream for key %s", key)
		}
		nodeCounts[up.String()]++
	}

	if len(nodeCounts) != N {
		t.Errorf("Expected %d nodes to be used, got %d", N, len(nodeCounts))
	}

	counts := make([]int, N)
	for i := 0; i < N; i++ {
		id := fmt.Sprintf("node%d", i)
		counts[i] = nodeCounts[id]
	}

	mu := float64(K) / float64(N)
	p := 1.0 / float64(N)
	sigma := math.Sqrt(float64(K) * p * (1.0 - p))

	mean := 0.0
	for _, count := range counts {
		mean += float64(count)
	}
	mean /= float64(N)

	variance := 0.0
	for _, count := range counts {
		diff := float64(count) - mean
		variance += diff * diff
	}
	variance /= float64(N)
	stdDev := math.Sqrt(variance)

	CV := stdDev / mean
	CVatteso := math.Sqrt((float64(N) - 1.0) / float64(K))
	CVmax := CVatteso * 1.2

	t.Logf("Distribution Test (N=%d, K=%d):", N, K)
	t.Logf("  Expected per node (μ): %.2f", mu)
	t.Logf("  Expected std dev (σ): %.2f", sigma)
	t.Logf("  Observed mean: %.2f", mean)
	t.Logf("  Observed std dev: %.2f", stdDev)
	t.Logf("  Coefficient of Variation (CV): %.6f", CV)
	t.Logf("  Expected CV: %.6f", CVatteso)
	t.Logf("  Max allowed CV (CV_atteso + 20%%): %.6f", CVmax)

	if CV > CVmax {
		t.Errorf("Coefficient of Variation too high: %.6f > %.6f (expected CV: %.6f, margin: +20%%)",
			CV, CVmax, CVatteso)
	}
}

func TestMementoLoadBalancing(t *testing.T) {
	const numNodes = 50
	const numKeys = 100000

	consistentEngine := NewConsistentEngine()

	upstreams := make([]*Upstream, 0, numNodes)
	for i := 0; i < numNodes; i++ {
		up := makeUpstream(fmt.Sprintf("node%d", i))
		upstreams = append(upstreams, up)
		consistentEngine.AddNode(up)
	}

	if consistentEngine.Size() != numNodes {
		t.Fatalf("Expected engine size %d, got %d", numNodes, consistentEngine.Size())
	}

	distributionBefore := make([]int, numNodes)
	keyToUpstream := make(map[string]*Upstream, numKeys)
	indexByUpstream := make(map[*Upstream]int, numNodes)
	for i, up := range upstreams {
		indexByUpstream[up] = i
	}

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("memento-key-%d", i)
		up := consistentEngine.GetBucket(key)
		if up == nil {
			t.Fatalf("Invalid upstream for key %s", key)
		}
		index, ok := indexByUpstream[up]
		if !ok {
			t.Fatalf("Upstream %s not found in index map", up.String())
		}
		distributionBefore[index]++
		keyToUpstream[key] = up
	}

	meanBefore := float64(numKeys) / float64(numNodes)
	t.Logf("Distribution BEFORE node removal:")
	t.Logf("  Mean keys per node: %.2f", meanBefore)
	t.Logf("  Nodes: %d", numNodes)

	rand.Seed(42)
	randomNodeIndex := rand.Intn(numNodes)
	removedUpstream := upstreams[randomNodeIndex]

	t.Logf("Removing random node: %s (index: %d)", removedUpstream.Dial, randomNodeIndex)

	keysOnRemovedNode := distributionBefore[randomNodeIndex]
	t.Logf("  Keys on removed node: %d", keysOnRemovedNode)

	consistentEngine.RemoveNode(removedUpstream)

	stats := consistentEngine.GetMementoStats()
	if stats["memento_empty"].(bool) {
		t.Error("Memento should not be empty after node removal")
	}
	if stats["memento_size"].(int) != 1 {
		t.Errorf("Expected memento size 1, got %d", stats["memento_size"].(int))
	}

	distributionAfter := make([]int, numNodes)
	keysMoved := 0

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("memento-key-%d", i)
		upAfter := consistentEngine.GetBucket(key)
		upBefore := keyToUpstream[key]

		if upAfter == nil {
			t.Errorf("Invalid upstream for key %s after removal", key)
			continue
		}
		if upAfter == removedUpstream {
			t.Errorf("Key %s was still mapped to removed upstream %s", key, removedUpstream.Dial)
		}

		indexAfter, ok := indexByUpstream[upAfter]
		if !ok {
			t.Errorf("Upstream %s not found in index map after removal", upAfter.String())
			continue
		}

		distributionAfter[indexAfter]++
		if upBefore != upAfter {
			keysMoved++
		}
	}

	totalKeysAfter := 0
	minKeysAfter := numKeys
	maxKeysAfter := 0
	nonZeroNodesAfter := 0

	for i := 0; i < numNodes; i++ {
		if i == randomNodeIndex {
			continue
		}
		count := distributionAfter[i]
		totalKeysAfter += count
		if count > 0 {
			nonZeroNodesAfter++
		}
		if count < minKeysAfter {
			minKeysAfter = count
		}
		if count > maxKeysAfter {
			maxKeysAfter = count
		}
	}

	meanAfter := float64(totalKeysAfter) / float64(numNodes-1)

	varianceAfter := 0.0
	for i := 0; i < numNodes; i++ {
		if i == randomNodeIndex {
			continue
		}
		diff := float64(distributionAfter[i]) - meanAfter
		varianceAfter += diff * diff
	}
	varianceAfter /= float64(numNodes - 1)
	stdDevAfter := math.Sqrt(varianceAfter)
	coefficientOfVariationAfter := stdDevAfter / meanAfter

	t.Logf("\nDistribution AFTER node removal:")
	t.Logf("  Nodes remaining: %d", numNodes-1)
	t.Logf("  Total keys: %d", totalKeysAfter)
	t.Logf("  Mean keys per node: %.2f", meanAfter)
	t.Logf("  Min keys per node: %d", minKeysAfter)
	t.Logf("  Max keys per node: %d", maxKeysAfter)
	t.Logf("  Nodes with keys: %d", nonZeroNodesAfter)
	t.Logf("  Standard deviation: %.2f", stdDevAfter)
	t.Logf("  Coefficient of variation: %.4f", coefficientOfVariationAfter)
	t.Logf("  Keys that moved: %d (%.2f%%)", keysMoved, float64(keysMoved)/float64(numKeys)*100)
	t.Logf("  Keys on removed node (should be moved): %d", keysOnRemovedNode)

	minNodesPercent := float64(numNodes-1) * 0.95
	expectedMinNodesWithKeys := int(minNodesPercent + 0.5)
	if nonZeroNodesAfter < expectedMinNodesWithKeys {
		t.Errorf("Expected at least %d nodes with keys after removal, got %d",
			expectedMinNodesWithKeys, nonZeroNodesAfter)
	}

	if coefficientOfVariationAfter > 0.6 {
		t.Errorf("Coefficient of variation too high after removal: %.4f (expected < 0.6)",
			coefficientOfVariationAfter)
	}

	maxExpectedKeys := int(meanAfter * 3.5)
	if maxKeysAfter > maxExpectedKeys {
		t.Errorf("Max keys per node (%d) exceeds 3.5x average (%.1f)",
			maxKeysAfter, meanAfter*3.5)
	}

	if totalKeysAfter != numKeys {
		t.Errorf("Total keys mismatch after removal: expected %d, got %d",
			numKeys, totalKeysAfter)
	}

	if keysMoved < keysOnRemovedNode {
		t.Errorf("Expected at least %d keys moved, got %d",
			keysOnRemovedNode, keysMoved)
	}
}

func TestConsistentEngineLoadBalancingLockFree(t *testing.T) {
	const N = 50
	const K = 100000

	consistentEngine := NewConsistentEngineWithType(true)
	upstreams := make([]*Upstream, 0, N)

	for i := 0; i < N; i++ {
		up := makeUpstream(fmt.Sprintf("node%d", i))
		upstreams = append(upstreams, up)
		if err := consistentEngine.AddNode(up); err != nil {
			t.Fatalf("Failed to add node %s: %v", up.Dial, err)
		}
	}

	if consistentEngine.Size() != N {
		t.Fatalf("Expected engine size %d, got %d", N, consistentEngine.Size())
	}

	nodeCounts := make(map[string]int, N)
	for i := 0; i < K; i++ {
		key := fmt.Sprintf("consistent-key-%d", i)
		up := consistentEngine.GetBucket(key)
		if up == nil {
			t.Fatalf("Invalid upstream for key %s", key)
		}
		nodeCounts[up.String()]++
	}

	if len(nodeCounts) != N {
		t.Errorf("Expected %d nodes to be used, got %d", N, len(nodeCounts))
	}

	counts := make([]int, N)
	for i := 0; i < N; i++ {
		id := fmt.Sprintf("node%d", i)
		counts[i] = nodeCounts[id]
	}

	mu := float64(K) / float64(N)
	p := 1.0 / float64(N)
	sigma := math.Sqrt(float64(K) * p * (1.0 - p))

	mean := 0.0
	for _, count := range counts {
		mean += float64(count)
	}
	mean /= float64(N)

	variance := 0.0
	for _, count := range counts {
		diff := float64(count) - mean
		variance += diff * diff
	}
	variance /= float64(N)
	stdDev := math.Sqrt(variance)

	CV := stdDev / mean
	CVatteso := math.Sqrt((float64(N) - 1.0) / float64(K))
	CVmax := CVatteso * 1.2

	t.Logf("Distribution Test (N=%d, K=%d):", N, K)
	t.Logf("  Expected per node (μ): %.2f", mu)
	t.Logf("  Expected std dev (σ): %.2f", sigma)
	t.Logf("  Observed mean: %.2f", mean)
	t.Logf("  Observed std dev: %.2f", stdDev)
	t.Logf("  Coefficient of Variation (CV): %.6f", CV)
	t.Logf("  Expected CV: %.6f", CVatteso)
	t.Logf("  Max allowed CV (CV_atteso + 20%%): %.6f", CVmax)

	if CV > CVmax {
		t.Errorf("Coefficient of Variation too high: %.6f > %.6f (expected CV: %.6f, margin: +20%%)",
			CV, CVmax, CVatteso)
	}
}

func BenchmarkConsistentEngineGetBucket(b *testing.B) {
	consistentEngine := NewConsistentEngine()

	for i := 0; i < 100; i++ {
		consistentEngine.AddNode(makeUpstream(fmt.Sprintf("node%d", i)))
	}

	key := "benchmark-key"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = consistentEngine.GetBucket(key)
	}
}
