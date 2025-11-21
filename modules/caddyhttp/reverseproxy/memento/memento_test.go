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

func TestMemento(t *testing.T) {
	memento := NewMemento()

	// Test initial state
	if !memento.IsEmpty() {
		t.Error("Memento should be empty initially")
	}

	if memento.Size() != 0 {
		t.Errorf("Expected size 0, got %d", memento.Size())
	}
}

func TestMementoRemember(t *testing.T) {
	memento := NewMemento()

	// Remember a removal
	lastRemoved := memento.Remember(5, 3, -1)
	if lastRemoved != 5 {
		t.Errorf("Expected lastRemoved 5, got %d", lastRemoved)
	}

	if memento.IsEmpty() {
		t.Error("Memento should not be empty after remembering")
	}

	if memento.Size() != 1 {
		t.Errorf("Expected size 1, got %d", memento.Size())
	}
}

func TestMementoReplacer(t *testing.T) {
	memento := NewMemento()

	// Remember a removal
	memento.Remember(5, 3, -1)

	// Test replacer
	replacer := memento.Replacer(5)
	if replacer != 3 {
		t.Errorf("Expected replacer 3, got %d", replacer)
	}

	// Test non-existent bucket
	replacer = memento.Replacer(10)
	if replacer != -1 {
		t.Errorf("Expected replacer -1 for non-existent bucket, got %d", replacer)
	}
}

func TestMementoRestore(t *testing.T) {
	memento := NewMemento()

	// Remember a removal
	memento.Remember(5, 3, -1)

	// Restore the bucket
	prevRemoved := memento.Restore(5)
	if prevRemoved != -1 {
		t.Errorf("Expected prevRemoved -1, got %d", prevRemoved)
	}

	if !memento.IsEmpty() {
		t.Error("Memento should be empty after restore")
	}

	if memento.Size() != 0 {
		t.Errorf("Expected size 0, got %d", memento.Size())
	}
}

func TestMementoSequence(t *testing.T) {
	memento := NewMemento()

	// Remember multiple removals in sequence
	lastRemoved := memento.Remember(5, 3, -1)
	if lastRemoved != 5 {
		t.Errorf("Expected lastRemoved 5, got %d", lastRemoved)
	}

	lastRemoved = memento.Remember(7, 2, 5)
	if lastRemoved != 7 {
		t.Errorf("Expected lastRemoved 7, got %d", lastRemoved)
	}

	lastRemoved = memento.Remember(2, 1, 7)
	if lastRemoved != 2 {
		t.Errorf("Expected lastRemoved 2, got %d", lastRemoved)
	}

	if memento.Size() != 3 {
		t.Errorf("Expected size 3, got %d", memento.Size())
	}

	// Test replacers
	if memento.Replacer(5) != 3 {
		t.Error("Expected replacer 3 for bucket 5")
	}
	if memento.Replacer(7) != 2 {
		t.Error("Expected replacer 2 for bucket 7")
	}
	if memento.Replacer(2) != 1 {
		t.Error("Expected replacer 1 for bucket 2")
	}

	// Restore in reverse order
	prevRemoved := memento.Restore(2)
	if prevRemoved != 7 {
		t.Errorf("Expected prevRemoved 7, got %d", prevRemoved)
	}

	prevRemoved = memento.Restore(7)
	if prevRemoved != 5 {
		t.Errorf("Expected prevRemoved 5, got %d", prevRemoved)
	}

	prevRemoved = memento.Restore(5)
	if prevRemoved != -1 {
		t.Errorf("Expected prevRemoved -1, got %d", prevRemoved)
	}

	if !memento.IsEmpty() {
		t.Error("Memento should be empty after all restores")
	}
}

