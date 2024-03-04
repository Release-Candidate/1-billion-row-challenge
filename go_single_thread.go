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
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Error: no data file to process given! Exiting.")
		os.Exit(1)
	}
	fileName := os.Args[1]
	fmt.Println(fileName)
}
