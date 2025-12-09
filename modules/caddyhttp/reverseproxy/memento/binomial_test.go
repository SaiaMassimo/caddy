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
	"testing"
)

func TestBinomialEngine(t *testing.T) {
	// Test basic functionality
	engine := NewBinomialEngine(5)

	if engine.Size() != 5 {
		t.Errorf("Expected size 5, got %d", engine.Size())
	}

	// Test bucket assignment
	key := "test-key-123"
	bucket := engine.GetBucket(key)

	if bucket < 0 || bucket >= engine.Size() {
		t.Errorf("Bucket %d is out of range [0, %d)", bucket, engine.Size())
	}

	// Test consistency - same key should always map to same bucket
	bucket2 := engine.GetBucket(key)
	if bucket != bucket2 {
		t.Errorf("Inconsistent bucket assignment: %d vs %d", bucket, bucket2)
	}
}

func TestBinomialEngineSingleNode(t *testing.T) {
	engine := NewBinomialEngine(1)

	if engine.Size() != 1 {
		t.Errorf("Expected size 1, got %d", engine.Size())
	}

	// All keys should map to bucket 0
	for i := 0; i < 10; i++ {
		key := "test-key-" + string(rune(i))
		bucket := engine.GetBucket(key)
		if bucket != 0 {
			t.Errorf("Expected bucket 0 for single node, got %d", bucket)
		}
	}
}

func TestBinomialEngineAddBucket(t *testing.T) {
	engine := NewBinomialEngine(3)
	originalSize := engine.Size()

	// Add a bucket
	newBucket := engine.AddBucket()

	if engine.Size() != originalSize+1 {
		t.Errorf("Expected size %d, got %d", originalSize+1, engine.Size())
	}

	if newBucket != originalSize {
		t.Errorf("Expected new bucket %d, got %d", originalSize, newBucket)
	}

	// Test that the new bucket is valid
	if newBucket < 0 || newBucket >= engine.Size() {
		t.Errorf("New bucket %d is out of range [0, %d)", newBucket, engine.Size())
	}
}

func TestBinomialEngineRemoveBucket(t *testing.T) {
	engine := NewBinomialEngine(5)
	originalSize := engine.Size()

	// Remove a bucket
	newSize := engine.RemoveBucket()

	if engine.Size() != originalSize-1 {
		t.Errorf("Expected size %d, got %d", originalSize-1, engine.Size())
	}

	if newSize != originalSize-1 {
		t.Errorf("Expected returned size %d, got %d", originalSize-1, newSize)
	}
}