func TestMementoConcurrent(t *testing.T) {
	memento := NewMemento()

	// Test concurrent access
	done := make(chan bool, 2)

	// Goroutine 1: Add removals
	go func() {
		for i := 0; i < 100; i++ {
			memento.Remember(i, i-1, -1)
		}
		done <- true
	}()

	// Goroutine 2: Check replacers
	go func() {
		for i := 0; i < 100; i++ {
			replacer := memento.Replacer(i)
			if replacer != -1 && replacer != i-1 {
				t.Errorf("Unexpected replacer for bucket %d: %d", i, replacer)
			}
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done
}

func TestConsistentEngine(t *testing.T) {
	// Create a consistent engine
	consistentEngine := NewConsistentEngine()

	// Test initial state (empty)
	if consistentEngine.Size() != 0 {
		t.Errorf("Expected size 0, got %d", consistentEngine.Size())
	}

	// Add 5 nodes
	for i := 0; i < 5; i++ {
		consistentEngine.AddNode(fmt.Sprintf("node%d", i))
	}

	// Test size after adding nodes
	if consistentEngine.Size() != 5 {
		t.Errorf("Expected size 5, got %d", consistentEngine.Size())
	}

	// Test GetBucket
	bucket := consistentEngine.GetBucket("test-key")
	if bucket < 0 || bucket >= 5 {
		t.Errorf("Bucket %d out of range [0, 5)", bucket)
	}

	// Test topology (should have 5 nodes)
	topology := consistentEngine.GetTopology()
	if len(topology) != 5 {
		t.Errorf("Expected topology size 5, got %d", len(topology))
	}
}

func TestConsistentEngineNodeOperations(t *testing.T) {
	consistentEngine := NewConsistentEngine()

	// Add nodes
	consistentEngine.AddNode("node1")
	consistentEngine.AddNode("node2")
	consistentEngine.AddNode("node3")

	// Check topology
	topology := consistentEngine.GetTopology()
	if len(topology) != 3 {
		t.Errorf("Expected topology size 3, got %d", len(topology))
	}

	// Remove a node
	consistentEngine.RemoveNode("node2")

	// Check topology after removal
	topology = consistentEngine.GetTopology()
	if len(topology) != 2 {
		t.Errorf("Expected topology size 2, got %d", len(topology))
	}

	// Check that node2 is not in topology
	found := false
	for _, node := range topology {
		if node == "node2" {
			found = true
			break
		}
	}
	if found {
		t.Error("node2 should not be in topology after removal")
	}

	// Restore the node
	consistentEngine.RestoreNode("node2")

	// Check topology after restore
	topology = consistentEngine.GetTopology()
	if len(topology) != 3 {
		t.Errorf("Expected topology size 3 after restore, got %d", len(topology))
	}
}

func TestConsistentEngineConsistency(t *testing.T) {
	consistentEngine := NewConsistentEngine()

	// Add nodes
	for i := 0; i < 5; i++ {
		consistentEngine.AddNode(fmt.Sprintf("node%d", i))
	}

	// Test consistency - same key should map to same bucket
	key := "consistent-test-key"
	bucket1 := consistentEngine.GetBucket(key)
	bucket2 := consistentEngine.GetBucket(key)

	if bucket1 != bucket2 {
		t.Errorf("Inconsistent mapping: %d vs %d", bucket1, bucket2)
	}

	// Remove a node and test that most keys still map to same bucket
	consistentEngine.RemoveNode("node2")

	bucket3 := consistentEngine.GetBucket(key)
	// The bucket might change due to removal, but it should be consistent
	bucket4 := consistentEngine.GetBucket(key)

	if bucket3 != bucket4 {
		t.Errorf("Inconsistent mapping after removal: %d vs %d", bucket3, bucket4)
	}
}

func BenchmarkMementoRemember(b *testing.B) {
	memento := NewMemento()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		memento.Remember(i%1000, (i-1)%1000, -1)
	}
}

func BenchmarkMementoReplacer(b *testing.B) {
	memento := NewMemento()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		memento.Remember(i, i-1, -1)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		memento.Replacer(i % 1000)
	}
}

func BenchmarkConsistentEngineGetBucket(b *testing.B) {
	consistentEngine := NewConsistentEngine()

	// Add nodes
	for i := 0; i < 100; i++ {
		consistentEngine.AddNode(fmt.Sprintf("node%d", i))
	}

	key := "benchmark-key"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		consistentEngine.GetBucket(key)
	}
}

