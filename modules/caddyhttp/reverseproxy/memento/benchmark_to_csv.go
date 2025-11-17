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

//go:build ignore
// +build ignore

package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <benchmark_output_file> [output_csv_file]\n", os.Args[0])
		os.Exit(1)
	}

	inputFile := os.Args[1]
	outputFile := "benchmark_results.csv"
	if len(os.Args) > 2 {
		outputFile = os.Args[2]
	}

	file, err := os.Open(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	csvFile, err := os.Create(outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating CSV file: %v\n", err)
		os.Exit(1)
	}
	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)
	defer writer.Flush()

	// Write CSV header
	writer.Write([]string{
		"Benchmark",
		"NsPerOp",
		"AllocBytes",
		"AllocsPerOp",
		"CPU",
		"GOOS",
		"GOARCH",
	})

	// Regex to parse benchmark output
	// Format: BenchmarkName-N              N    N ns/op        N B/op        N allocs/op
	// Handle both formats: with and without scientific notation
	benchRegex := regexp.MustCompile(`^([\w/]+)-(\d+)\s+(\d+)\s+([\d.eE+-]+)\s+ns/op\s+(\d+)\s+B/op\s+(\d+)\s+allocs/op$`)

	// Parse system info
	scanner := bufio.NewScanner(file)
	var cpu, goos, goarch string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Parse system info
		if strings.HasPrefix(line, "goos: ") {
			goos = strings.TrimPrefix(line, "goos: ")
		} else if strings.HasPrefix(line, "goarch: ") {
			goarch = strings.TrimPrefix(line, "goarch: ")
		} else if strings.HasPrefix(line, "cpu: ") {
			cpu = strings.TrimPrefix(line, "cpu: ")
		} else if benchRegex.MatchString(line) {
			matches := benchRegex.FindStringSubmatch(line)
			if len(matches) == 7 {
				benchmark := matches[1]
				nsPerOp, _ := strconv.ParseFloat(matches[4], 64)
				allocBytes, _ := strconv.Atoi(matches[5])
				allocsPerOp, _ := strconv.Atoi(matches[6])

				writer.Write([]string{
					benchmark,
					fmt.Sprintf("%.6f", nsPerOp),
					strconv.Itoa(allocBytes),
					strconv.Itoa(allocsPerOp),
					cpu,
					goos,
					goarch,
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("CSV file created: %s\n", outputFile)
}
