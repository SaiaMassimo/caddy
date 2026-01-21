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

package memento

import (
	"fmt"
	"testing"
)

// TestWCE_Distribution verifies that keys are distributed according to weights.
func TestWCE_Distribution(t *testing.T) {
	engine := NewWeightedConsistentEngine()
	nodesWithWeights := map[string]int{
		"node1": 10,
		"node2": 1,
		"node3": 1,
		"node4": 5,
		"node5": 3,
	}

	engine.InitCluster(nodesWithWeights)

	const numTestKeys = 10000
	distribution := make(map[string]int)
	for i := 0; i < numTestKeys; i++ {
		key := fmt.Sprintf("192.168.1.%d:%d", i%256, i)
		nodeID, ok := engine.Lookup(key)
		if !ok {
			t.Fatalf("Expected node selection for key %s, but got none", key)
		}
		distribution[nodeID]++
	}

	totalWeight := 0
	for _, w := range nodesWithWeights {
		totalWeight += w
	}

	t.Logf("Distribution results for %d keys:", numTestKeys)
	for nodeID, weight := range nodesWithWeights {
		count := distribution[nodeID]
		expectedRatio := float64(weight) / float64(totalWeight)
		actualRatio := float64(count) / float64(numTestKeys)

		tolerance := 0.05 // Allow for a 5% tolerance
		if actualRatio < (expectedRatio-tolerance) || actualRatio > (expectedRatio+tolerance) {
			t.Errorf("Node %s (Weight %d): Expected ratio around %.3f, got %.3f (Count: %d)",
				nodeID, weight, expectedRatio, actualRatio, count)
		} else {
			t.Logf("Node %s (Weight %d): Ratio %.3f (Count: %d) - OK",
				nodeID, weight, actualRatio, count)
		}
	}
}

// TestWCE_Removal verifies correct remapping after a node is removed.
func TestWCE_Removal(t *testing.T) {
	engine := NewWeightedConsistentEngine()
	nodesWithWeights := map[string]int{
		"nodeA": 10,
		"nodeB": 2,
		"nodeC": 8,
	}
	engine.InitCluster(nodesWithWeights)

	nodeToRemove := "nodeB"
	var keyMappedToRemovedNode string

	// Find a key that maps to the node we will remove
	for i := 0; i < 5000; i++ {
		key := fmt.Sprintf("10.0.0.%d", i)
		nodeID, _ := engine.Lookup(key)
		if nodeID == nodeToRemove {
			keyMappedToRemovedNode = key
			break
		}
	}
	if keyMappedToRemovedNode == "" {
		t.Fatalf("Could not find a key that maps to the node being removed. Cannot proceed.")
	}

	// Remove the node
	engine.RemoveNode(nodeToRemove)

	// Verify the key is remapped to a different, active node
	newNodeID, ok := engine.Lookup(keyMappedToRemovedNode)

	if !ok {
		t.Fatalf("Key %s was not remapped to any node after removal.", keyMappedToRemovedNode)
	}
	if newNodeID == nodeToRemove {
		t.Fatalf("Key %s is still mapped to the removed node %s.", keyMappedToRemovedNode, nodeToRemove)
	}
	if newNodeID != "nodeA" && newNodeID != "nodeC" {
		t.Errorf("Key %s was remapped to an unexpected node: %s", keyMappedToRemovedNode, newNodeID)
	} else {
		t.Logf("Key %s successfully remapped from %s to %s.", keyMappedToRemovedNode, nodeToRemove, newNodeID)
	}
}

// TestWCE_RemovalAndRestore verifies that mappings are restored after a node is brought back online.
func TestWCE_RemovalAndRestore(t *testing.T) {
	engine := NewWeightedConsistentEngine()
	nodesWithWeights := map[string]int{
		"host1": 5,
		"host2": 8,
		"host3": 3,
	}
	engine.InitCluster(nodesWithWeights)

	const numTestKeys = 500
	initialMappings := make(map[string]string)
	for i := 0; i < numTestKeys; i++ {
		key := fmt.Sprintf("172.16.1.%d", i)
		nodeID, ok := engine.Lookup(key)
		if !ok {
			t.Fatalf("Initial lookup failed for key %s", key)
		}
		initialMappings[key] = nodeID
	}

	// Remove a node
	nodeToRemove := "host2"
	weightOfRemovedNode := nodesWithWeights[nodeToRemove]
	engine.RemoveNode(nodeToRemove)

	// Restore the node
	engine.AddNode(nodeToRemove, weightOfRemovedNode)

	// Verify that all mappings have been restored to their original state
	restorationFailures := 0
	for key, originalNodeID := range initialMappings {
		currentNodeID, ok := engine.Lookup(key)
		if !ok {
			t.Errorf("Key %s failed to select any node after restoration.", key)
			restorationFailures++
			continue
		}
		if currentNodeID != originalNodeID {
			t.Errorf("Key %s: Mapping not restored. Expected %s, got %s.", key, originalNodeID, currentNodeID)
			restorationFailures++
		}
	}

	if restorationFailures > 0 {
		t.Fatalf("Restoration failed for %d out of %d keys.", restorationFailures, numTestKeys)
	} else {
		t.Logf("Successfully restored all %d key mappings.", numTestKeys)
	}
}