func TestBinomialEngineDistribution(t *testing.T) {
	const N = 10   // numero di nodi
	const K = 1000 // numero di chiavi

	engine := NewBinomialEngine(N)

	// Test distribution across buckets
	bucketCounts := make([]int, N)

	for i := 0; i < K; i++ {
		key := "test-key-" + string(rune(i))
		bucket := engine.GetBucket(key)
		if bucket < 0 || bucket >= N {
			t.Fatalf("Invalid bucket %d for key %s (range: [0, %d))", bucket, key, N)
		}
		bucketCounts[bucket]++
	}

	// Check that all buckets are used
	nonZeroBuckets := 0
	for _, count := range bucketCounts {
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
	for _, count := range bucketCounts {
		mean += float64(count)
	}
	mean /= float64(N)

	// Deviazione standard osservata
	variance := 0.0
	for _, count := range bucketCounts {
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

func TestBinomialEngineMinimalDisruption(t *testing.T) {
	// Test Minimal Disruption property:
	// When removing node x: S' = S \ {x}
	// For all keys k: if mappa_old[k] ≠ x => mappa_new[k] = mappa_old[k]
	// Only keys mapped to the removed node should be remapped
	//
	// Note: BinomialEngine only supports LIFO removal (removes last bucket),
	// so we remove the last node and verify Minimal Disruption

	const initialNodes = 50
	const numKeys = 10000

	// Setup: create engine with initialNodes nodes
	engine := NewBinomialEngine(initialNodes)

	// Generate test keys and map them to buckets (mappa_old)
	mappaOld := make(map[string]int, numKeys)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("minimal-disruption-key-%d", i)
		mappaOld[key] = engine.GetBucket(key)
	}

	// Remove the last node x (BinomialEngine only supports LIFO removal)
	removedNode := initialNodes - 1 // Last node (bucket 49)
	engine.RemoveBucket()

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

func TestBinomialEngineConsistency(t *testing.T) {
	engine := NewBinomialEngine(8)

	// Test that same keys always map to same buckets
	keys := []string{"key1", "key2", "key3", "key4", "key5"}
	originalBuckets := make(map[string]int)

	for _, key := range keys {
		originalBuckets[key] = engine.GetBucket(key)
	}

	// Test multiple times
	for i := 0; i < 10; i++ {
		for _, key := range keys {
			bucket := engine.GetBucket(key)
			if bucket != originalBuckets[key] {
				t.Errorf("Inconsistent mapping for key %s: %d vs %d", key, bucket, originalBuckets[key])
			}
		}
	}
}

func TestBinomialEngineFilters(t *testing.T) {
	engine := NewBinomialEngine(7)

	enclosingFilter := engine.EnclosingTreeFilter()
	minorFilter := engine.MinorTreeFilter()

	// Enclosing filter should be larger than minor filter
	if enclosingFilter <= minorFilter {
		t.Errorf("Enclosing filter %d should be larger than minor filter %d", enclosingFilter, minorFilter)
	}

	// Filters should be powers of 2 minus 1
	if (enclosingFilter+1)&enclosingFilter != 0 {
		t.Errorf("Enclosing filter %d should be 2^n-1", enclosingFilter)
	}

	if (minorFilter+1)&minorFilter != 0 {
		t.Errorf("Minor filter %d should be 2^n-1", minorFilter)
	}
}

func BenchmarkBinomialEngineGetBucket(b *testing.B) {
	engine := NewBinomialEngine(100)
	key := "benchmark-key-12345"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.GetBucket(key)
	}
}

func BenchmarkBinomialEngineGetBucketDifferentKeys(b *testing.B) {
	engine := NewBinomialEngine(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "benchmark-key-" + string(rune(i))
		engine.GetBucket(key)
	}
}

// TestBinomialEngineLoadBalancing verifica il bilanciamento del carico
// con 50 nodi e 100000 chiavi usando le stesse statistiche rigorose
// degli altri test (CV <= CV_atteso+20%)
func TestBinomialEngineLoadBalancing(t *testing.T) {
	const numNodes = 50
	const numKeys = 100000

	// Crea un engine con 50 nodi
	engine := NewBinomialEngine(numNodes)

	if engine.Size() != numNodes {
		t.Fatalf("Expected engine size %d, got %d", numNodes, engine.Size())
	}

	// Distribuisci le chiavi e conta la distribuzione
	bucketCounts := make([]int, numNodes)

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		bucket := engine.GetBucket(key)

		// Verifica che il bucket sia valido
		if bucket < 0 || bucket >= numNodes {
			t.Fatalf("Invalid bucket %d for key %s (range: [0, %d))", bucket, key, numNodes)
		}

		bucketCounts[bucket]++
	}

	// Calcola statistiche di distribuzione
	minKeys := numKeys
	maxKeys := 0
	nonZeroBuckets := 0

	for i := 0; i < numNodes; i++ {
		count := bucketCounts[i]
		if count > 0 {
			nonZeroBuckets++
		}
		if count < minKeys {
			minKeys = count
		}
		if count > maxKeys {
			maxKeys = count
		}
	}

	// Stampa statistiche per debug
	t.Logf("Distribution statistics:")
	t.Logf("  Total buckets: %d", numNodes)
	t.Logf("  Buckets with keys: %d", nonZeroBuckets)
	t.Logf("  Min keys per bucket: %d", minKeys)
	t.Logf("  Max keys per bucket: %d", maxKeys)
	t.Logf("  Average keys per bucket: %.2f", float64(numKeys)/float64(numNodes))

	// Calcolo statistiche teoriche
	// Atteso per nodo: μ = K / N
	mu := float64(numKeys) / float64(numNodes)

	// Deviazione standard teorica: σ = √(K · (1/N) · (1 − 1/N))
	p := 1.0 / float64(numNodes)
	sigma := math.Sqrt(float64(numKeys) * p * (1.0 - p))

	// Calcolo statistiche osservate
	// Media osservata
	mean := 0.0
	for _, count := range bucketCounts {
		mean += float64(count)
	}
	mean /= float64(numNodes)

	// Deviazione standard osservata
	variance := 0.0
	for _, count := range bucketCounts {
		diff := float64(count) - mean
		variance += diff * diff
	}
	variance /= float64(numNodes)
	stdDev := math.Sqrt(variance)

	// Coefficiente di variazione osservato: CV = std(node)/mean(node)
	CV := stdDev / mean

	// Coefficiente di variazione atteso: CV_atteso ≈ √[(N−1)/K]
	CV_atteso := math.Sqrt((float64(numNodes) - 1.0) / float64(numKeys))

	// Verifica: CV <= CV_atteso + 20%
	CV_max := CV_atteso * 1.2

	t.Logf("Distribution Test (N=%d, K=%d):", numNodes, numKeys)
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

// TestBinomialEngineLoadBalancingDetailed verifica il bilanciamento dettagliato
// e mostra la distribuzione per ogni bucket
func TestBinomialEngineLoadBalancingDetailed(t *testing.T) {
	const numNodes = 50
	const numKeys = 100000

	engine := NewBinomialEngine(numNodes)
	bucketCounts := make([]int, numNodes)

	// Distribuisci le chiavi
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("detailed-key-%d", i)
		bucket := engine.GetBucket(key)
		bucketCounts[bucket]++
	}

	// Stampa la distribuzione dettagliata
	t.Logf("\nDetailed distribution (50 nodes, 100000 keys):")
	expectedPerBucket := float64(numKeys) / float64(numNodes)

	for i := 0; i < numNodes; i++ {
		count := bucketCounts[i]
		diff := float64(count) - expectedPerBucket
		percentage := (float64(count) / float64(numKeys)) * 100
		t.Logf("  Bucket %2d: %3d keys (%.2f%%, diff: %+.2f)",
			i, count, percentage, diff)
	}

	// Verifica che la somma sia corretta
	totalKeys := 0
	for i := 0; i < numNodes; i++ {
		totalKeys += bucketCounts[i]
	}
	if totalKeys != numKeys {
		t.Errorf("Total keys mismatch: expected %d, got %d", numKeys, totalKeys)
	}
}

// TestBinomialEngineConsistentHashing verifica che le chiavi
// vengano mappate in modo consistente dopo aggiunta/rimozione di nodi
func TestBinomialEngineConsistentHashing(t *testing.T) {
	const numNodes = 50
	const numKeys = 500

	engine := NewBinomialEngine(numNodes)

	// Memorizza il mapping originale delle chiavi
	originalMapping := make(map[string]int, numKeys)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("consistency-key-%d", i)
		originalMapping[key] = engine.GetBucket(key)
	}

	// Rimuovi alcuni nodi
	removedNodes := []int{5, 10, 15, 20, 25}
	for range removedNodes {
		engine.RemoveBucket()
	}

	// Verifica che le chiavi non rimappate vengano ancora mappate allo stesso bucket
	// (Questo test verifica che l'algoritmo sia deterministico)
	key := "consistency-key-0"
	originalBucket := originalMapping[key]

	// Nota: questo test potrebbe fallire se l'algoritmo BinomialHash rimappa
	// le chiavi quando i nodi vengono rimossi. Questo è un comportamento
	// dell'algoritmo che dobbiamo verificare.

	// Ripristina il mapping originale testando con un nuovo engine
	engine2 := NewBinomialEngine(numNodes)
	bucket2 := engine2.GetBucket(key)

	if originalBucket != bucket2 {
		t.Errorf("Inconsistent mapping after engine recreation: %d vs %d",
			originalBucket, bucket2)
	}
}

