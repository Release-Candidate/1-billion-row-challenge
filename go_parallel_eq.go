// SPDX-FileCopyrightText:  Copyright 2024 Roland Csaszar
// SPDX-License-Identifier: MIT
//
// Project:  1-billion-row-challenge
// File:     go_parallel_eq.go
// Date:     23.Mar.2024
//
// =============================================================================

// Uses FNV hash algorithm: http://www.isthe.com/chongo/tech/comp/fnv/index.html

package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"syscall"
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

type mapStruct struct {
	Station string
	idx     int
}

const (
	numBits        = 16
	mask           = (1 << numBits) - 1
	fnvPrime       = 16777619
	fnvOffsetBasis = 2166136261
)

func fnvHash(s string) uint32 {
	var hash uint32 = fnvOffsetBasis
	for _, ch := range s {
		hash ^= uint32(ch)
		hash *= fnvPrime
	}
	return hash & mask
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Error: no data file to process given! Exiting.")
		os.Exit(1)
	}
	numCPUs := 10 * runtime.NumCPU()
	fileName := os.Args[1]

	// f, err := os.Create("cpu.prof")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// pprof.StartCPUProfile(f)
	// defer pprof.StopCPUProfile()

	// f1, err := os.Create("trace.prof")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// trace.Start(f1)
	// defer trace.Stop()

	file, err := os.Open(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file '%s':\n%s\n", fileName, err)
		os.Exit(2)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting data of file '%s':\n%s\n", fileName, err)
		os.Exit(3)
	}

	size := int64(stat.Size())
	chunkSize := size / int64(numCPUs)

	content, err := syscall.Mmap(int(file.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error mapping file '%s':\n%s\n", fileName, err)
		os.Exit(4)
	}
	defer func() {
		err := syscall.Munmap(content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error unmapping file '%s':\n%s\n", fileName, err)
			os.Exit(5)
		}
	}()

	chunkList := generateChunkIndices(numCPUs, size, chunkSize, err, file, fileName)

	channels := make([]chan resultType, numCPUs)

	for idx, chunk := range chunkList {
		// non-blocking channels
		channels[idx] = make(chan resultType, 1)
		go processChunk(content[chunk.StartIdx:chunk.EndIdx+1], channels[idx])
	}

	numSumChans := 2
	numToSum := numCPUs / numSumChans
	sumChannels := make([]chan resultType, numSumChans)

	for i := 0; i < numSumChans; i++ {
		sumChannels[i] = make(chan resultType, 1)

		go sumResults(channels[i*numToSum:(i+1)*numToSum], sumChannels[i])
	}

	stationSumData := stationTemperatures{
		TempSum: make([]int, 10_000),
		Count:   make([]uint, 10_000),
		Min:     make([]int, 10_000),
		Max:     make([]int, 10_000),
	}
	stationSumIdxMap := make([]mapStruct, mask+1)

	stationIdx := 0
	for _, channel := range sumChannels {
		result := <-channel
		stationData := result.Temps
		stationIdxMap := result.IdxMap

		for _, station := range stationIdxMap {
			if station.Station == "" {
				continue
			}
			idx := station.idx
			nameHash := fnvHash(station.Station)
			for i := nameHash; i < mask+1; i++ {
				if bytes.Equal([]byte(stationSumIdxMap[i].Station), []byte(station.Station)) {
					stIdx := stationSumIdxMap[i].idx
					stationSumData.TempSum[stIdx] += stationData.TempSum[idx]
					stationSumData.Count[stIdx] += stationData.Count[idx]
					stationSumData.Min[stIdx] = min(stationData.Min[idx], stationSumData.Min[stIdx])
					stationSumData.Max[stIdx] = max(stationData.Max[idx], stationSumData.Max[stIdx])
					break
				} else if stationSumIdxMap[i].Station == "" {
					stationSumIdxMap[i].idx = stationIdx
					stationSumIdxMap[i].Station = station.Station
					stationSumData.TempSum[stationIdx] = stationData.TempSum[idx]
					stationSumData.Count[stationIdx] = stationData.Count[idx]
					stationSumData.Min[stationIdx] = stationData.Min[idx]
					stationSumData.Max[stationIdx] = stationData.Max[idx]
					stationIdx++
					break
				}
			}
		}
	}

	keys := make([]string, 0, mask+1)
	for _, key := range stationSumIdxMap {
		if key.Station != "" {
			keys = append(keys, key.Station)
		}
	}
	sort.Strings(keys)

	fmt.Printf("{")
	for i, station := range keys {
		if i > 0 {
			fmt.Printf(", ")
		}
		idx := 0
		nameHash := fnvHash(station)
		for i := nameHash; i < mask+1; i++ {
			if stationSumIdxMap[i].Station == station {
				idx = stationSumIdxMap[i].idx
				break
			}
		}

		meanF := float64(stationSumData.TempSum[idx]) / float64(stationSumData.Count[idx])
		fmt.Printf("%s=%.1f/%.1f/%.1f", station,
			roundJava(float64(stationSumData.Min[idx])),
			roundJava(meanF),
			roundJava(float64(stationSumData.Max[idx])))
	}
	fmt.Printf("}\n")
}