// TestWCE_Monotonicity verifies that when a new node is added,
// keys either stay on their current node or move to the new node.
func TestWCE_Monotonicity(t *testing.T) {
	engine := NewWeightedConsistentEngine()
	nodesWithWeights := map[string]int{
		"node1": 10,
		"node2": 20,
		"node3": 5,
	}
	engine.InitCluster(nodesWithWeights)

	const numKeys = 10000
	mappaOld := make(map[string]string, numKeys)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("monotonicity-key-%d", i)
		nodeID, _ := engine.Lookup(key)
		mappaOld[key] = nodeID
	}

	// Add a new node
	newNodeID := "node4"
	engine.AddNode(newNodeID, 15)

	violations := 0
	for key, oldNodeID := range mappaOld {
		newNode, ok := engine.Lookup(key)
		if !ok {
			t.Errorf("Monotonicity violation: key %s failed to map to any node after addition", key)
			violations++
			continue
		}

		if newNode != oldNodeID && newNode != newNodeID {
			violations++
			t.Errorf("Monotonicity violation: key %s moved from %s to %s (expected %s or %s)",
				key, oldNodeID, newNode, oldNodeID, newNodeID)
		}
	}

	if violations > 0 {
		t.Fatalf("Monotonicity property violated for %d keys", violations)
	} else {
		t.Logf("Monotonicity property maintained for all %d keys after node addition.", numKeys)
	}
}

// TestWCE_MinimalDisruption verifies that when a node is removed,
// only keys that were on that node are remapped.
func TestWCE_MinimalDisruption(t *testing.T) {
	engine := NewWeightedConsistentEngine()
	nodesWithWeights := map[string]int{
		"nodeA": 10,
		"nodeB": 2,
		"nodeC": 8,
	}
	engine.InitCluster(nodesWithWeights)

	const numKeys = 10000
	mappaOld := make(map[string]string, numKeys)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("disruption-key-%d", i)
		nodeID, _ := engine.Lookup(key)
		mappaOld[key] = nodeID
	}

	// Remove a node
	nodeToRemove := "nodeB"
	engine.RemoveNode(nodeToRemove)

	violations := 0
	for key, oldNodeID := range mappaOld {
		if oldNodeID == nodeToRemove {
			continue // This key is expected to be remapped.
		}

		newNode, ok := engine.Lookup(key)
		if !ok {
			t.Errorf("Minimal Disruption violation: key %s failed to map after removal", key)
			violations++
			continue
		}

		if newNode != oldNodeID {
			violations++
			t.Errorf("Minimal Disruption violation: key %s moved from %s to %s (was not on removed node %s)",
				key, oldNodeID, newNode, nodeToRemove)
		}
	}

	if violations > 0 {
		t.Fatalf("Minimal Disruption property violated for %d keys", violations)
	} else {
		t.Logf("Minimal Disruption property maintained for all keys not on the removed node.")
	}
}

// TestWCE_LoadBalancing verifies the fairness of key distribution according to weights.
func TestWCE_LoadBalancing(t *testing.T) {
	engine := NewWeightedConsistentEngine()
	nodesWithWeights := map[string]int{
		"node_w50": 50,
		"node_w30": 30,
		"node_w20": 20,
	}
	engine.InitCluster(nodesWithWeights)

	const numKeys = 100000
	distribution := make(map[string]int)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("balance-key-%d", i)
		nodeID, _ := engine.Lookup(key)
		distribution[nodeID]++
	}

	totalWeight := 0
	for _, w := range nodesWithWeights {
		totalWeight += w
	}

	t.Logf("Load balancing results for %d keys:", numKeys)
	maxDeviation := 0.0
	for nodeID, weight := range nodesWithWeights {
		count := distribution[nodeID]
		expectedCount := float64(weight) / float64(totalWeight) * float64(numKeys)
		deviation := (float64(count) - expectedCount) / expectedCount * 100

		if deviation < 0 {
			deviation = -deviation
		}
		if deviation > maxDeviation {
			maxDeviation = deviation
		}

		t.Logf("Node %s (Weight %d): %d keys (Expected: %.0f, Deviation: %.2f%%)",
			nodeID, weight, count, expectedCount, deviation)
	}

	// Allow a maximum deviation of 15% from the expected value for any node.
	// This is a simple way to check for severe imbalances.
	const tolerance = 15.0
	if maxDeviation > tolerance {
		t.Errorf("Load balancing deviation exceeds tolerance of %.2f%%. Max deviation was %.2f%%.",
			tolerance, maxDeviation)
	} else {
		t.Logf("Maximum load deviation (%.2f%%) is within the %.2f%% tolerance.", maxDeviation, tolerance)
	}
}