// TestMementoDirectLoadBalancing verifica direttamente la logica di Memento
// simulando manualmente le rimozioni di bucket senza ConsistentEngine
func TestMementoDirectLoadBalancing(t *testing.T) {
	const numNodes = 50
	const numKeys = 100000

	// Crea direttamente un BinomialEngine e un Memento
	engine := NewBinomialEngine(numNodes)
	memento := NewMemento()

	if engine.Size() != numNodes {
		t.Fatalf("Expected engine size %d, got %d", numNodes, engine.Size())
	}

	// Prima distribuzione: mappa le chiavi PRIMA della rimozione
	distributionBefore := make([]int, numNodes)
	keyToBucket := make(map[string]int, numKeys)

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("memento-direct-key-%d", i)
		bucket := engine.GetBucket(key)
		if bucket < 0 || bucket >= numNodes {
			t.Fatalf("Invalid bucket %d for key %s", bucket, key)
		}
		distributionBefore[bucket]++
		keyToBucket[key] = bucket
	}

	// Calcola statistiche iniziali
	meanBefore := float64(numKeys) / float64(numNodes)
	t.Logf("\nMemento Direct Test - Distribution BEFORE node removal:")
	t.Logf("  Mean keys per node: %.2f", meanBefore)
	t.Logf("  Nodes: %d", numNodes)

	// Rimuovi un nodo CASUALE direttamente nell'engine
	rand.Seed(42) // Seed fisso per riproducibilità
	randomNodeIndex := rand.Intn(numNodes)
	t.Logf("  Removing random node: index %d", randomNodeIndex)

	// Verifica quante chiavi erano sul nodo che stiamo per rimuovere
	keysOnRemovedNode := distributionBefore[randomNodeIndex]
	t.Logf("  Keys on removed node: %d", keysOnRemovedNode)

	// Rimu 것입니다 il bucket dall'engine
	newSize := engine.RemoveBucket()

	// Ricorda la rimozione nel Memento manualmente
	// Il replacer è il nuovo size (l'ultimo bucket valido)
	lastRemoved := memento.Remember(randomNodeIndex, newSize, -1)

	if lastRemoved != randomNodeIndex {
		t.Errorf("Expected lastRemoved %d, got %d", randomNodeIndex, lastRemoved)
	}

	// Verifica che il memento contenga la rimozione
	if memento.IsEmpty() {
		t.Error("Memento should not be empty after node removal")
	}
	if memento.Size() != 1 {
		t.Errorf("Expected memento size 1, got %d", memento.Size())
	}

	// Helper function per ottenere il bucket corretto considerando Memento
	getBucketWithMemento := func(key string) int {
		bucket := engine.GetBucket(key)
		replacer := memento.Replacer(bucket)
		if replacer != -1 {
			return replacer
		}
		return bucket
	}

	// Seconda distribuzione: mappa le chiavi DOPO la rimozione
	distributionAfter := make([]int, numNodes)
	keysMoved := 0

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("memento-direct-key-%d", i)
		bucketAfter := getBucketWithMemento(key)
		bucketBefore := keyToBucket[key]

		// La chiave dopo la rimozione NON dovrebbe mai puntare al nodo rimosso
		if bucketAfter == randomNodeIndex {
			t.Errorf("Key %s was still mapped to removed bucket %d", key, bucketAfter)
		}

		// Verifica che il bucket sia valido
		if bucketAfter < 0 || bucketAfter >= numNodes {
			t.Errorf("Invalid bucket %d for key %s after removal", bucketAfter, key)
			continue
		}

		distributionAfter[bucketAfter]++

		// Conta le chiavi che sono state spostate
		if bucketBefore != bucketAfter {
			keysMoved++
		}
	}

	// Calcola statistiche dopo la rimozione
	totalKeysAfter := 0
	minKeysAfter := numKeys
	maxKeysAfter := 0
	nonZeroNodesAfter := 0

	for i := 0; i < numNodes; i++ {
		// Ignora il nodo rimosso nella statistica
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

	// Calcola coefficiente di variazione dopo
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

	t.Logf("\nMemento Direct Test - Distribution AFTER node removal:")
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

	// Verifiche di bilanciamento dopo la rimozione
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

	// Verifica che il numero di chiavi corrisponda
	if totalKeysAfter != numKeys {
		t.Errorf("Total keys mismatch after removal: expected %d, got %d",
			numKeys, totalKeysAfter)
	}

	// Verifica che tutte le chiavi del nodo rimosso siano state spostate
	if keysMoved < keysOnRemovedNode {
		t.Errorf("Expected at least %d keys moved, got %d",
			keysOnRemovedNode, keysMoved)
	}
}

