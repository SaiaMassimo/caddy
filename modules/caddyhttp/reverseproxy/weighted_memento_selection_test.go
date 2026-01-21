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
	distribution := make(map[string]int)
	for i := 0; i < numTestKeys; i++ {
		key := fmt.Sprintf("192.168.1.%d:%d", i%256, i)
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = key

		host := policy.Select(pool, req, nil)
		if host == nil {
			t.Fatalf("Expected host selection for key %s", key)
		}
		distribution[host.Dial]++
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
		count := distribution[host.Dial]
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
	initialMappings := make(map[string]string)
	for i := 0; i < numTestKeys; i++ {
		key := fmt.Sprintf("172.16.1.%d", i)
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = key
		host := policy.Select(pool, req, nil)
		if host == nil {
			t.Fatalf("Initial selection failed for key %s", key)
		}
		initialMappings[key] = host.Dial
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
	for key, originalHostDial := range initialMappings {
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = key
		currentHost := policy.Select(pool, req, nil)
		if currentHost == nil {
			t.Errorf("Key %s failed to select any host after restoration.", key)
			restorationFailures++
			continue
		}
		if currentHost.Dial != originalHostDial {
			t.Errorf("Key %s: Mapping not restored. Expected %s, got %s.", key, originalHostDial, currentHost.Dial)
			restorationFailures++
		}
	}

	if restorationFailures > 0 {
		t.Fatalf("Restoration failed for %d out of %d keys.", restorationFailures, numTestKeys)
	} else {
		t.Logf("Successfully restored all %d key mappings.", numTestKeys)
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
