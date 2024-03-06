// SPDX-FileCopyrightText:  Copyright 2024 Roland Csaszar
// SPDX-License-Identifier: MIT
//
// Project:  1-billion-row-challenge
// File:     go_single_thread.go
// Date:     04.Mar.2024
//
// ==============================================================================

package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"sort"
)

type stationTemperature struct {
	TempSum int32
	Count   uint32
	Min     int16
	Max     int16
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

	stationData := make(map[string]stationTemperature, 10_000)
	idx := 0
	// We suppose the file is valid, without a single error.
	// Not a single error check is made.
	for idx < len(content) {
		semiColonIdx := bytes.IndexByte(content[idx:], ';')
		if semiColonIdx < 0 {
			break
		}
		station := content[idx : idx+semiColonIdx]
		newLineIdx := bytes.IndexByte(content[idx+semiColonIdx:], '\n')
		// End of file.
		if newLineIdx < 0 {
			newLineIdx = len(content) - (idx + semiColonIdx)
		}
		var temperature int16 = 0
		var negate int16 = 1
		for tmpIdx := idx + semiColonIdx + 1; tmpIdx < idx+semiColonIdx+newLineIdx; {
			currByte := content[tmpIdx]
			if currByte == '-' {
				negate = -1
			} else if currByte != '.' {
				intVal := currByte - '0'
				temperature = temperature*10 + int16(intVal)
			}
			tmpIdx++
		}
		temperature *= negate

		currData, ok := stationData[string(station)]
		if ok {
			currData.TempSum += int32(temperature)
			currData.Count++
			currData.Min = min(currData.Min, temperature)
			currData.Max = max(currData.Max, temperature)
			stationData[string(station)] = currData
		} else {
			stationData[string(station)] = stationTemperature{
				TempSum: int32(temperature),
				Count:   1,
				Min:     temperature,
				Max:     temperature,
			}
		}

		idx += semiColonIdx + newLineIdx + 1
	}

	keys := make([]string, 0, 10_000)
	for key := range stationData {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	fmt.Printf("{")
	for i, station := range keys {
		if i > 0 {
			fmt.Printf(", ")
		}
		meanF := float64(stationData[station].TempSum) / float64(stationData[station].Count)
		fmt.Printf("%s=%.1f/%0.1f/%.1f", station,
			roundJava(float64(stationData[station].Min)),
			roundJava(meanF),
			roundJava(float64(stationData[station].Max)))
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
