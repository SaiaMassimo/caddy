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
	"math"
	"testing"
)

// TestMementoEngineMonotonicity verifica la monotonicità usando MementoEngine
// Insieme di nodi S e insieme di chiavi K, funzione f_S(k)
// mappa_old[k] = f_S(k) per tutte le k ∈ K
// Aggiunta (S′ = S ∪ {x})
// mappa_new[k] = f_S′(k) con il nuovo nodo x
// Verifica: mappa_new[k] ≠ mappa_old[k] => mappa_new[k] = x
func TestMementoEngineMonotonicity(t *testing.T) {
	const initialNodes = 50
	const numKeys = 100000

	// Setup iniziale: S = 50 nodi
	engine := NewMementoEngine(initialNodes)

	// Genera chiavi e calcola mappa_old[k] = f_S(k) per tutte le k ∈ K
	mappaOld := make(map[string]int, numKeys)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("memento-key-%d", i)
		mappaOld[key] = engine.GetBucket(key)
	}

	// Aggiunta (S′ = S ∪ {x})
	newNodeIndex := engine.AddBucket()

	// Verifica: mappa_new[k] ≠ mappa_old[k] => mappa_new[k] = x
	violations := 0
	for key, bucketOld := range mappaOld {
		bucketNew := engine.GetBucket(key)

		// Se mappa_new[k] ≠ mappa_old[k], allora mappa_new[k] DEVE essere x
		if bucketNew != bucketOld && bucketNew != newNodeIndex {
			violations++
			t.Errorf("Monotonicity violation: key %s moved from %d to %d (expected %d or %d)",
				key, bucketOld, bucketNew, bucketOld, newNodeIndex)
		}
	}

	if violations > 0 {
		t.Fatalf("Monotonicity property violated: %d keys incorrectly remapped", violations)
	}
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

// TestMementoEngineMinimalDisruption verifica la minimal disruption usando MementoEngine
// Insieme di nodi S e insieme di chiavi K, funzione f_S(k)
// mappa_old[k] = f_S(k) per tutte le k ∈ K
// Rimozione (S′ = S \ {x})
// Verifica: mappa_old[k] ≠ x => mappa_new[k] = mappa_old[k]
func TestMementoEngineMinimalDisruption(t *testing.T) {
	const initialNodes = 50
	const numKeys = 100000

	// Setup: insieme di nodi S
	engine := NewMementoEngine(initialNodes)

	// Genera chiavi e calcola mappa_old[k] = f_S(k) per tutte le k ∈ K
	mappaOld := make(map[string]int, numKeys)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("memento-minimal-disruption-key-%d", i)
		mappaOld[key] = engine.GetBucket(key)
	}

	// Rimozione (S′ = S \ {x})
	removedNode := 25
	engine.RemoveBucket(removedNode)

	// Verifica: mappa_old[k] ≠ x => mappa_new[k] = mappa_old[k]
	violations := 0
	for key, bucketOld := range mappaOld {
		// Se mappa_old[k] ≠ x, allora mappa_new[k] DEVE essere mappa_old[k]
		if bucketOld != removedNode {
			bucketNew := engine.GetBucket(key)
			if bucketNew != bucketOld {
				violations++
				t.Errorf("Minimal Disruption violation: key %s moved from %d to %d (was not on removed node %d)",
					key, bucketOld, bucketNew, removedNode)
			}
		}
		// Se mappa_old[k] = x, la chiave può essere reindirizzata (OK)
	}

	if violations > 0 {
		t.Fatalf("Minimal Disruption property violated: %d keys incorrectly remapped", violations)
	}
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
	const N = 50     // numero di nodi
	const K = 100000 // numero di chiavi

	engine := NewMementoEngine(N)

	counts := make([]int, N)
	for i := 0; i < K; i++ {
		key := fmt.Sprintf("balance-key-%d", i)
		b := engine.GetBucket(key)
		if b < 0 || b >= N {
			t.Fatalf("Invalid bucket %d for key %s (range: [0, %d))", b, key, N)
		}
		counts[b]++
	}

	// Check that all buckets are used
	nonZeroBuckets := 0
	for _, count := range counts {
		if count > 0 {
			nonZeroBuckets++
		}
	}
	if nonZeroBuckets != N {
		t.Errorf("Expected %d buckets to be used, got %d", N, nonZeroBuckets)
	}

	// Calcolo statistiche teoriche
	// Atteso per nodo: μ = K / N
	mu := float64(K) / float64(N)

	// Deviazione standard teorica: σ = √(K · (1/N) · (1 − 1/N))
	p := 1.0 / float64(N)
	sigma := math.Sqrt(float64(K) * p * (1.0 - p))

	// Calcolo statistiche osservate
	// Media osservata
	mean := 0.0
	for _, count := range counts {
		mean += float64(count)
	}
	mean /= float64(N)

	// Deviazione standard osservata
	variance := 0.0
	for _, count := range counts {
		diff := float64(count) - mean
		variance += diff * diff
	}
	variance /= float64(N)
	stdDev := math.Sqrt(variance)

	// Coefficiente di variazione osservato: CV = std(node)/mean(node)
	CV := stdDev / mean

	// Coefficiente di variazione atteso: CV_atteso ≈ √[(N−1)/K]
	CV_atteso := math.Sqrt((float64(N) - 1.0) / float64(K))

	// Verifica: CV <= CV_atteso + 20%
	CV_max := CV_atteso * 1.2

	t.Logf("Distribution Test (N=%d, K=%d):", N, K)
	t.Logf("  Expected per node (μ): %.2f", mu)
	t.Logf("  Expected std dev (σ): %.2f", sigma)
	t.Logf("  Observed mean: %.2f", mean)
	t.Logf("  Observed std dev: %.2f", stdDev)
	t.Logf("  Coefficient of Variation (CV): %.6f", CV)
	t.Logf("  Expected CV: %.6f", CV_atteso)
	t.Logf("  Max allowed CV (CV_atteso + 20%%): %.6f", CV_max)

	// Verifica che il CV sia entro i limiti
	if CV > CV_max {
		t.Errorf("Coefficient of Variation too high: %.6f > %.6f (expected CV: %.6f, margin: +20%%)",
			CV, CV_max, CV_atteso)
	}
}
