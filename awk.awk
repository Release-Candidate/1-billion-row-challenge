# SPDX-FileCopyrightText:  Copyright 2024 Roland Csaszar
# SPDX-License-Identifier: MIT
#
# Project:  1-billion-row-challenge
# File:     awk.awk
# Date:     04.Mar.2024
#
# ==============================================================================
# An GNU awk script to solve the 1 billion row challenge.

# $1, the first field, is the location's name.
# $2, the second field, is the temperature in Celsius.
# The field separator `FS` is `;`.

BEGIN {
    FS = ";"
}

{
    if (!count[$1]) {
        count[$1] = 1
        min[$1] = $2
        max[$1] = $2
        sum[$1] = $2
    } else {
        count[$1]++
        if ($2 < min[$1]) {
            min[$1] = $2
        } else if ($2 > max[$1]) {
            max[$1] = $2
        }
        sum[$1] += $2
    }
}

END {
    printf "{"
    num = asorti(count, stations_sorted)
    for (i = 1; i <= num; i++) {
        station = stations_sorted[i]
        printf "%s=%.1f/%.1f/%.1f", station, min[station], sum[station] / count[station], max[station]
        if (i < num) {
            printf ", "
        }
    }
    printf "}\n"
}
