// SPDX-FileCopyrightText:  Copyright 2024 Roland Csaszar
// SPDX-License-Identifier: MIT
//
// Project:  1-billion-row-challenge
// File:     go_parallel_thread_factor.go
// Date:     06.Mar.2024
//
// =============================================================================

package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
)

type stationTemperatures struct {
	TempSum []int
	Count   []uint
	Min     []int
	Max     []int
}

type chunk struct {
	StartIdx int64
	EndIdx   int64
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Error: no data file to process given! Exiting.")
		os.Exit(1)
	}
	numCPUs := 2 * runtime.NumCPU()
	fileName := os.Args[1]

	file, err := os.Open(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file '%s':\n%s\n", fileName, err)
		os.Exit(2)
	}
	defer file.Close()

	fsInfo, err := file.Stat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting data of file '%s':\n%s\n", fileName, err)
		os.Exit(3)
	}

	size := fsInfo.Size()
	chunkSize := size / int64(numCPUs)

	chunkList := make([]chunk, 0, numCPUs)
	chunkList = append(chunkList, chunk{
		StartIdx: 0,
		EndIdx:   size - 1,
	})

	var readOff int64 = chunkSize
	buffer := make([]byte, 150)
	for cpuIdx := 1; cpuIdx < numCPUs; cpuIdx++ {
		_, err = file.ReadAt(buffer, readOff)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file '%s' for chunking:\n%s\n", fileName, err)
			os.Exit(4)
		}
		newlineIdx := bytes.IndexByte(buffer, '\n')
		if newlineIdx < 0 {
			chunkList[cpuIdx-1].EndIdx = size - 1
			break
		}
		chunkList = append(chunkList, chunk{
			StartIdx: readOff + int64(newlineIdx) + 1,
			EndIdx:   size - 1,
		})
		chunkList[cpuIdx-1].EndIdx = readOff + int64(newlineIdx)
		readOff += chunkSize
	}

	stationSumData := stationTemperatures{
		TempSum: make([]int, 10_000),
		Count:   make([]uint, 10_000),
		Min:     make([]int, 10_000),
		Max:     make([]int, 10_000),
	}
	stationSumIdxMap := make(map[string]int, 10_000)

	channels := make([]chan resultType, numCPUs)

	for idx, chunk := range chunkList {
		// blocking channels
		channels[idx] = make(chan resultType)
		go processChunk(chunk, file, channels[idx])
	}

	for _, channel := range channels {
		result := <-channel
		stationData := result.Temps
		stationIdxMap := result.IdxMap

		stationIdx := 0
		for station, idx := range stationIdxMap {
			stIdx, ok := stationSumIdxMap[station]
			if ok {
				stationSumData.TempSum[stIdx] += stationData.TempSum[idx]
				stationSumData.Count[stIdx] += stationData.Count[idx]
				stationSumData.Min[stIdx] = min(stationData.Min[idx], stationSumData.Min[stIdx])
				stationSumData.Max[stIdx] = max(stationData.Max[idx], stationSumData.Max[stIdx])
			} else {
				stationSumIdxMap[station] = stationIdx
				stationSumData.TempSum[stationIdx] = stationData.TempSum[idx]
				stationSumData.Count[stationIdx] = stationData.Count[idx]
				stationSumData.Min[stationIdx] = stationData.Min[idx]
				stationSumData.Max[stationIdx] = stationData.Max[idx]
				stationIdx++
			}
		}
	}

	keys := make([]string, 0, 10_000)
	for key := range stationSumIdxMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	fmt.Printf("{")
	for i, station := range keys {
		if i > 0 {
			fmt.Printf(", ")
		}
		idx := stationSumIdxMap[station]
		meanF := float64(stationSumData.TempSum[idx]) / float64(stationSumData.Count[idx])
		fmt.Printf("%s=%.1f/%.1f/%.1f", station,
			roundJava(float64(stationSumData.Min[idx])),
			roundJava(meanF),
			roundJava(float64(stationSumData.Max[idx])))
	}
	fmt.Printf("}\n")
}

type resultType struct {
	Temps  stationTemperatures
	IdxMap map[string]int
}

func processChunk(chunk chunk, file *os.File, channel chan resultType) {
	content := make([]byte, chunk.EndIdx-chunk.StartIdx+1)
	_, err := file.ReadAt(content, chunk.StartIdx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading data file at offset %d, len %d:\n%s\n",
			chunk.StartIdx, chunk.EndIdx-chunk.StartIdx+1, err)
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
	channel <- resultType{Temps: stationData, IdxMap: stationIdxMap}
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
