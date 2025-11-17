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
	"math"
	"math/rand"
	"testing"
)

// TestMementoEngineMonotonicity verifica la monotonicità usando MementoEngine
func TestMementoEngineMonotonicity(t *testing.T) {
	const initialNodes = 50
	const numKeys = 100000

	t.Run("Monotonicity_On_Removal", func(t *testing.T) {
		// Setup iniziale: S = 50 nodi con MementoEngine
		engine := NewMementoEngine(initialNodes)

		// Genera 100k chiavi e calcola mappa_old[k]
		keyToBucketOld := make(map[string]int, numKeys)
		for i := 0; i < numKeys; i++ {
			key := fmt.Sprintf("memento-key-%d", i)
			keyToBucketOld[key] = engine.GetBucket(key)
		}

		// Rimuovi un nodo casuale x
		rand.Seed(42)
		removedNodeIndex := rand.Intn(initialNodes)
		t.Logf("Removing node: %d", removedNodeIndex)

		// Calcola mappa_new su S' = S \ {x}
		engine.RemoveBucket(removedNodeIndex)

		// Verifica monotonicità su rimozione
		// ASSERZIONE: per ogni k con mappa_old[k] ≠ x deve valere mappa_new[k] = mappa_old[k]
		monotonicityViolations := 0
		keysOnRemovedNode := 0

		for key, bucketOld := range keyToBucketOld {
			bucketNew := engine.GetBucket(key)

			if bucketOld == removedNodeIndex {
				keysOnRemovedNode++
				// Verifica che sia stata reindirizzata e non punti ancora al nodo rimosso
				if bucketNew == removedNodeIndex {
					t.Errorf("Key %s still mapped to removed bucket %d", key, bucketNew)
				}
			} else {
				// Se NON era sul nodo rimosso, DEVE rimanere sullo stesso bucket
				if bucketOld != bucketNew {
					monotonicityViolations++
				}
			}
		}

		violationRate := float64(monotonicityViolations) / float64(numKeys) * 100
		t.Logf("MementoEngine Monotonicity on removal stats:")
		t.Logf("  Keys on removed node: %d", keysOnRemovedNode)
		t.Logf("  Violations: %d/%d (%.4f%%)", monotonicityViolations, numKeys, violationRate)

		// MementoEngine dovrebbe avere violazioni molto basse (< 5%)
		if violationRate > 5.0 {
			t.Errorf("Too many monotonicity violations: %.4f%% (expected < 5%%)",
				violationRate)
		} else {
			t.Logf("  MementoEngine introduced %.4f%% monotonicity violations (acceptable)",
				violationRate)
		}
	})

	t.Run("Monotonicity_On_Addition", func(t *testing.T) {
		// Setup iniziale: S = 50 nodi
		engine := NewMementoEngine(initialNodes)

		// Genera 100k chiavi e calcola mappa_old[k]
		keyToBucketOld := make(map[string]int, numKeys)
		for i := 0; i < numKeys; i++ {
			key := fmt.Sprintf("memento-key-%d", i)
			keyToBucketOld[key] = engine.GetBucket(key)
		}

		// Aggiungi un nuovo nodo
		newNodeIndex := engine.AddBucket()
		t.Logf("Added new node: %d", newNodeIndex)

		// Verifica monotonicità su aggiunta
		// ASSERZIONE: per ogni k, se mappa_new[k] ≠ mappa_old[k],
		// allora mappa_new[k] DEVE essere il nuovo nodo
		monotonicityViolations := 0
		keysMovedToNewNode := 0
		keysMovedToOldNode := 0

		for key, bucketOld := range keyToBucketOld {
			bucketNew := engine.GetBucket(key)

			if bucketOld != bucketNew {
				keysMovedToNewNode++
				if bucketNew != newNodeIndex {
					// Se la chiave si è spostata ma non sul nuovo nodo, è una violazione
					monotonicityViolations++
					keysMovedToOldNode++
				}
			}
		}

		expectedKeysMoving := float64(numKeys) / float64(initialNodes+1)
		violationRate := float64(monotonicityViolations) / float64(numKeys) * 100

		t.Logf("MementoEngine Monotonicity on addition stats:")
		t.Logf("  Keys moved to new node: %d (expected: ~%.0f)",
			keysMovedToNewNode, expectedKeysMoving)
		t.Logf("  Keys stayed on old nodes: %d", numKeys-keysMovedToNewNode)
		t.Logf("  Keys incorrectly moved to old node: %d", keysMovedToOldNode)
		t.Logf("  Violations: %d/%d (%.4f%% enormi)",
			monotonicityViolations, numKeys, violationRate)

		// MementoEngine dovrebbe avere violazioni molto basse sull'aggiunta
		if violationRate > 1.0 {
			t.Errorf("Too many monotonicity violations on addition: %.4f%% (expected < 1%%)",
				violationRate)
		}

		// Verifica che il numero di chiavi mosse sia ragionevole (circa 1/(N+1) delle chiavi)
		lowerBound := int(expectedKeysMoving * 0.7)
		upperBound := int(expectedKeysMoving * 1.3)
		if keysMovedToNewNode < lowerBound || keysMovedToNewNode > upperBound {
			t.Logf("Warning: Keys moved to new node (%d) outside expected range [%d, %d]",
				keysMovedToNewNode, lowerBound, upperBound)
		}
	})
}