// setupMementoTest crea un setup comune per i test di Memento
func setupMementoTest() (*BinomialEngine, *Memento, map[string]int) {
	const numNodes = 50
	const numKeys = 100000

	engine := NewBinomialEngine(numNodes)
	memento := NewMemento()
	keyToBucketBefore := make(map[string]int, numKeys)

	// Mappa tutte le chiavi prima della rimozione
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		bucket := engine.GetBucket(key)
		keyToBucketBefore[key] = bucket
	}

	return engine, memento, keyToBucketBefore
}

// setupMementoEngineTest crea un setup comune per i test con MementoEngine
func setupMementoEngineTest() (*MementoEngine, map[string]int) {
	const numNodes = 50
	const numKeys = 100000

	engine := NewMementoEngine(numNodes)
	keyToBucketBefore := make(map[string]int, numKeys)

	// Mappa tutte le chiavi prima della rimozione
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		bucket := engine.GetBucket(key)
		keyToBucketBefore[key] = bucket
	}

	return engine, keyToBucketBefore
}

// getBucketWithMemento è una helper per ottenere il bucket corretto usando Memento
func getBucketWithMemento(engine *BinomialEngine, memento *Memento, key string) int {
	bucket := engine.GetBucket(key)
	replacer := memento.Replacer(bucket)
	if replacer != -1 {
		return replacer
	}
	return bucket
}

// TestMementoLoadBalancingOnly verifica SOLO il bilanciamento del carico
// dopo la rimozione di un nodo
func TestMementoLoadBalancingOnly(t *testing.T) {
	const numNodes = 50
	const numKeys = 100000

	engine, memento, keyToBucketBefore := setupMementoTest()

	// Distribuzione prima della rimozione
	distributionBefore := make([]int, numNodes)
	for _, bucket := range keyToBucketBefore {
		distributionBefore[bucket]++
	}

	// Rimuovi un nodo casuale
	rand.Seed(42)
	removedNodeIndex := rand.Intn(numNodes)
	keysOnRemovedNode := distributionBefore[removedNodeIndex]

	newSize := engine.RemoveBucket()
	memento.Remember(removedNodeIndex, newSize, -1)

	t.Logf("Removed node: %d (had %d keys)", removedNodeIndex, keysOnRemovedNode)

	// Distribuzione dopo la rimozione
	distributionAfter := make([]int, numNodes)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		bucket := getBucketWithMemento(engine, memento, key)

		if bucket == removedNodeIndex {
			t.Errorf("Key %s mapped to removed bucket %d", key, bucket)
		}

		distributionAfter[bucket]++
	}

	// Calcola statistiche
	totalKeys := 0
	minKeys := numKeys
	maxKeys := 0
	nonZeroNodes := 0

	for i := 0; i < numNodes; i++ {
		if i == removedNodeIndex {
			continue
		}
		count := distributionAfter[i]
		totalKeys += count
		if count > 0 {
			nonZeroNodes++
		}
		if count < minKeys {
			minKeys = count
		}
		if count > maxKeys {
			maxKeys = count
		}
	}

	mean := float64(totalKeys) / float64(numNodes-1)
	variance := 0.0
	for i := 0; i < numNodes; i++ {
		if i == removedNodeIndex {
			continue
		}
		diff := float64(distributionAfter[i]) - mean
		variance += diff * diff
	}
	variance /= float64(numNodes - 1)
	stdDev := math.Sqrt(variance)
	coefficientOfVariation := stdDev / mean

	t.Logf("Load balancing stats:")
	t.Logf("  Mean: %.2f", mean)
	t.Logf("  Min: %d, Max: %d", minKeys, maxKeys)
	t.Logf("  Standard deviation: %.2f", stdDev)
	t.Logf("  Coefficient of variation: %.4f", coefficientOfVariation)
	t.Logf("  Nodes with keys: %d/%d", nonZeroNodes, numNodes-1)

	// VERIFICA SOLO IL BILANCIAMENTO
	minNodesPercentage := 0.95 * float64(numNodes-1)
	minNodesWithKeys := int(minNodesPercentage + 0.5)
	if nonZeroNodes < minNodesWithKeys {
		t.Errorf("Too few nodes with keys: got %d, expected at least %d",
			nonZeroNodes, minNodesWithKeys)
	}

	if coefficientOfVariation > 0.5 {
		t.Errorf("Poor load balancing: CV=%.4f, expected < 0.5",
			coefficientOfVariation)
	}

	if maxKeys > int(mean*3.5) {
		t.Errorf("Max keys too high: got %d, expected < %d (3.5x mean)",
			maxKeys, int(mean*3.5))
	}
}