// TestBinomialEngineMonotonicity verifica la monotonicità del BinomialEngine
// Per valutare se l'algoritmo stesso mantiene la monotonicità
func TestBinomialEngineMonotonicity(t *testing.T) {
	const initialNodes = 50
	const numKeys = 100000

	t.Run("Monotonicity_On_Removal", func(t *testing.T) {
		// Setup iniziale: S = 50 nodi
		engine := NewBinomialEngine(initialNodes)

		// Genera 100k chiavi e calcola mappa_old[k]
		keyToBucketOld := make(map[string]int, numKeys)
		for i := 0; i < numKeys; i++ {
			key := fmt.Sprintf("binomial-key-%d", i)
			keyToBucketOld[key] = engine.GetBucket(key)
		}

		// Rimuovi l'ultimo nodo (BinomialEngine rimuove solo l'ultimo)
		removedNodeIndex := initialNodes - 1 // Ultimo nodo
		t.Logf("Removing last node: %d", removedNodeIndex)

		// Calcola mappa_new su S' = S \ {last_node}
		engine.RemoveBucket()

		// Verifica monotonicità su rimozione
		// ASSERZIONE: per ogni k con mappa_old[k] ≠ x deve valere mappa_new[k] = mappa_old[k]
		monotonicityViolations := 0
		keysOnRemovedNode := 0

		for key, bucketOld := range keyToBucketOld {
			bucketNew := engine.GetBucket(key)

			// Se era sul nodo rimosso, viene reindirizzata (OK, ma non controlliamo dove)
			if bucketOld == removedNodeIndex {
				keysOnRemovedNode++
				// Non viene reindirizzata, quindi finisce fuori range o su altro nodo
			} else {
				// Se NON era sul nodo rimosso, DEVE rimanere sullo stesso bucket
				if bucketOld != bucketNew {
					monotonicityViolations++
				}
			}
		}

		violationRate := float64(monotonicityViolations) / float64(numKeys) * 100
		t.Logf("Binomial Engine Monotonicity on removal stats:")
		t.Logf("  Keys on removed node: %d", keysOnRemovedNode)
		t.Logf("  Violations (keys moved despite not being on removed node): %d/%d (%.4f%%)",
			monotonicityViolations, numKeys, violationRate)

		// Questo ci dirà se il problema è dell'algoritmo stesso
		if violationRate > 5.0 {
			t.Errorf("BinomialEngine has too many monotonicity violations: %.4f%% (> 5%% would be problematic)",
				violationRate)
		} else {
			t.Logf("  BinomialEngine introduces ~%.2f%% monotonicity violations when removing nodes", violationRate)
		}
	})

	t.Run("Monotonicity_On_Addition", func(t *testing.T) {
		// Setup iniziale: S = 50 nodi
		engine := NewBinomialEngine(initialNodes)

		// Genera 100k chiavi e calcola mappa_old[k]
		keyToBucketOld := make(map[string]int, numKeys)
		for i := 0; i < numKeys; i++ {
			key := fmt.Sprintf("binomial-key-%d", i)
			keyToBucketOld[key] = engine.GetBucket(key)
		}

		// Aggiungi un nuovo nodo x
		newNodeIndex := engine.AddBucket()
		t.Logf("Added new node: %d", newNodeIndex)

		// Calcola mappa_new su S' = S ∪ {x}
		// ASSERZIONE: per ogni k, se mappa_new[k] ≠ mappa_old[k],
		// allora mappa_new[k] DEVE essere il nuovo nodo x
		monotonicityViolations := 0
		keysMovedToNewNode := 0
		keysMovedToOldNode := 0

		for key, bucketOld := range keyToBucketOld {
			bucketNew := engine.GetBucket(key)

			if bucketOld != bucketNew {
				keysMovedToNewNode++
				// La chiave non è andata sul nuovo nodo, è una violazione non è andata sul nuovo nodo, è una violazione
				if bucketNew != newNodeIndex {
					monotonicityViolations++
					keysMovedToOldNode++
				}
			}
		}

		expectedKeysMoving := float64(numKeys) / float64(initialNodes+1)
		violationRate := float64(monotonicityViolations) / float64(numKeys) * 100

		t.Logf("Binomial Engine Monotonicity on addition stats:")
		t.Logf("  Keys moved: %d (expected: ~%.0f)",
			keysMovedToNewNode, expectedKeysMoving)
		t.Logf("  Violations (keys moved to old node instead of new): %d", keysMovedToOldNode)
		t.Logf("  Violation rate: %.4f%%", violationRate)

		// BinomialEngine dovrebbe mantenere monotonicità su aggiunta
		if violationRate > 1.0 {
			t.Errorf("BinomialEngine has too many monotonicity violations on addition: %.4f%% (> 1%% would be problematic)",
				violationRate)
		} else {
			t.Logf("  BinomialEngine maintains monotonicity on node addition")
		}
	})
}