// TestMementoEngineSequentialAdditions verifica la monotonicità durante aggiunte sequenziali
// Genera una catena di insiemi S0 ⊂ S1 ⊂ … ⊂ St aggiungendo un nodo alla volta
// Verifica: per tutte le k, "o restano sullo stesso nodo, o finiscono sul nodo aggiunto"
func TestMementoEngineSequentialAdditions(t *testing.T) {
	const initialNodes = 10
	const numSteps = 20
	const numKeys = 100000

	// Crea engine iniziale con 10 nodi
	engine := NewMementoEngine(initialNodes)

	// Genera chiavi iniziali e calcola mapping
	keyToBucketHistory := make(map[string][]int)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("seq-key-%d", i)
		bucket := engine.GetBucket(key)
		keyToBucketHistory[key] = []int{bucket}
	}

	// Aggiungi nodi uno alla volta e verifica monotonicità
	violations := 0
	totalKeyMovements := 0

	for step := 0; step < numSteps; step++ {
		// Aggiungi un nuovo nodo
		newNodeIndex := engine.AddBucket()
		t.Logf("Step %d: Added node %d", step+1, newNodeIndex)

		// Verifica tutte le chiavi
		for key, bucketHistory := range keyToBucketHistory {
			oldBucket := bucketHistory[len(bucketHistory)-1]
			newBucket := engine.GetBucket(key)
			bucketHistory = append(bucketHistory, newBucket)
			keyToBucketHistory[key] = bucketHistory

			// Monotonicità: chiave deve restare sullo stesso nodo O spostarsi sul nuovo nodo
			if newBucket != oldBucket && newBucket != newNodeIndex {
				violations++
				t.Errorf("Violation at step %d: key %s moved from %d to %d (expected %d or %d)",
					step+1, key, oldBucket, newBucket, oldBucket, newNodeIndex)
			}

			if newBucket != oldBucket {
				totalKeyMovements++
			}
		}
	}

	violationRate := float64(violations) / float64(numKeys*numSteps) * 100
	t.Logf("\nSequential additions stats:")
	t.Logf("  Total violations: %d", violations)
	t.Logf("  Violation rate: %.4f%%", violationRate)
	t.Logf("  Total key movements: %d", totalKeyMovements)

	if violations > 0 {
		t.Fatalf("Too many monotonicity violations: %d (expected 0)", violations)
	}
}

