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

package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type BenchmarkResult struct {
	TestName    string
	Algorithm   string
	TimeNs      float64
	MemoryBytes int
	Allocations int
	Scenario    string
}

func main() {
	fmt.Println("Running benchmark comparison and generating CSV...")

	// Run all benchmark tests
	benchmarks := []string{
		"BenchmarkRendezvousVsMemento_SameKey",
		"BenchmarkRendezvousVsMemento_DifferentKeys",
		"BenchmarkRendezvousVsMemento_EventDrivenPerformance",
		"BenchmarkRendezvousVsMemento_URIHash",
		"BenchmarkRendezvousVsMemento_HeaderHash",
		"BenchmarkRendezvousVsMemento_DifferentPoolSizes",
		"BenchmarkRendezvous_PoolSizes",
		"BenchmarkRendezvousVsMemento_MemoryAllocation",
		"BenchmarkRendezvousVsMemento_ConcurrentAccess",
		"BenchmarkRendezvousVsMemento_RealisticConcurrent",
		"BenchmarkRendezvousVsMemento_ConsistencyCheck",
		"BenchmarkMementoVsRendezvous_WithRemovedNodes",
		"BenchmarkMementoVsRendezvous_100Nodes_ProgressiveRemoval",
	}

	var results []BenchmarkResult

	for _, benchmark := range benchmarks {
		fmt.Printf("Running %s...\n", benchmark)
		benchmarkResults := runBenchmark(benchmark)
		results = append(results, benchmarkResults...)
	}

	// Generate CSV
	err := generateCSV(results)
	if err != nil {
		fmt.Printf("Error generating CSV: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("CSV file 'benchmark_results.csv' generated successfully!")
	fmt.Printf("Total results: %d\n", len(results))
}

func runBenchmark(benchmarkPattern string) []BenchmarkResult {
	cmd := exec.Command("go", "test",
		"-bench", benchmarkPattern, "-benchmem", "-run", "^$")
	cmd.Dir = "/home/massimo.saia/sw/caddy/modules/caddyhttp/reverseproxy"

	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error running benchmark %s: %v\n", benchmarkPattern, err)
		return nil
	}

	return parseBenchmarkOutput(string(output), benchmarkPattern)
}

func parseBenchmarkOutput(output, pattern string) []BenchmarkResult {
	var results []BenchmarkResult

	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if strings.Contains(line, "Benchmark") && strings.Contains(line, "ns/op") {
			result := parseBenchmarkLine(line, pattern)
			if result != nil {
				results = append(results, *result)
			}
		}
	}

	return results
}

func parseBenchmarkLine(line, pattern string) *BenchmarkResult {
	// Regex to match benchmark output format:
	// BenchmarkRendezvousVsMemento_SameKey/Rendezvous_IPHash_SameKey-6         	 5766648	       204.7 ns/op	       0 B/op	       0 allocs/op

	re := regexp.MustCompile(`Benchmark([^/]+)/([^-]+)-(\d+)\s+(\d+)\s+([\d.]+)\s+ns/op\s+(\d+)\s+B/op\s+(\d+)\s+allocs/op`)
	matches := re.FindStringSubmatch(line)

	if len(matches) < 8 {
		return nil
	}

	testName := matches[2] // Extract the actual test name after the slash
	timeNs, _ := strconv.ParseFloat(matches[5], 64)
	memoryBytes, _ := strconv.Atoi(matches[6])
	allocations, _ := strconv.Atoi(matches[7])

	// Determine algorithm and scenario
	algorithm, scenario := parseTestName(testName, pattern)

	return &BenchmarkResult{
		TestName:    testName,
		Algorithm:   algorithm,
		TimeNs:      timeNs,
		MemoryBytes: memoryBytes,
		Allocations: allocations,
		Scenario:    scenario,
	}
}

func parseTestName(testName, pattern string) (algorithm, scenario string) {
	// Parse algorithm type based on test name
	if strings.Contains(testName, "Rendezvous") {
		algorithm = "Rendezvous"
	} else if strings.Contains(testName, "Memento") {
		algorithm = "Memento"
	} else {
		algorithm = "Unknown"
	}

	// Parse scenario based on pattern and test name
	switch pattern {
	case "BenchmarkRendezvousVsMemento_SameKey":
		scenario = "Same Key"
	case "BenchmarkRendezvousVsMemento_DifferentKeys":
		scenario = "Different Keys"
	case "BenchmarkRendezvousVsMemento_EventDrivenPerformance":
		if strings.Contains(testName, "WithTopologyChanges") {
			scenario = "Event-Driven with Topology Changes"
		} else {
			scenario = "Event-Driven Performance"
		}
	case "BenchmarkRendezvousVsMemento_URIHash":
		if strings.Contains(testName, "SameURI") {
			scenario = "Same URI"
		} else {
			scenario = "Different URIs"
		}
	case "BenchmarkRendezvousVsMemento_HeaderHash":
		if strings.Contains(testName, "SameHeader") {
			scenario = "Same Header"
		} else {
			scenario = "Different Headers"
		}
	case "BenchmarkRendezvousVsMemento_DifferentPoolSizes":
		scenario = "Pool Size Scalability"
	case "BenchmarkRendezvous_PoolSizes":
		scenario = "Pool Size Scalability"
	case "BenchmarkRendezvousVsMemento_MemoryAllocation":
		scenario = "Memory Allocation"
	case "BenchmarkRendezvousVsMemento_ConcurrentAccess":
		scenario = "Concurrent Access"
	case "BenchmarkRendezvousVsMemento_RealisticConcurrent":
		scenario = "Realistic Concurrent"
	case "BenchmarkRendezvousVsMemento_ConsistencyCheck":
		scenario = "Consistency Check"
	case "BenchmarkMementoVsRendezvous_WithRemovedNodes":
		scenario = "Fixed Removals"
	case "BenchmarkMementoVsRendezvous_100Nodes_ProgressiveRemoval":
		scenario = "Progressive Removals"
	default:
		scenario = "Unknown"
	}

	return algorithm, scenario
}

func generateCSV(results []BenchmarkResult) error {
	file, err := os.Create("benchmark_results.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	// Write CSV header
	_, err = file.WriteString("TestName,Algorithm,TimeNs,MemoryBytes,Allocations,Scenario\n")
	if err != nil {
		return err
	}

	// Write data rows
	for _, result := range results {
		line := fmt.Sprintf("%s,%s,%.2f,%d,%d,%s\n",
			result.TestName,
			result.Algorithm,
			result.TimeNs,
			result.MemoryBytes,
			result.Allocations,
			result.Scenario)

		_, err = file.WriteString(line)
		if err != nil {
			return err
		}
	}

	return nil
}