func sumResults(channels []chan resultType, result chan resultType) {
	stationSumData := stationTemperatures{
		TempSum: make([]int, 10_000),
		Count:   make([]uint, 10_000),
		Min:     make([]int, 10_000),
		Max:     make([]int, 10_000),
	}

	stationSumIdxMap := make([]mapStruct, mask+1)

	stationIdx := 0
	for _, channel := range channels {
		result := <-channel
		stationData := result.Temps
		stationIdxMap := result.IdxMap

		for _, station := range stationIdxMap {
			if station.Station == "" {
				continue
			}
			nameHash := fnvHash(station.Station)
			idx := station.idx
			for i := nameHash; i < mask+1; i++ {
				if bytes.Equal([]byte(stationSumIdxMap[i].Station), []byte(station.Station)) {
					stIdx := stationSumIdxMap[i].idx
					stationSumData.TempSum[stIdx] += stationData.TempSum[idx]
					stationSumData.Count[stIdx] += stationData.Count[idx]
					stationSumData.Min[stIdx] = min(stationData.Min[idx], stationSumData.Min[stIdx])
					stationSumData.Max[stIdx] = max(stationData.Max[idx], stationSumData.Max[stIdx])
					break
				} else if stationSumIdxMap[i].Station == "" {
					stationSumIdxMap[i].idx = stationIdx
					stationSumIdxMap[i].Station = station.Station
					stationSumData.TempSum[stationIdx] = stationData.TempSum[idx]
					stationSumData.Count[stationIdx] = stationData.Count[idx]
					stationSumData.Min[stationIdx] = stationData.Min[idx]
					stationSumData.Max[stationIdx] = stationData.Max[idx]
					stationIdx++
					break
				}
			}
		}
	}

	result <- resultType{
		Temps:  stationSumData,
		IdxMap: stationSumIdxMap,
	}
}

func generateChunkIndices(numCPUs int, size int64, chunkSize int64, err error, file *os.File, fileName string) []chunk {
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
	return chunkList
}

type resultType struct {
	Temps  stationTemperatures
	IdxMap []mapStruct
}

func processChunk(content []byte, channel chan resultType) {
	stationData := stationTemperatures{
		TempSum: make([]int, 10_000),
		Count:   make([]uint, 10_000),
		Min:     make([]int, 10_000),
		Max:     make([]int, 10_000),
	}
	//colls := 0
	stationIdxMap := make([]mapStruct, mask+1)
	stationIdx := 0

	station := [100]byte{}
	// We suppose the file is valid, without a single error.
	// Not a single error check is made.
	for len(content) > 0 {

		// Station name is not empty.
		semiColonIdx := 1
		station[0] = content[0]
		currByte := content[1]
		var nameHash uint32 = fnvOffsetBasis
		for currByte != ';' {
			station[semiColonIdx] = currByte
			nameHash ^= uint32(currByte)
			nameHash *= fnvPrime
			semiColonIdx++
			currByte = content[semiColonIdx]
		}
		nameHash &= mask
		var temperature int = 0
		negate := 1
		if content[semiColonIdx+1] == '-' {
			negate = -1
			content = content[semiColonIdx+2:]
		} else {
			content = content[semiColonIdx+1:]
		}

		// Either `N.N\n` or `NN.N\n`
		if content[1] == '.' {
			temperature = negate * (int(content[0])*10 + int(content[2]) - 528)
			content = content[4:]
		} else {
			temperature = negate * (int(content[0])*100 + int(content[1])*10 + int(content[3]) - 5328)
			content = content[5:]
		}

		for i := nameHash; i < mask+1; i++ {
			if bytes.Equal(station[:semiColonIdx], []byte(stationIdxMap[i].Station)) {
				stIdx := stationIdxMap[i].idx
				stationData.TempSum[stIdx] += temperature
				stationData.Count[stIdx]++
				stationData.Min[stIdx] = min(stationData.Min[stIdx], temperature)
				stationData.Max[stIdx] = max(stationData.Max[stIdx], temperature)
				break
			} else if stationIdxMap[i].Station == "" {
				stationIdxMap[i].Station = string(station[:semiColonIdx])
				stationIdxMap[i].idx = stationIdx
				stationData.TempSum[stationIdx] = temperature
				stationData.Count[stationIdx] = 1
				stationData.Min[stationIdx] = temperature
				stationData.Max[stationIdx] = temperature
				stationIdx++
				break
			}
			// else {
			// 	colls++
			// }

		}

	}
	//fmt.Fprintf(os.Stderr, "Station name collisions: %d\n", colls)
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