func TestMementoEngineMinimalDisruption(t *testing.T) {
	// Test Minimal Disruption property:
	// When removing node x: S' = S \ {x}
	// For all keys k: if mappa_old[k] ≠ x => mappa_new[k] = mappa_old[k]
	// Only keys mapped to the removed node should be remapped

	const initialNodes = 50
	const numKeys = 10000

	// Setup: create engine with initialNodes nodes
	engine := NewMementoEngine(initialNodes)

	// Generate test keys and map them to buckets (mappa_old)
	mappaOld := make(map[string]int, numKeys)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("memento-minimal-disruption-key-%d", i)
		mappaOld[key] = engine.GetBucket(key)
	}

	// Remove a random node x
	removedNode := 25 // Fixed node for reproducibility
	engine.RemoveBucket(removedNode)

	// Verify Minimal Disruption property
	violations := 0
	for key, oldBucket := range mappaOld {
		newBucket := engine.GetBucket(key)

		// Minimal Disruption: if oldBucket ≠ removedNode, then newBucket = oldBucket
		if oldBucket != removedNode {
			if newBucket != oldBucket {
				violations++
				t.Errorf("Minimal Disruption violation: key %s moved from bucket %d to %d (was not on removed node %d)",
					key, oldBucket, newBucket, removedNode)
			}
		}
		// If oldBucket == removedNode, the key is allowed to be remapped (OK)
	}

	if violations > 0 {
		t.Fatalf("Minimal Disruption property violated: %d keys incorrectly remapped", violations)
	}

	t.Logf("Minimal Disruption verified: all %d keys that were not on removed node %d remained unchanged", numKeys, removedNode)
}

func TestMementoEngineTwoRemovals(t *testing.T) {
	const initialNodes = 50
	const numKeys = 10000

	// Setup: 50 nodi
	engine := NewMementoEngine(initialNodes)

	// Mappa iniziale
	keyToBucketBefore := make(map[string]int, numKeys)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("two-removals-key-%d", i)
		keyToBucketBefore[key] = engine.GetBucket(key)
	}

	// Prima rimozione
	removedNode1 := 10
	engine.RemoveBucket(removedNode1)
	t.Logf("Removed node: %d", removedNode1)

	// Mappa dopo prima rimozione
	keyToBucketAfterFirst := make(map[string]int, numKeys)
	violationsFirst := 0
	for key, bucketBefore := range keyToBucketBefore {
		bucketAfterFirst := engine.GetBucket(key)
		keyToBucketAfterFirst[key] = bucketAfterFirst

		if bucketBefore == removedNode1 {
			// OK, deve essere reindirizzata
		} else if bucketBefore != bucketAfterFirst {
			violationsFirst++
		}
	}

	t.Logf("First removal: %d violations", violationsFirst)

	// Seconda rimozione
	removedNode2 := 25
	engine.RemoveBucket(removedNode2)
	t.Logf("Removed node: %d", removedNode2)

	// Verifica monotonicità
	violationsSecond := 0
	for key, bucketAfterFirst := range keyToBucketAfterFirst {
		bucketAfterSecond := engine.GetBucket(key)

		if bucketAfterFirst == removedNode2 {
			// OK, deve essere reindirizzata
		} else if bucketAfterFirst != bucketAfterSecond {
			violationsSecond++
			t.Errorf("Key %s moved from %d to %d (was not on removed node %d)",
				key, bucketAfterFirst, bucketAfterSecond, removedNode2)
		}
	}

	t.Logf("Second removal: %d violations (%.2f%%)",
		violationsSecond, float64(violationsSecond)/float64(numKeys)*100)

	if violationsFirst > 0 {
		t.Fatalf("First removal has violations: %d", violationsFirst)
	}
	if violationsSecond > 0 {
		t.Fatalf("Second removal has violations: %d", violationsSecond)
	}
}

