// modules/caddyhttp/reverseproxy/weighted_memento_selection_test.go
package reverseproxy

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/caddyserver/caddy/v2"
)

// createWeightedPool creates a pool of n upstream hosts for testing with specific weights.
func createWeightedPool(n int, weights []int) UpstreamPool {
	pool := make(UpstreamPool, n)
	for i := 0; i < n; i++ {
		pool[i] = &Upstream{
			Host: new(Host),
			Dial: fmt.Sprintf("localhost:%d", 8080+i),
		}
		pool[i].setHealthy(true)
	}
	return pool
}

// TestWeightedMementoSelectionDistribution verifies that keys are distributed according to weights.
func TestWeightedMementoSelectionDistribution(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	weights := []int{10, 1, 1, 5, 3}
	numHosts := len(weights)
	pool := createWeightedPool(numHosts, weights)

	// Create and provision the weighted memento policy
	policy := &WeightedMementoSelection{Field: "ip", Weights: weights}
	if err := policy.Provision(ctx); err != nil {
		t.Fatalf("Provision error: %v", err)
	}
	// Replace the default fallback with a selector that fails the test if called.
	policy.fallback = &failSelector{t: t}

	policy.PopulateInitialTopology(pool)

	// Generate a large number of test keys and record distribution
	const numTestKeys = 10000
	distribution := make(map[*Upstream]int)
	for i := 0; i < numTestKeys; i++ {
		key := fmt.Sprintf("192.168.1.%d:%d", i%256, i)
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = key

		host := policy.Select(pool, req, nil)
		if host == nil {
			t.Fatalf("Expected host selection for key %s", key)
		}
		distribution[host]++
	}

	// Calculate total weight
	totalWeight := 0
	for _, w := range weights {
		totalWeight += w
	}

	// Verify the distribution is proportional to the weights
	t.Logf("Distribution results for %d keys:", numTestKeys)
	for i, host := range pool {
		hostWeight := weights[i]
		count := distribution[host]
		expectedRatio := float64(hostWeight) / float64(totalWeight)
		actualRatio := float64(count) / float64(numTestKeys)

		// Allow for a tolerance in distribution
		tolerance := 0.05
		if actualRatio < (expectedRatio-tolerance) || actualRatio > (expectedRatio+tolerance) {
			t.Errorf("Host %s (Weight %d): Expected ratio around %.3f, got %.3f (Count: %d)",
				host.Dial, hostWeight, expectedRatio, actualRatio, count)
		} else {
			t.Logf("Host %s (Weight %d): Ratio %.3f (Count: %d) - OK",
				host.Dial, hostWeight, actualRatio, count)
		}
	}
}

// TestWeightedMementoSelectionRemoval verifies correct remapping after a host is removed.
func TestWeightedMementoSelectionRemoval(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	weights := []int{10, 2, 8}
	numHosts := len(weights)
	pool := createWeightedPool(numHosts, weights)

	// Create and provision the weighted memento policy
	policy := &WeightedMementoSelection{Field: "ip", Weights: weights}
	if err := policy.Provision(ctx); err != nil {
		t.Fatalf("Provision error: %v", err)
	}
	// Replace the default fallback with a selector that fails the test if called.
	policy.fallback = &failSelector{t: t}

	policy.PopulateInitialTopology(pool)

	// Find a key that maps to the host we will remove (host index 1, weight 2)
	hostToRemove := pool[1]
	var keyMappedToRemovedHost string
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("10.0.0.%d", i)
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = key
		if policy.Select(pool, req, nil) == hostToRemove {
			keyMappedToRemovedHost = key
			break
		}
	}
	if keyMappedToRemovedHost == "" {
		t.Fatalf("Could not find a key that maps to the host being removed. Cannot proceed.")
	}

	// Remove the host via an unhealthy event
	policy.handleUnhealthyEvent(context.Background(), caddy.Event{
		Data: map[string]any{"host": hostToRemove.String()},
	})

	// Verify the key is remapped to a different, active host
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = keyMappedToRemovedHost
	newHost := policy.Select(pool, req, nil)

	if newHost == nil {
		t.Fatalf("Key %s was not remapped to any host after removal.", keyMappedToRemovedHost)
	}
	if newHost == hostToRemove {
		t.Fatalf("Key %s is still mapped to the removed host %s.", keyMappedToRemovedHost, hostToRemove.Dial)
	}
	if newHost != pool[0] && newHost != pool[2] {
		t.Errorf("Key %s was remapped to an unexpected host: %s", keyMappedToRemovedHost, newHost.Dial)
	} else {
		t.Logf("Key %s successfully remapped from %s to %s.", keyMappedToRemovedHost, hostToRemove.Dial, newHost.Dial)
	}
}

