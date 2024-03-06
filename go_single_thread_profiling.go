// SPDX-FileCopyrightText:  Copyright 2024 Roland Csaszar
// SPDX-License-Identifier: MIT
//
// Project:  1-billion-row-challenge
// File:     go_single_thread_profiling.go
// Date:     05.Mar.2024
//
// =============================================================================

package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"runtime/pprof"
	"sort"
)

type stationTemperatures struct {
	TempSum []int
	Count   []uint
	Min     []int
	Max     []int
}

func readFile(fileName string) []byte {
	content, err := os.ReadFile(fileName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	return content
}

func parseStationName(content []byte, idx int) (int, []byte) {
	semiColonIdx := 0
	station := [100]byte{}
	currByte := content[idx]
	for currByte != ';' {
		station[semiColonIdx] = currByte
		semiColonIdx++
		currByte = content[idx+semiColonIdx]
	}
	return semiColonIdx, station[:semiColonIdx]
}

func parseTemperature(idx int, semiColonIdx int, content []byte) (int, int) {
	var temperature int = 0
	var negate int = 1
	tmpIdx := idx + semiColonIdx + 1
	newLineIdx := 0
Loop:
	for tmpIdx < len(content) {
		currByte := content[tmpIdx]
		tmpIdx++
		newLineIdx++
		switch currByte {
		case '-':
			negate = -1
		case '\n':
			break Loop
		case '.':
			continue
		default:
			intVal := currByte - '0'
			temperature = temperature*10 + int(intVal)
		}
	}
	temperature *= negate
	return temperature, newLineIdx
}

func addTemperatureData(stationIdxMap *map[string]int, station []byte, stationData *stationTemperatures, temperature int, stationIdx int) int {
	stIdx, ok := (*stationIdxMap)[string(station)]
	if ok {
		stationData.TempSum[stIdx] += temperature
		stationData.Count[stIdx]++
		stationData.Min[stIdx] = min(stationData.Min[stIdx], temperature)
		stationData.Max[stIdx] = max(stationData.Max[stIdx], temperature)
	} else {
		(*stationIdxMap)[string(station)] = stationIdx
		stationData.TempSum[stationIdx] += temperature
		stationData.Count[stationIdx]++
		stationData.Min[stationIdx] = temperature
		stationData.Max[stationIdx] = temperature
		stationIdx++
	}

	return stationIdx
}

func sortStationNames(stationIdxMap map[string]int) []string {
	keys := make([]string, 0, 10_000)
	for key := range stationIdxMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func printSolution(keys []string, stationIdxMap map[string]int, stationData stationTemperatures) {
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

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Error: no data file to process given! Exiting.")
		os.Exit(1)
	}
	f, err := os.Create("cpu.prof")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	fileName := os.Args[1]
	content := readFile(fileName)

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
		semiColonIdx, station := parseStationName(content, idx)
		temperature, newLineIdx := parseTemperature(idx, semiColonIdx, content)
		stationIdx = addTemperatureData(&stationIdxMap, station, &stationData, temperature, stationIdx)
		idx += semiColonIdx + newLineIdx + 1
	}

	keys := sortStationNames(stationIdxMap)

	printSolution(keys, stationIdxMap, stationData)
}