// TestMementoEnginePropertyBased verifica invarianti usando property-based testing
// Genera casualmente: nodi iniziali, sequenza di operazioni add/remove, chiavi
func TestMementoEnginePropertyBased(t *testing.T) {
	// Scenari hardcoded per debugging
	scenarios := []struct {
		name         string
		initialNodes int
		numKeys      int
		operations   []struct {
			opType    string // "ADD" or "REMOVE"
			nodeIndex int    // per REMOVE, l'indice del nodo da rimuovere
		}
	}{
		{
			name:         "Simple_two_removals",
			initialNodes: 50,
			numKeys:      1000,
			operations: []struct {
				opType    string
				nodeIndex int
			}{
				{"REMOVE", 10},
				{"REMOVE", 25},
			},
		},
		{
			name:         "Add_then_remove",
			initialNodes: 20,
			numKeys:      500,
			operations: []struct {
				opType    string
				nodeIndex int
			}{
				{"ADD", -1}, // ADD non ha bisogno di indice
				{"REMOVE", 5},
			},
		},
		{
			name:         "Multiple_removals",
			initialNodes: 30,
			numKeys:      800,
			operations: []struct {
				opType    string
				nodeIndex int
			}{
				{"REMOVE", 0},
				{"REMOVE", 5},
				{"REMOVE", 10},
			},
		},
		{
			name:         "Remove_middle_then_end",
			initialNodes: 25,
			numKeys:      600,
			operations: []struct {
				opType    string
				nodeIndex int
			}{
				{"REMOVE", 12},
				{"REMOVE", 20},
			},
		},
		{
			name:         "Add_remove_add",
			initialNodes: 15,
			numKeys:      400,
			operations: []struct {
				opType    string
				nodeIndex int
			}{
				{"ADD", -1},
				{"REMOVE", 7},
				{"ADD", -1},
			},
		},
		{
			name:         "Remove_first",
			initialNodes: 20,
			numKeys:      500,
			operations: []struct {
				opType    string
				nodeIndex int
			}{
				{"REMOVE", 0},
			},
		},
		{
			name:         "Remove_last",
			initialNodes: 20,
			numKeys:      500,
			operations: []struct {
				opType    string
				nodeIndex int
			}{
				{"REMOVE", 19},
			},
		},
		{
			name:         "Complex_sequence",
			initialNodes: 30,
			numKeys:      1000,
			operations: []struct {
				opType    string
				nodeIndex int
			}{
				{"ADD", -1},
				{"REMOVE", 5},
				{"ADD", -1},
				{"REMOVE", 10},
				{"REMOVE", 15},
			},
		},
		{
			name:         "Remove_early_then_late",
			initialNodes: 35,
			numKeys:      700,
			operations: []struct {
				opType    string
				nodeIndex int
			}{
				{"REMOVE", 3},
				{"REMOVE", 30},
			},
		},
		{
			name:         "Three_consecutive_removals",
			initialNodes: 40,
			numKeys:      900,
			operations: []struct {
				opType    string
				nodeIndex int
			}{
				{"REMOVE", 10},
				{"REMOVE", 20},
				{"REMOVE", 30},
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Crea engine
			engine := NewMementoEngine(scenario.initialNodes)

			// Genera chiavi iniziali
			keyToBucketBefore := make(map[string]int)
			for i := 0; i < scenario.numKeys; i++ {
				key := fmt.Sprintf("key-%s-%d", scenario.name, i)
				keyToBucketBefore[key] = engine.GetBucket(key)
			}

			// Track operations for debugging
			var operationSequence []string

			// Esegui sequenza di operazioni hardcoded
			for op, operation := range scenario.operations {
				if operation.opType == "ADD" {
					// OPERAZIONE: ADD
					newNodeIndex := engine.AddBucket()
					operationSequence = append(operationSequence, fmt.Sprintf("ADD(%d)", newNodeIndex))

					// INVARIANTE: chiavi devono restare sullo stesso nodo O spostarsi sul nuovo
					violations := 0
					for key, oldBucket := range keyToBucketBefore {
						newBucket := engine.GetBucket(key)

						if newBucket != oldBucket && newBucket != newNodeIndex {
							violations++
							t.Errorf("%s, Op %d (ADD): key %s moved from %d to %d (violation)",
								scenario.name, op+1, key, oldBucket, newBucket)
						}

						// Aggiorna il mapping
						keyToBucketBefore[key] = newBucket
					}

					if violations > 0 {
						// Log the state that caused the failure
						t.Logf("%s, Op %d (ADD) failed with %d violations", scenario.name, op+1, violations)
						t.Logf("  Operation sequence: %v", operationSequence)
						t.Logf("  Current size: %d, binomial size: %d, memento size: %d",
							engine.Size(), engine.binomialArraySize(), engine.memento.Size())
						t.Fatalf("%s, Op %d (ADD): found %d violations", scenario.name, op+1, violations)
					}
				} else {
					// OPERAZIONE: REMOVE
					currentSize := engine.Size()
					if currentSize <= 1 {
						continue // Skip removal if only 1 node
					}

					removedNodeIndex := operation.nodeIndex
					engine.RemoveBucket(removedNodeIndex)
					operationSequence = append(operationSequence, fmt.Sprintf("REMOVE(%d)", removedNodeIndex))

					// INVARIANTE: chiavi che NON erano sul nodo rimosso must restare immutate
					violations := 0
					for key, oldBucket := range keyToBucketBefore {
						newBucket := engine.GetBucket(key)

						// Se la chiave era sul nodo rimosso, viene reindirizzata (OK)
						if oldBucket == removedNodeIndex {
							// Verifica solo che non punti più al nodo rimosso
							if newBucket == removedNodeIndex {
								t.Errorf("%s, Op %d (REMOVE): key %s still mapped to removed node %d",
									scenario.name, op+1, key, removedNodeIndex)
							}
						} else {
							// Se NON era sul nodo rimosso, DEVE rimanere invariata
							if oldBucket != newBucket {
								violations++
								t.Errorf("%s, Op %d (REMOVE): key %s moved from %d to %d (was not on removed node)",
									scenario.name, op+1, key, oldBucket, newBucket)
							}
						}

						// Aggiorna SEMPRE il mapping (anche se non era sul nodo rimosso)
						keyToBucketBefore[key] = newBucket
					}

					if violations > 0 {
						// Log the state that caused the failure
						t.Logf("%s, Op %d (REMOVE) failed with %d violations", scenario.name, op+1, violations)
						t.Logf("  Operation sequence: %v", operationSequence)
						t.Logf("  Current size: %d, binomial size: %d, memento size: %d",
							engine.Size(), engine.binomialArraySize(), engine.memento.Size())
						t.Logf("  Removed node: %d (out of %d nodes)", removedNodeIndex, currentSize)

						// Print which buckets are currently removed in memento
						for i := 0; i < engine.binomialArraySize(); i++ {
							if replacer := engine.memento.Replacer(i); replacer >= 0 {
								t.Logf("  Bucket %d -> replaced by %d", i, replacer)
							}
						}

						t.Fatalf("%s, Op %d (REMOVE): found %d violations", scenario.name, op+1, violations)
					}
				}
			}

			t.Logf("%s completed successfully (initialNodes=%d, keys=%d, ops=%d)",
				scenario.name, scenario.initialNodes, scenario.numKeys, len(scenario.operations))
		})
	}
}

