# Consistent Hashing Test Documentation

This document describes the three fundamental tests used to validate consistent hashing implementations. These tests evaluate key properties that ensure proper load distribution and minimal disruption when the cluster topology changes.

## Overview

The tests operate on the following abstract model:
- **S**: A set of nodes in the cluster
- **K**: A set of keys to be distributed
- **f_S(k)**: A hash function that maps key `k` to a node in set `S`

## Test Structure

### 1. Load Balancing Test

**Purpose**: Verifies that keys are distributed uniformly across all nodes in the cluster.

**Test Structure**:
1. Initialize a cluster with `N` nodes
2. Generate `K` distinct keys
3. For each key `k`, compute `f_S(k)` to determine which node it maps to
4. Count how many keys map to each node
5. Calculate distribution statistics

**What It Evaluates**:
- **Uniformity**: Each node should receive approximately `K/N` keys
- **Distribution Quality**: The coefficient of variation (CV) measures the relative standard deviation of the distribution
- **Expected CV**: For a uniform distribution, the expected CV is approximately `√[(N-1)/K]`
- **Validation**: The observed CV must be within 20% of the expected CV

**Mathematical Model**:
- Expected keys per node: `μ = K / N`
- Expected standard deviation: `σ = √(K · (1/N) · (1 - 1/N))`
- Coefficient of variation: `CV = σ_observed / μ_observed`
- Expected CV: `CV_expected ≈ √[(N-1)/K]`
- Pass condition: `CV ≤ CV_expected × 1.2`

**Key Metrics**:
- Mean keys per node (should be close to `K/N`)
- Standard deviation across nodes
- Coefficient of variation (measures distribution uniformity)

---

### 2. Monotonicity Test

**Purpose**: Ensures that when a new node is added to the cluster, keys either remain on their current node or move to the newly added node. Keys should never move to existing nodes.

**Test Structure**:
1. Initialize a cluster with set `S` containing `N` nodes
2. Generate `K` distinct keys
3. For each key `k`, compute `map_old[k] = f_S(k)` to establish the initial mapping
4. Add a new node `x` to create set `S' = S ∪ {x}`
5. For each key `k`, compute `map_new[k] = f_S'(k)` to get the new mapping
6. Verify the monotonicity property

**What It Evaluates**:
- **Monotonicity Property**: For every key `k`, if `map_new[k] ≠ map_old[k]`, then `map_new[k] = x`
- This ensures that when a node is added, keys only move to the new node, never to existing nodes
- Keys that don't move remain on their original node

**Mathematical Formulation**:
- Initial state: `map_old[k] = f_S(k)` for all `k ∈ K`
- After adding node `x`: `map_new[k] = f_S'(k)` where `S' = S ∪ {x}`
- Verification: `∀k ∈ K: (map_new[k] ≠ map_old[k]) → (map_new[k] = x)`

**Key Assertion**:
- If a key changes its mapping, it must move to the newly added node
- No key should move from one existing node to another existing node

---

### 3. Minimal Disruption Test

**Purpose**: Ensures that when a node is removed from the cluster, only keys that were mapped to the removed node are remapped. All other keys must remain on their original nodes.

**Test Structure**:
1. Initialize a cluster with set `S` containing `N` nodes
2. Generate `K` distinct keys
3. For each key `k`, compute `map_old[k] = f_S(k)` to establish the initial mapping
4. Remove node `x` to create set `S' = S \ {x}`
5. For each key `k`, compute `map_new[k] = f_S'(k)` to get the new mapping
6. Verify the minimal disruption property

**What It Evaluates**:
- **Minimal Disruption Property**: For every key `k`, if `map_old[k] ≠ x`, then `map_new[k] = map_old[k]`
- This ensures that only keys originally mapped to the removed node are affected
- All other keys remain unchanged, minimizing disruption to the system

**Mathematical Formulation**:
- Initial state: `map_old[k] = f_S(k)` for all `k ∈ K`
- After removing node `x`: `map_new[k] = f_S'(k)` where `S' = S \ {x}`
- Verification: `∀k ∈ K: (map_old[k] ≠ x) → (map_new[k] = map_old[k])`

**Key Assertion**:
- Keys not mapped to the removed node must remain on their original node
- Only keys mapped to the removed node are allowed to be remapped

---

## Test Relationships

These three tests evaluate complementary properties:

- **Load Balancing**: Ensures fair distribution of keys across nodes
- **Monotonicity**: Ensures predictable behavior when adding nodes
- **Minimal Disruption**: Ensures minimal remapping when removing nodes

Together, they validate that a consistent hashing implementation provides:
1. Uniform load distribution
2. Predictable key movement patterns during cluster growth
3. Minimal disruption during cluster shrinkage

## Test Parameters

Typical test configurations:
- **Number of nodes (N)**: 50
- **Number of keys (K)**: 100,000
- **Tolerance**: 20% margin for coefficient of variation in load balancing tests

These parameters provide sufficient statistical significance while maintaining reasonable test execution time.