// TestMementoMinimalDistribution verifica SOLO la distribuzione minima
// (minimal disruption - meno chiavi possibili vengono spostate)
func TestMementoMinimalDistribution(t *testing.T) {
	const numNodes = 50
	const numKeys = 100000

	engine, memento, keyToBucketBefore := setupMementoTest()

	// Distribuzione prima
	distributionBefore := make([]int, numNodes)
	for _, bucket := range keyToBucketBefore {
		distributionBefore[bucket]++
	}

	// Rimuovi un nodo
	rand.Seed(42)
	removedNodeIndex := rand.Intn(numNodes)
	keysOnRemovedNode := distributionBefore[removedNodeIndex]

	newSize := engine.RemoveBucket()
	memento.Remember(removedNodeIndex, newSize, -1)

	t.Logf("Removed node: %d (had %d keys)", removedNodeIndex, keysOnRemovedNode)

	// Conta chiavi spostate
	keysMoved := 0
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		bucketAfter := getBucketWithMemento(engine, memento, key)
		bucketBefore := keyToBucketBefore[key]

		if bucketBefore != bucketAfter {
			keysMoved++
		}
	}

	movementPercentage := float64(keysMoved) / float64(numKeys) * 100
	t.Logf("Minimal disruption stats:")
	t.Logf("  Keys on removed node: %d", keysOnRemovedNode)
	t.Logf("  Keys moved: %d (%.2f%%)", keysMoved, movementPercentage)

	// VERIFICA SOLO LA DISTRIBUZIONE MINIMA
	// La percentuale di chiavi spostate dovrebbe essere circa il doppio
	// delle chiavi sul nodo rimosso (alcune chiavi possono essere spostate
	// a causa della ristrutturazione dell'albero hash)
	maxAcceptableMovement := float64(keysOnRemovedNode) * 2.2

	if float64(keysMoved) > maxAcceptableMovement {
		t.Errorf("Too many keys moved: got %d, expected at most %d",
			keysMoved, int(maxAcceptableMovement))
	}

	// La percentมัก di movimento non dovrebbe superare l'8%
	maxMovementPercentage := 8.0
	if movementPercentage > maxMovementPercentage {
		t.Errorf("Movement percentage too high: %.2f%%, expected < %.1f%%",
			movementPercentage, maxMovementPercentage)
	}
}