// TestMementoEngineMinimalDistribution verifica la "minimal distribution"
// per MementoEngine con rimozioni arbitrarie
func TestMementoEngineMinimalDistribution(t *testing.T) {
	t.Run("MinimalDistribution_On_Addition", func(t *testing.T) {
		const numRuns = 5
		const initialNodes = 50
		const numKeys = 100000

		for run := 0; run < numRuns; run++ {
			engine := NewMementoEngine(initialNodes)

			keyToBucketBefore := make(map[string]int, numKeys)
			for i := 0; i < numKeys; i++ {
				key := fmt.Sprintf("memento-min-dist-key-%d-%d", run, i)
				keyToBucketBefore[key] = engine.GetBucket(key)
			}

			newNodeIndex := engine.AddBucket()

			keysMoved := 0
			violations := 0

			for key, oldBucket := range keyToBucketBefore {
				newBucket := engine.GetBucket(key)

				if oldBucket != newBucket {
					keysMoved++
					if newBucket != newNodeIndex {
						violations++
					}
				}
			}

			expectedMoveProbability := 1.0 / float64(initialNodes+1)
			expectedMoved := expectedMoveProbability * float64(numKeys)

			// Verifica frazione: |movedFraction - expectedFraction| < expectedFraction * 0.5
			movedFraction := float64(keysMoved) / float64(numKeys)
			expectedFraction := expectedMoveProbability
			fracDiff := math.Abs(movedFraction - expectedFraction)

			t.Logf("Run %d: MementoEngine minimal distribution on addition", run+1)
			t.Logf("  Keys moved: %d (expected: ~%.0f), violations: %d", keysMoved, expectedMoved, violations)
			t.Logf("  Moved fraction: %.6f (expected: %.6f, diff: %.6f)", movedFraction, expectedFraction, fracDiff)

			if violations > 0 {
				t.Errorf("Run %d: Found %d keys moved to old nodes", run+1, violations)
			}

			// Asserzione: |movedFraction - expectedFraction| < expectedFraction * 0.5
			if fracDiff >= expectedFraction*0.5 {
				t.Errorf("Run %d: Moved fraction diff too high: %.6f >= %.6f", run+1, fracDiff, expectedFraction*0.5)
			}
		}
	})

	t.Run("MinimalDistribution_On_Removal", func(t *testing.T) {
		const numRuns = 5
		const initialNodes = 50
		const numKeys = 100000

		for run := 0; run < numRuns; run++ {
			engine := NewMementoEngine(initialNodes)

			keyToBucketBefore := make(map[string]int, numKeys)
			for i := 0; i < numKeys; i++ {
				key := fmt.Sprintf("memento-min-dist-key-%d-%d", run, i)
				keyToBucketBefore[key] = engine.GetBucket(key)
			}

			removedNodeIndex := run * 10
			if removedNodeIndex >= initialNodes {
				removedNodeIndex = initialNodes - 1
			}
			engine.RemoveBucket(removedNodeIndex)

			keysMoved := 0
			keysOnRemovedNode := 0
			violations := 0

			for key, oldBucket := range keyToBucketBefore {
				newBucket := engine.GetBucket(key)

				if oldBucket == removedNodeIndex {
					keysOnRemovedNode++
					if newBucket == removedNodeIndex {
						violations++
					} else if oldBucket != newBucket {
						keysMoved++
					}
				} else if oldBucket != newBucket {
					violations++
					keysMoved++
				}
			}

			// Calcolo teorico: p* = 1/N (le chiavi sul nodo rimosso)
			expectedMoveProbability := 1.0 / float64(initialNodes)
			expectedMoved := expectedMoveProbability * float64(numKeys)

			// Verifica frazione: |movedFraction - expectedFraction| < expectedFraction * 0.5
			movedFraction := float64(keysMoved) / float64(numKeys)
			expectedFraction := expectedMoveProbability
			fracDiff := math.Abs(movedFraction - expectedFraction)

			t.Logf("Run %d: Removed node %d, violations: %d", run+1, removedNodeIndex, violations)
			t.Logf("  Keys on removed node: %d (expected: ~%.0f)", keysOnRemovedNode, expectedMoved)
			t.Logf("  Keys moved: %d", keysMoved)
			t.Logf("  Moved fraction: %.6f (expected: %.6f, diff: %.6f)", movedFraction, expectedFraction, fracDiff)

			if violations > 0 {
				t.Errorf("Run %d: Found %d keys moved that were not on removed node", run+1, violations)
			}

			// Asserzione: |movedFraction - expectedFraction| < expectedFraction * 0.5
			if fracDiff >= expectedFraction*0.5 {
				t.Errorf("Run %d: Moved fraction diff too high: %.6f >= %.6f", run+1, fracDiff, expectedFraction*0.5)
			}
		}
	})
}