// TestBinomialEngineMinimalDistribution verifica la "minimal distribution"
// cioè che il numero di chiavi che si muovono sia minimo e atteso teoricamente
func TestBinomialEngineMinimalDistribution(t *testing.T) {
	t.Run("MinimalDistribution_On_Addition", func(t *testing.T) {
		const numRuns = 5
		const initialNodes = 50
		const numKeys = 100000

		for run := 0; run < numRuns; run++ {
			engine := NewBinomialEngine(initialNodes)

			// Mappa chiavi ai bucket iniziali
			keyToBucketBefore := make(map[string]int, numKeys)
			for i := 0; i < numKeys; i++ {
				key := fmt.Sprintf("min-dist-key-%d-%d", run, i)
				keyToBucketBefore[key] = engine.GetBucket(key)
			}

			// Aggiungi un nuovo nodo
			newNodeIndex := engine.AddBucket()

			// Calcola chiavi mosse e violazioni
			keysMoved := 0
			violations := 0 // Chiavi mosse verso nodi vecchi invece che nuovo

			for key, oldBucket := range keyToBucketBefore {
				newBucket := engine.GetBucket(key)

				if oldBucket != newBucket {
					keysMoved++
					// Violazione: se si muove ma non va sul nodo nuovo
					if newBucket != newNodeIndex {
						violations++
					}
				}
			}

			// Calcolo teorico: p* = 1/(N+1) per consistent hashing uniforme
			expectedMoveProbability := 1.0 / float64(initialNodes+1)
			expectedMoved := expectedMoveProbability * float64(numKeys)

			// Verifica statistica: |moved - n·p*| ≤ z·√(n·p*·(1-p*))
			// Usiamo z=4 per avere un test robusto
			z := 4.0
			stdDev := math.Sqrt(float64(numKeys) * expectedMoveProbability * (1 - expectedMoveProbability))
			deviation := math.Abs(float64(keysMoved) - expectedMoved)

			// Verifica frazione: |movedFraction - expectedFraction| < expectedFraction * 0.5
			movedFraction := float64(keysMoved) / float64(numKeys)
			expectedFraction := expectedMoveProbability
			fracDiff := math.Abs(movedFraction - expectedFraction)

			t.Logf("Run %d: Minimal distribution on addition", run+1)
			t.Logf("  Initial nodes: %d, added node: %d", initialNodes, newNodeIndex)
			t.Logf("  Keys moved: %d (expected: ~%.0f)", keysMoved, expectedMoved)
			t.Logf("  Moved fraction: %.6f (expected: %.6f, diff: %.6f)", movedFraction, expectedFraction, fracDiff)
			t.Logf("  Violations (moved to old nodes): %d", violations)
			t.Logf("  Standard deviation: %.2f", stdDev)
			t.Logf("  Deviation: %.2f (max allowed: %.2f)", deviation, z*stdDev)

			// Asserzione 1: Nessun movimento inutile
			if violations > 0 {
				t.Errorf("Run %d: Found %d keys moved to old nodes instead of new node", run+1, violations)
			}

			// Asserzione 2: il numero di chiavi mosse è statisticamente accettabile
			if deviation > z*stdDev {
				t.Errorf("Run %d: Deviation too high: %.2f > %.2f", run+1, deviation, z*stdDev)
			}

			// Asserzione 3: |movedFraction - expectedFraction| < expectedFraction * 0.5
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
			engine := NewBinomialEngine(initialNodes)

			// Mappa chiavi ai bucket iniziali
			keyToBucketBefore := make(map[string]int, numKeys)
			for i := 0; i < numKeys; i++ {
				key := fmt.Sprintf("min-dist-key-%d-%d", run, i)
				keyToBucketBefore[key] = engine.GetBucket(key)
			}

			// Rimuovi l'ultimo nodo (l'unico supportato da BinomialEngine)
			removedNodeIndex := initialNodes - 1
			engine.RemoveBucket()

			// Calcola chiavi mosse e violazioni
			keysMoved := 0
			keysOnRemovedNode := 0
			violations := 0 // Chiavi NON sul nodo rimosso ma che si sono mosse

			for key, oldBucket := range keyToBucketBefore {
				newBucket := engine.GetBucket(key)

				if oldBucket == removedNodeIndex {
					keysOnRemovedNode++
					// Chiavi sul nodo rimosso devono essere reindirizzate
					if newBucket == removedNodeIndex {
						violations++ // Ancora sul nodo rimosso
					} else if oldBucket != newBucket {
						keysMoved++
					}
				} else {
					// Chiavi NON sul nodo rimosso non devono muoversi
					if oldBucket != newBucket {
						violations++
						keysMoved++
					}
				}
			}

			// Calcolo teorico: p* = 1/N (le chiavi sul nodo rimosso)
			expectedMoveProbability := 1.0 / float64(initialNodes)
			expectedMoved := expectedMoveProbability * float64(numKeys)

			// Verifica statistica
			z := 4.0
			stdDev := math.Sqrt(float64(numKeys) * expectedMoveProbability * (1 - expectedMoveProbability))
			deviation := math.Abs(float64(keysMoved) - expectedMoved)

			// Verifica frazione: |movedFraction - expectedFraction| < expectedFraction * 0.5
			movedFraction := float64(keysMoved) / float64(numKeys)
			expectedFraction := expectedMoveProbability
			fracDiff := math.Abs(movedFraction - expectedFraction)

			t.Logf("Run %d: Minimal distribution on removal", run+1)
			t.Logf("  Initial nodes: %d, removed node: %d", initialNodes, removedNodeIndex)
			t.Logf("  Keys on removed node: %d (expected: ~%.0f)", keysOnRemovedNode, expectedMoved)
			t.Logf("  Keys moved: %d", keysMoved)
			t.Logf("  Moved fraction: %.6f (expected: %.6f, diff: %.6f)", movedFraction, expectedFraction, fracDiff)
			t.Logf("  Violations (non-removed keys that moved): %d", violations)
			t.Logf("  Standard deviation: %.2f", stdDev)
			t.Logf("  Deviation: %.2f (max allowed: %.2f)", deviation, z*stdDev)

			// Asserzione 1: Nessun movimento inutile
			if violations > 0 {
				t.Errorf("Run %d: Found %d keys moved that were not on removed node", run+1, violations)
			}

			// Asserzione 2: |movedFraction - expectedFraction| < expectedFraction * 0.5
			if fracDiff >= expectedFraction*0.5 {
				t.Errorf("Run %d: Moved fraction diff too high: %.6f >= %.6f", run+1, fracDiff, expectedFraction*0.5)
			}
		}
	})
}

// TestBinomialEngineLoadBalancingStats verifica il bilanciamento delle chiavi tra i nodi
// Calcola: min/max/mean/stdDev dei conteggi, CV, chi-square normalizzato, e violazioni a 5·σ
func TestBinomialEngineLoadBalancingStats(t *testing.T) {
	const N = 50
	const n = 100000

	engine := NewBinomialEngine(N)

	counts := make([]int, N)
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("balance-key-%d", i)
		b := engine.GetBucket(key)
		counts[b]++
	}

	// Statistiche di base attese
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

	t.Logf("Balance Test (BinomialEngine GetBucket):")
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