// TestWeightedMementoSelectionRemovalAndRestore verifies that mappings are restored after a host is brought back online.
func TestWeightedMementoSelectionRemovalAndRestore(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	weights := []int{5, 8, 3}
	numHosts := len(weights)
	pool := createWeightedPool(numHosts, weights)

	// Create and provision the weighted memento policy
	policy := &WeightedMementoSelection{Field: "ip", Weights: weights}
	if err := policy.Provision(ctx); err != nil {
		t.Fatalf("Provision error: %v", err)
	}
	// Replace the default fallback with a selector that fails the test if called.
	policy.fallback = &failSelector{t: t}

	policy.PopulateInitialTopology(pool)

	// Store initial mappings for a set of keys
	const numTestKeys = 200
	initialMappings := make(map[string]*Upstream)
	for i := 0; i < numTestKeys; i++ {
		key := fmt.Sprintf("172.16.1.%d", i)
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = key
		host := policy.Select(pool, req, nil)
		if host == nil {
			t.Fatalf("Initial selection failed for key %s", key)
		}
		initialMappings[key] = host
	}

	// Remove a host
	hostToRemove := pool[1]
	policy.handleUnhealthyEvent(context.Background(), caddy.Event{
		Data: map[string]any{"host": hostToRemove.String()},
	})

	// Restore the host
	policy.handleHealthyEvent(context.Background(), caddy.Event{
		Data: map[string]any{"host": hostToRemove.String()},
	})

	// Verify that all mappings have been restored to their original state
	restorationFailures := 0
	for key, originalHost := range initialMappings {
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = key
		currentHost := policy.Select(pool, req, nil)
		if currentHost == nil {
			t.Errorf("Key %s failed to select any host after restoration.", key)
			restorationFailures++
			continue
		}
		if currentHost != originalHost {
			t.Errorf("Key %s: Mapping not restored. Expected %s, got %s.", key, originalHost.Dial, currentHost.Dial)
			restorationFailures++
		}
	}

	if restorationFailures > 0 {
		t.Fatalf("Restoration failed for %d out of %d keys.", restorationFailures, numTestKeys)
	} else {
		t.Logf("Successfully restored all %d key mappings.", numTestKeys)
	}
}

// TestWeightedMementoSelectionLoadBalancing verifies the fairness of key distribution according to weights.
func TestWeightedMementoSelectionLoadBalancing(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	weights := []int{50, 30, 20}
	numHosts := len(weights)
	pool := createWeightedPool(numHosts, weights)

	policy := &WeightedMementoSelection{Field: "ip", Weights: weights}
	if err := policy.Provision(ctx); err != nil {
		t.Fatalf("Provision error: %v", err)
	}
	policy.fallback = &failSelector{t: t}
	policy.PopulateInitialTopology(pool)

	const numKeys = 100000
	distribution := make(map[*Upstream]int)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("balance-key-%d", i)
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = key
		host := policy.Select(pool, req, nil)
		distribution[host]++
	}

	totalWeight := 0
	for _, w := range weights {
		totalWeight += w
	}

	t.Logf("Load balancing results for %d keys:", numKeys)
	maxDeviation := 0.0
	for i, host := range pool {
		hostWeight := weights[i]
		count := distribution[host]
		expectedCount := float64(hostWeight) / float64(totalWeight) * float64(numKeys)
		deviation := (float64(count) - expectedCount) / expectedCount * 100

		if deviation < 0 {
			deviation = -deviation
		}
		if deviation > maxDeviation {
			maxDeviation = deviation
		}

		t.Logf("Host %s (Weight %d): %d keys (Expected: %.0f, Deviation: %.2f%%)",
			host.Dial, hostWeight, count, expectedCount, deviation)
	}

	const tolerance = 15.0
	if maxDeviation > tolerance {
		t.Errorf("Load balancing deviation exceeds tolerance of %.2f%%. Max deviation was %.2f%%.",
			tolerance, maxDeviation)
	} else {
		t.Logf("Maximum load deviation (%.2f%%) is within the %.2f%% tolerance.", maxDeviation, tolerance)
	}
}