// TestMementoEngineLoadBalancing verifica il bilanciamento delle chiavi tra i nodi
// Calcola: min/max/mean/stdDev dei conteggi, CV, chi-square normalizzato, e violazioni a 5·σ
func TestMementoEngineLoadBalancing(t *testing.T) {
	const N = 50
	const n = 100000

	engine := NewMementoEngine(N)

	counts := make([]int, N)
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("balance-key-%d", i)
		b := engine.GetBucket(key)
		counts[b]++
	}

	// Statistiche di base
	mu := float64(n) / float64(N)
	p := 1.0 / float64(N)
	sigma := math.Sqrt(float64(n) * p * (1.0 - p))

	minCount := counts[0]
	maxCount := counts[0]
	sum := 0
	for _, c := range counts {
		if c < minCount {
			minCount = c
		}
		if c > maxCount {
			maxCount = c
		}
		sum += c
	}
	mean := float64(sum) / float64(N)

	// Deviazione standard osservata tra i conteggi per nodo
	var variance float64
	for _, c := range counts {
		diff := float64(c) - mean
		variance += diff * diff
	}
	variance /= float64(N)
	stdDev := math.Sqrt(variance)
	CV := stdDev / mean

	// Chi-square
	var chiSquare float64
	for _, c := range counts {
		diff := float64(c) - mu
		chiSquare += (diff * diff) / mu
	}
	normalizedChiSquare := chiSquare / float64(N-1)

	// Violazioni: |O_i - mu| > 5·sigma (per nodo, modello binomiale)
	violations := 0
	threshold := 5.0 * sigma
	for _, c := range counts {
		if math.Abs(float64(c)-mu) > threshold {
			violations++
		}
	}

	maxMinRatio := float64(maxCount) / float64(minCount)
	expectedCV := math.Sqrt((float64(N) - 1.0) / float64(n))

	t.Logf("Balance Test (GetBucket):")
	t.Logf("  Nodes: %d", N)
	t.Logf("  Keys: %d", n)
	t.Logf("  Expected per node (mu): %.2f", mu)
	t.Logf("  Standard deviation per node (sigma): %.2f", sigma)
	t.Logf("  Min count: %d", minCount)
	t.Logf("  Max count: %d", maxCount)
	t.Logf("  Mean: %.2f", mean)
	t.Logf("  Observed std dev across nodes: %.2f", stdDev)
	t.Logf("  Coefficient of Variation (CV): %.6f", CV)
	t.Logf("  Max/Min ratio: %.4f", maxMinRatio)
	t.Logf("  Chi-square: %.2f", chiSquare)
	t.Logf("  Normalized Chi-square X^2/(N-1): %.4f", normalizedChiSquare)
	t.Logf("  Violations (|O_i - mu| > 5·sigma): %d", violations)

	// Asserzioni
	if violations != 0 {
		t.Errorf("All nodes should satisfy |O_i - mu| ≤ 5·sigma, found %d violations", violations)
	}
	if CV > expectedCV*1.2 {
		t.Errorf("Coefficient of Variation should be ≤ expectedCV*1.2 (expectedCV=%.6f, margin=+20%%), but was %.6f", expectedCV, CV)
	}
	if maxMinRatio > 1.15 {
		t.Errorf("Max/Min ratio should be ≤ 1.15, but was %.4f", maxMinRatio)
	}
	if normalizedChiSquare >= 3.0 {
		t.Errorf("Normalized Chi-square X^2/(N-1) should be < 3, but was %.4f", normalizedChiSquare)
	}
}
