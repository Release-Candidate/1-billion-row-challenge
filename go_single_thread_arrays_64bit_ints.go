// SPDX-FileCopyrightText:  Copyright 2024 Roland Csaszar
// SPDX-License-Identifier: MIT
//
// Project:  1-billion-row-challenge
// File:     go_single_thread_arrays_64bit_ints.go
// Date:     05.Mar.2024
//
// =============================================================================

package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
)

type stationTemperatures struct {
	TempSum []int
	Count   []uint
	Min     []int
	Max     []int
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Error: no data file to process given! Exiting.")
		os.Exit(1)
	}
	fileName := os.Args[1]
	content, err := os.ReadFile(fileName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	stationData := stationTemperatures{
		TempSum: make([]int, 10_000),
		Count:   make([]uint, 10_000),
		Min:     make([]int, 10_000),
		Max:     make([]int, 10_000),
	}

	stationIdxMap := make(map[string]int, 10_000)
	stationIdx := 0
	idx := 0
	// We suppose the file is valid, without a single error.
	// Not a single error check is made.
	for idx < len(content) {
		semiColonIdx := bytes.IndexByte(content[idx:], ';')
		// End of file.
		if semiColonIdx < 0 {
			break
		}
		station := content[idx : idx+semiColonIdx]
		newLineIdx := bytes.IndexByte(content[idx+semiColonIdx:], '\n')
		// End of file.
		if newLineIdx < 0 {
			newLineIdx = len(content) - (idx + semiColonIdx)
		}
		var temperature int = 0
		var negate int = 1
		for tmpIdx := idx + semiColonIdx + 1; tmpIdx < idx+semiColonIdx+newLineIdx; {
			currByte := content[tmpIdx]
			if currByte == '-' {
				negate = -1
			} else if currByte != '.' {
				intVal := currByte - '0'
				temperature = temperature*10 + int(intVal)
			}
			tmpIdx++
		}
		temperature *= negate

		stIdx, ok := stationIdxMap[string(station)]
		if ok {
			stationData.TempSum[stIdx] += temperature
			stationData.Count[stIdx]++
			stationData.Min[stIdx] = min(stationData.Min[stIdx], temperature)
			stationData.Max[stIdx] = max(stationData.Max[stIdx], temperature)
		} else {
			stationIdxMap[string(station)] = stationIdx
			stationData.TempSum[stationIdx] += temperature
			stationData.Count[stationIdx]++
			stationData.Min[stationIdx] = temperature
			stationData.Max[stationIdx] = temperature
			stationIdx++
		}

		idx += semiColonIdx + newLineIdx + 1
	}

	keys := make([]string, 0, 10_000)
	for key := range stationIdxMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	fmt.Printf("{")
	for i, station := range keys {
		if i > 0 {
			fmt.Printf(", ")
		}
		idx := stationIdxMap[station]
		meanF := float64(stationData.TempSum[idx]) / float64(stationData.Count[idx]*10)
		mean := fmt.Sprintf("%.1f", meanF)
		if mean == "-0.0" {
			mean = "0.0"
		}
		fmt.Printf("%s=%.1f/%s/%.1f", station, float32(stationData.Min[idx])*0.1, mean, float32(stationData.Max[idx])*0.1)
	}
	fmt.Printf("}\n")
}