// TestWeightedMementoSelectionMonotonicity verifies that when a new host is added,
// keys either stay on their current host or move to the new host.
func TestWeightedMementoSelectionMonotonicity(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	weights := []int{10, 20, 5}
	numHosts := len(weights)
	pool := createWeightedPool(numHosts, weights)

	policy := &WeightedMementoSelection{Field: "ip", Weights: weights}
	if err := policy.Provision(ctx); err != nil {
		t.Fatalf("Provision error: %v", err)
	}
	policy.fallback = &failSelector{t: t}
	policy.PopulateInitialTopology(pool)

	const numKeys = 10000
	mappaOld := make(map[string]*Upstream, numKeys)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("monotonicity-key-%d", i)
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = key
		host := policy.Select(pool, req, nil)
		mappaOld[key] = host
	}

	// Add a new host
	newHost := &Upstream{
		Host: new(Host),
		Dial: fmt.Sprintf("localhost:%d", 8080+numHosts),
	}
	newHost.setHealthy(true)
	// The newPool variable is not strictly necessary for the policy's selection logic
	// after the event, but it's good practice to keep a consistent view of the pool.
	newPool := append(pool, newHost)
	policy.Weights = append(weights, 15) // Add weight for the new host

	// Notify the policy about the new healthy host. This updates the internal hash ring.
	policy.handleHealthyEvent(context.Background(), caddy.Event{
		Data: map[string]any{"host": newHost.String()},
	})

	violations := 0
	for key, oldHost := range mappaOld {
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = key
		// Select from the updated pool. The policy's internal ring now includes the new host.
		currentHost := policy.Select(newPool, req, nil)
		if currentHost == nil {
			t.Errorf("Monotonicity violation: key %s failed to map to any host after addition", key)
			violations++
			continue
		}

		if currentHost != oldHost && currentHost != newHost {
			violations++
			t.Errorf("Monotonicity violation: key %s moved from %s to %s (expected %s or %s)",
				key, oldHost.Dial, currentHost.Dial, oldHost.Dial, newHost.Dial)
		}
	}

	if violations > 0 {
		t.Fatalf("Monotonicity property violated for %d keys", violations)
	} else {
		t.Logf("Monotonicity property maintained for all %d keys after host addition.", numKeys)
	}
}

// TestWeightedMementoSelectionMinimalDisruption verifies that when a host is removed,
// only keys that were on that host are remapped.
func TestWeightedMementoSelectionMinimalDisruption(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	weights := []int{10, 2, 8}
	numHosts := len(weights)
	pool := createWeightedPool(numHosts, weights)

	policy := &WeightedMementoSelection{Field: "ip", Weights: weights}
	if err := policy.Provision(ctx); err != nil {
		t.Fatalf("Provision error: %v", err)
	}
	policy.fallback = &failSelector{t: t}
	policy.PopulateInitialTopology(pool)

	const numKeys = 10000
	mappaOld := make(map[string]*Upstream, numKeys)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("disruption-key-%d", i)
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = key
		host := policy.Select(pool, req, nil)
		mappaOld[key] = host
	}

	// Remove a host
	hostToRemove := pool[1]
	policy.handleUnhealthyEvent(context.Background(), caddy.Event{
		Data: map[string]any{"host": hostToRemove.String()},
	})

	violations := 0
	for key, oldHost := range mappaOld {
		if oldHost == hostToRemove {
			continue // This key is expected to be remapped.
		}

		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = key
		newHost := policy.Select(pool, req, nil)
		if newHost == nil {
			t.Errorf("Minimal Disruption violation: key %s failed to map after removal", key)
			violations++
			continue
		}

		if newHost != oldHost {
			violations++
			t.Errorf("Minimal Disruption violation: key %s moved from %s to %s (was not on removed host %s)",
				key, oldHost.Dial, newHost.Dial, hostToRemove.Dial)
		}
	}

	if violations > 0 {
		t.Fatalf("Minimal Disruption property violated for %d keys", violations)
	} else {
		t.Logf("Minimal Disruption property maintained for all keys not on the removed host.")
	}
}

// failSelector is a selection policy that always fails the test.
// It's used to ensure that the fallback policy is never called.
type failSelector struct {
	t *testing.T
}

func (fs *failSelector) Select(pool UpstreamPool, r *http.Request, w http.ResponseWriter) *Upstream {
	fs.t.Helper()
	fs.t.Fatal("Fallback selector was called unexpectedly. This indicates the primary lookup failed.")
	return nil
}