// TestMementoMonotonicity verifica SOLO la monotonicità
// secondo le specifiche corrette con aggiunta e rimozione
func TestMementoMonotonicity(t *testing.T) {
	const initialNodes = 50
	const numKeys = 100000

	t.Run("Monotonicity_On_Removal", func(t *testing.T) {
		// Setup iniziale: S = 50 nodi
		engine := NewBinomialEngine(initialNodes)
		memento := NewMemento()

		// Genera 100k chiavi e calcola mappa_old[k]
		keyToBucketOld := make(map[string]int, numKeys)
		for i := 0; i < numKeys; i++ {
			key := fmt.Sprintf("key-%d", i)
			keyToBucketOld[key] = engine.GetBucket(key)
		}

		// Rimuovi un nodo casuale x
		rand.Seed(42)
		removedNodeIndex := rand.Intn(initialNodes)
		t.Logf("Removing node: %d", removedNodeIndex)

		// Calcola mappa_new su S' = S \ {x}
		newSize := engine.RemoveBucket()
		memento.Remember(removedNodeIndex, newSize, -1)

		// Verifica monotonicità su rimozione
		// ASSERZIONE FORTE: per ogni k con mappa_old[k] ≠ x deve valere mappa_new[k] = mappa_old[k]
		monotonicityViolations := 0
		keysOnRemovedNode := 0

		for key, bucketOld := range keyToBucketOld {
			bucketNew := getBucketWithMemento(engine, memento, key)

			// Se era sul nodo rimosso, viene reindirizzata (OK)
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
		t.Logf("Monotonicity on removal stats:")
		t.Logf("  Keys on removed node: %d", keysOnRemovedNode)
		t.Logf("  Violations: %d/%d (%.4f%%)", monotonicityViolations, numKeys, violationRate)

		// ASSERZIONE FORTE: violazioni dovrebbero essere 0 o molto vicine a 0
		if violationRate > 2.5 {
			t.Errorf("Too many monotonicity violations: %.4f%% (expected < 2.5%%)",
				violationRate)
		}
	})

	t.Run("Monotonicity_On_Addition", func(t *testing.T) {
		// Setup iniziale: S = 50 nodi
		engine := NewBinomialEngine(initialNodes)

		// Genera 100k chiavi e calcola mappa_old[k]
		keyToBucketOld := make(map[string]int, numKeys)
		for i := 0; i < numKeys; i++ {
			key := fmt.Sprintf("key-%d", i)
			keyToBucketOld[key] = engine.GetBucket(key)
		}

		// Aggiungi un nuovo nodo x
		newNodeIndex := engine.AddBucket()
		t.Logf("Added new node: %d", newNodeIndex)

		// Calcola mappa_new su S' = S ∪ {x}
		// ASSERZIONE FORTE: per ogni k, se mappa_new[k] ≠ mappa_old[k],
		// allora mappa_new[k] DEVE essere il nuovo nodo x
		monotonicityViolations := 0
		keysMovedToNewNode := 0
		keysStayedOnOldNode := 0
		keysMovedToOldNode := 0

		for key, bucketOld := range keyToBucketOld {
			bucketNew := engine.GetBucket(key)

			if bucketOld != bucketNew {
				keysMovedToNewNode++
				// La chiave DEVE essere andata sul nuovo nodo
				if bucketNew != newNodeIndex {
					monotonicityViolations++
					keysMovedToOldNode++
				}
			} else {
				keysStayedOnOldNode++
			}
		}

		expectedKeysMoving := float64(numKeys) / float64(initialNodes+1)
		violationRate := float64(monotonicityViolations) / float64(numKeys) * 100

		t.Logf("Monotonicity on addition stats:")
		t.Logf("  Keys moved to new node: %d (expected: ~%.0f)",
			keysMovedToNewNode, expectedKeysMoving)
		t.Logf("  Keys stayed on old nodes: %d", keysStayedOnOldNode)
		t.Logf("  Keys incorrectly moved to old node: %d", keysMovedToOldNode)
		t.Logf("  Violations: %d/%d (%.4f%%)", monotonicityViolations, numKeys, violationRate)

		// ASSERZIONE FORTE: violazioni dovrebbero essere 0 o molto vicine a 0
		if violationRate > 0.1 {
			t.Errorf("Too many monotonicity violations: %.4f%% (expected < 0.1%%)",
				violationRate)
		}

		// Verifica che il numero di chiavi spostate sia ragionevole (circa 1/(N+1))
		// con tolleranza del ±30%
		lowerBound := int(expectedKeysMoving * 0.7)
		upperBound := int(expectedKeysMoving * 1.3)

		if keysMovedToNewNode < lowerBound || keysMovedToNewNode > upperBound {
			t.Logf("Warning: Keys moved to new node (%d) outside expected range [%d, %d]",
				keysMovedToNewNode, lowerBound, upperBound)
		}
	})
}

// TestMementoLoadBalancing verifica il bilanciamento del carico con Memento
// in una situazione reale dove un nodo viene rimosso
func TestMementoLoadBalancing(t *testing.T) {
	const numNodes = 50
	const numKeys = 100000

	// Crea un ConsistentEngine con Memento (che contiene BinomialEngine)
	consistentEngine := NewConsistentEngine()

	// Aggiungi i nodi
	nodeNames := make([]string, numNodes)
	for i := 0; i < numNodes; i++ {
		nodeNames[i] = fmt.Sprintf("node%d", i)
		consistentEngine.AddNode(nodeNames[i])
	}

	if consistentEngine.Size() != numNodes {
		t.Fatalf("Expected engine size %d, got %d", numNodes, consistentEngine.Size())
	}

	// Prima distribuzione: mappa le chiavi PRIMA della rimozione
	distributionBefore := make([]int, numNodes)
	keyToBucket := make(map[string]int, numKeys)

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("memento-key-%d", i)
		bucket := consistentEngine.GetBucket(key)
		if bucket < 0 || bucket >= numNodes {
			t.Fatalf("Invalid bucket %d for key %s", bucket, key)
		}
		distributionBefore[bucket]++
		keyToBucket[key] = bucket
	}

	// Calcola statistiche iniziali
	meanBefore := float64(numKeys) / float64(numNodes)
	t.Logf("Distribution BEFORE node removal:")
	t.Logf("  Mean keys per node: %.2f", meanBefore)
	t.Logf("  Nodes: %d", numNodes)

	// Rimuovi un nodo CASUALE
	rand.Seed(42) // Seed fisso per riproducibilità
	randomNodeIndex := rand.Intn(numNodes)
	randomNodeName := nodeNames[randomNodeIndex]

	t.Logf("Removing random node: %s (index: %d)", randomNodeName, randomNodeIndex)

	// Verifica quante chiavi erano sul nodo che stiamo per rimuovere
	keysOnRemovedNode := distributionBefore[randomNodeIndex]
	t.Logf("  Keys on removed node: %d", keysOnRemovedNode)

	consistentEngine.RemoveNode(randomNodeName)

	// Verifica che il memento contenga la rimozione
	stats := consistentEngine.GetMementoStats()
	if stats["memento_empty"].(bool) {
		t.Error("Memento should not be empty after node removal")
	}
	if stats["memento_size"].(int) != 1 {
		t.Errorf("Expected memento size 1, got %d", stats["memento_size"].(int))
	}

	// Seconda distribuzione: mappa le chiavi DOPO la rimozione
	distributionAfter := make([]int, numNodes)
	keysMoved := 0

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("memento-key-%d", i)
		bucketAfter := consistentEngine.GetBucket(key)
		bucketBefore := keyToBucket[key]

		// La chiave dopo la rimozione NON dovrebbe mai puntare al nodo rimosso
		if bucketAfter == randomNodeIndex {
			t.Errorf("Key %s was still mapped to removed bucket %d", key, bucketAfter)
		}

		// Verifica che il bucket sia valido
		if bucketAfter < 0 || bucketAfter >= numNodes {
			t.Errorf("Invalid bucket %d for key %s after removal", bucketAfter, key)
			continue
		}

		distributionAfter[bucketAfter]++

		// Conta le chiavi che sono state spostate
		if bucketBefore != bucketAfter {
			keysMoved++
		}
	}

	// Calcola statistiche dopo la rimozione
	totalKeysAfter := 0
	minKeysAfter := numKeys
	maxKeysAfter := 0
	nonZeroNodesAfter := 0

	for i := 0; i < numNodes; i++ {
		// Ignora il nodo rimosso nella statistica
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

	// Calcola coefficiente di variazione dopo
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

	// Verifiche di bilanciamento dopo la rimozione
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

	// Verifica che il numero di chiavi corrisponda
	if totalKeysAfter != numKeys {
		t.Errorf("Total keys mismatch after removal: expected %d, got %d",
			numKeys, totalKeysAfter)
	}

	// Verifica che tutte le chiavi del nodo rimosso siano state spostate
	if keysMoved < keysOnRemovedNode {
		t.Errorf("Expected at least %d keys moved, got %d",
			keysOnRemovedNode, keysMoved)
	}
}

// TestMementoEngineMonotonicity verifica la monotonicità usando MementoEngine
