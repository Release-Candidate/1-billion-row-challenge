// SPDX-FileCopyrightText:  Copyright 2024 Roland Csaszar
// SPDX-License-Identifier: MIT
//
// Project:  1-billion-row-challenge
// File:     go_single_thread_single_parse_II.go
// Date:     05.Mar.2024
//
// =============================================================================

package main

import (
	"fmt"
	"math"
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
	// We suppose the file is valid, without a single error.
	// Not a single error check is made.
	for len(content) > 0 {
		station := [100]byte{}
		// Station name is not empty.
		semiColonIdx := 1
		station[0] = content[0]
		currByte := content[1]
		for currByte != ';' {
			station[semiColonIdx] = currByte
			semiColonIdx++
			currByte = content[semiColonIdx]
		}
		var temperature int = 0
		var negate = false
		if content[semiColonIdx+1] == '-' {
			negate = true
			content = content[semiColonIdx+2:]
		} else {
			content = content[semiColonIdx+1:]
		}
		// Either `N.N\n` or `NN.N\n`
		if content[1] == '.' {
			temperature = int(content[0])*10 + int(content[2]) - '0'*11
			content = content[4:]
		} else {
			temperature = int(content[0])*100 + int(content[1])*10 + int(content[3]) - '0'*111
			content = content[5:]
		}
		if negate {
			temperature *= -1
		}

		stIdx, ok := stationIdxMap[string(station[:semiColonIdx])]
		if ok {
			stationData.TempSum[stIdx] += temperature
			stationData.Count[stIdx]++
			stationData.Min[stIdx] = min(stationData.Min[stIdx], temperature)
			stationData.Max[stIdx] = max(stationData.Max[stIdx], temperature)
		} else {
			stationIdxMap[string(station[:semiColonIdx])] = stationIdx
			stationData.TempSum[stationIdx] = temperature
			stationData.Count[stationIdx] = 1
			stationData.Min[stationIdx] = temperature
			stationData.Max[stationIdx] = temperature
			stationIdx++
		}
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
		meanF := float64(stationData.TempSum[idx]) / float64(stationData.Count[idx])
		fmt.Printf("%s=%.1f/%.1f/%.1f", station,
			roundJava(float64(stationData.Min[idx])),
			roundJava(meanF),
			roundJava(float64(stationData.Max[idx])))
	}
	fmt.Printf("}\n")
}

func roundJava(x float64) float64 {
	rounded := math.Trunc(x)
	if x < 0.0 && rounded-x == 0.5 {
		// return
	} else if math.Abs(x-rounded) >= 0.5 {
		rounded += math.Copysign(1, x)
	}

	// oh, another hardcoded `-0.0` to `0.0` conversion.
	if rounded == 0 {
		return 0.0
	}

	return rounded / 10.0
}
