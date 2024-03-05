# 1 Billion Row Challenge

This is my take on the one billion row challenge: [gunnarmorling/1brc at Github](https://github.com/gunnarmorling/1brc?tab=readme-ov-file#rules-and-limits)

- [Relevant Rules and Properties of the Data](#relevant-rules-and-properties-of-the-data)
  - [The Task](#the-task)
  - [Properties We Can Use to Our Advantage](#properties-we-can-use-to-our-advantage)
- [How to Run](#how-to-run)
- [Other Solutions](#other-solutions)
  - [Using Go](#using-go)
- [Benchmarks](#benchmarks)
  - [Comparison](#comparison)
    - [wc](#wc)
    - [Java Reference Implementation](#java-reference-implementation)
    - [Naive (G)AWK](#naive-gawk)
  - [Go Solution by Shraddha Agrawal](#go-solution-by-shraddha-agrawal)
  - [Go Solution by Ben Hoyt](#go-solution-by-ben-hoyt)
  - [Go Solution by Alexander Yastrebov](#go-solution-by-alexander-yastrebov)
- [Results](#results)
- [Files](#files)
  - [Data and Java Reference Implementation](#data-and-java-reference-implementation)
- [License](#license)

## Relevant Rules and Properties of the Data

The non-Java specific rules and properties from [1BRC - Rules and limits](https://github.com/gunnarmorling/1brc?tab=readme-ov-file#rules-and-limits):

- No external library dependencies may be used
- The computation must happen at application runtime, i.e. you cannot process the measurements file at build time (for instance, when using GraalVM) and just bake the result into the binary
- Input value ranges are as follows:
  - Station name: non null UTF-8 string of min length 1 character and max length 100 bytes, containing neither `;` nor `\n` characters. (i.e. this could be 100 one-byte characters, or 50 two-byte characters, etc.)
  - Temperature value: non null double between -99.9 (inclusive) and 99.9 (inclusive), always with one fractional digit
- There is a maximum of 10,000 unique station names
- Line endings in the file are `\n` characters on all platforms
- Implementations must not rely on specifics of a given data set, e.g. any valid station name as per the constraints above and any data distribution (number of measurements per station) must be supported
- The rounding of output values must be done using the semantics of IEEE 754 rounding-direction "roundTowardPositive"

Interestingly these rules do not tell us what we have to do with all these values?! 😃

### The Task

Copied from [1BRC - Github](https://github.com/gunnarmorling/1brc)

> The task is to write a [Java] program which reads the file, calculates the min, mean, and max temperature value per weather station, and emits the results on stdout like this (i.e. sorted alphabetically by station name, and the result values per station in the format min/mean/max, rounded to one fractional digit)

Example solution:

```text
{Abha=-23.0/18.0/59.2, Abidjan=-16.2/26.0/67.3, Abéché=-10.0/29.4/69.0, ... }
```

Btw. `mean` here is the sum of all values divided by the number of values (the "arithmetic mean" of the values).

### Properties We Can Use to Our Advantage

- While 1 billion sounds like much, we can keep everything (the whole data file and the intermediate data) in 32GB RAM. So no need to think about memory. Which means that if you have less RAM, you should scale the number of rows accordingly (`500_000_000`, 500 millions if you've got 16GB, `250_000_000` if you've got 8GB, ...).
- A maximum of 10,000 unique station names: so we can set all capacities to 10,000 and won't need to reallocate when dynamically growing. This implies, that there is a maximum number of 100,000 (1,000,000,000 / 10,000) temperatures per station.
- All float values are in the inclusive interval `[-99.9, 99.9]` and have exactly (always!) one decimal digit. So if we parse these as integers, we get values in the interval `[-990, 990]`. Having a maximum of 100,000 temperature values implies that the sums of these integer values are (always) in `[-99 000 000, 99 000 000]`. This is a sub-interval of `[-2^27, 2^27] == [-134 217 728, 134 217 728]`, so 32 bit integers suffice to hold the sum of the temperature values.
- The max length of 100 bytes of the names => we can use a fixed 100 byte buffer to parse them into.

## How to Run

1. Generate the data file [./measurements.txt](./measurements.txt) by running the Python script [./create_measurements.py](./create_measurements.py) on the file [./weather_stations.csv](./weather_stations.csv):

   ```shell
   python3 ./create_measurements.py 1_000_000_000
   ```

   **Warning**: This script takes a long time to run and generates 15GB of data!
2. Generate the "official" output file for your data file by running the 1BRC's baseline Java implementation (you need Java 21 installed on your machine):

   ```shell
   java CalculateAverage_baseline.java > correct_results.txt
   ```

3. Compile and benchmark the single threaded Go version using [hyperfine](https://github.com/sharkdp/hyperfine):

    ```shell
    go build ./go_single_thread.go
    hyperfine -r 5 -w 1 './go_single_thread measurements.txt > solution.txt'
    ```

4. Compare the generated output file with the "official" output file:

   ```shell
   diff correct_results.txt ./solution.txt
   ```

## Other Solutions

Official Java implementations: [1BRC - Results](https://github.com/gunnarmorling/1brc?tab=readme-ov-file#results)

Other implementations mentioned on the 1BRC site: [1BRC on the Web](https://github.com/gunnarmorling/1brc?tab=readme-ov-file#1brc-on-the-web)

### Using Go

- [One Billion Rows Challenge in Golang - 19th February 2024](https://www.bytesizego.com/blog/one-billion-row-challenge-go)
- [The One Billion Row Challenge in Go: from 1m45s to 4s in nine solutions - March 2024](https://benhoyt.com/writings/go-1brc/)

## Benchmarks

All of this is running on my Apple M1 Max (Studio) with 32GB of RAM.

Detailed specs:

- 10-core CPU with 8 performance cores and 2 efficiency cores
- 24-core GPU
- 16-core Neural Engine
- 400GB/s memory bandwidth

Benchmark of [./go_single_thread.go](./go_single_thread.go): 98s, 1.6 minutes, 1 minute 38 seconds.

```shell
hyperfine -r 5 -w 1 './go_single_thread measurements_big.txt > solution_big.txt'
Benchmark 1: ./go_single_thread measurements_big.txt > solution_big.txt
  Time (mean ± σ):     98.484 s ±  1.296 s    [User: 90.378 s, System: 3.329 s]
  Range (min … max):   96.970 s … 100.237 s    5 runs
```

### Comparison

#### wc

First the time it takes `wc` to just count the lines of the data file to check if it really are 1 billion rows:

```shell
% time wc -l measurements.txt
 1000000000 measurements.txt
wc -l measurements.txt  14.95s user 1.20s system 95% cpu 16.857 total
```

So there really are 1 billion rows in this file! It took about 17s just to count the lines (`\n` characters).

#### Java Reference Implementation

The official Java solution needs about 220s, 3.7 minutes (no, I did not care to run it more than once, like using `hyperfine`):

```shell
export PATH="/opt/homebrew/opt/openjdk/bin:$PATH"; time java CalculateAverage_baseline.java > correct_results.txt
java CalculateAverage_baseline.java > correct_results.txt  215.46s user 4.90s system 100% cpu 3:39.22 total
```

#### Naive (G)AWK

The naive AWK script [./awk.awk](./awk.awk) takes about 600s, 10 minutes (no, I did not care to run it more than once, like using `hyperfine`):

```awk
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

```

This GAWK script does **not** round the average values like the official Java solution, but everything else is the same.

```shell
time gawk -f awk.awk measurements.txt > solution.txt
gawk -f awk.awk measurements_big.txt > solution_big.txt  595.04s user 3.70s system 99% cpu 10:00.36 total
```

### Go Solution by Shraddha Agrawal

The Go solution from the blog post [One Billion Rows Challenge in Golang](https://www.bytesizego.com/blog/one-billion-row-challenge-go). I had to add code to get output on Stdout, this program did not print the results. This takes about 12s.

```shell
hyperfine -r 5 -w 1 './go_Shraddha_Agrawal -input measurements.txt > solution.txt'
Benchmark 1: ./go_Shraddha_Agrawal -input measurements.txt > solution.txt
  Time (mean ± σ):     12.141 s ±  0.318 s    [User: 98.765 s, System: 1.857 s]
  Range (min … max):   11.736 s … 12.539 s    5 runs
```

### Go Solution by Ben Hoyt

This is the fasted version (R9) from the Blog post [The One Billion Row Challenge in Go: from 1m45s to 4s in nine solutions](https://benhoyt.com/writings/go-1brc/). **This version does not produce the correct mean values, it generates `-0.0` instead of `0.0` as the average for some stations**.

I needed to change the source to be able to have a comparable version for the benchmark. It did use 12 Threads, but took 75s (1:15 minutes), so maybe I did make some error when adding the main function to call it.

```shell
hyperfine -r 5 -w 1 './go_Ben_Hoyt measurements.txt > solution.txt'
Benchmark 1: ./go_Ben_Hoyt measurements.txt > solution.txt
  Time (mean ± σ):     74.583 s ±  1.005 s    [User: 696.073 s, System: 2.520 s]
  Range (min … max):   73.364 s … 76.123 s    5 runs
```

### Go Solution by Alexander Yastrebov

This program needs about 4s. The source can be found [1BRC - Go](https://github.com/gunnarmorling/1brc/blob/main/src/main/go/AlexanderYastrebov/calc.go).

```shell
hyperfine -r 5 -w 1 './go_Alexander_Yastrebov measurements.txt > solution.txt'
Benchmark 1: ./go_Alexander_Yastrebov measurements.txt > solution.txt
  Time (mean ± σ):      4.027 s ±  0.020 s    [User: 35.769 s, System: 1.355 s]
  Range (min … max):    4.003 s …  4.049 s    5 runs
```

## Results

For details see [Benchmarks](#benchmarks)

- `wc -l`: the time `wc -l measurements.txt` takes. Just the number of lines.
- [./awk.awk](./awk.awk): the time `gawk -f awk.awk measurements.txt > solution.txt` takes. A naive, single threaded GAWK implementation, does not produce correctly rounded output.
- [Java Reference Implementation](./CalculateAverage_baseline.java): the time `java CalculateAverage_baseline.java > correct_results.txt` takes. 1BRC's reference implementation (naive implementation).
- `Go Alexander Yastrebov`: fastest Go version of [1BRC - Go](https://github.com/gunnarmorling/1brc/blob/main/src/main/go/AlexanderYastrebov/calc.go)
- `Go Shraddha Agrawal`: fasted Version of [One Billion Rows Challenge in Golang](https://www.bytesizego.com/blog/one-billion-row-challenge-go)
- `Go Ben Hoyt`: does not produce the correct output, fastest version of [The One Billion Row Challenge in Go: from 1m45s to 4s in nine solutions](https://benhoyt.com/writings/go-1brc/).
- [./go_single_thread.go](./go_single_thread.go): my baseline Go version, single threaded and using a map of structures.

| Program                       | Time |
| ----------------------------- | ---- |
| wc -l                         | 17s  |
| awk.awk                       | 600s |
| Java Reference Implementation | 220s |
| Go Alexander Yastrebov        | 4s   |
| Go Shraddha Agrawal           | 12s  |
| Go Ben Hoyt*                  | 75s  |
| go_single_thread.go           | 98s  |

## Files

This is a description of the files in this repository and the generated files, which are not checked in to Git.

- [./awk.awk](./awk.awk): straightforward GNU AWK implementation, to get a baseline time. Needs GNU awk, `gawk` to run, POSIX `awk` is not supported because of the sorting function it uses.
- [./go_single_thread.go](./go_single_thread.go): the baseline go version, single threaded and using a map of structures.

### Data and Java Reference Implementation

- [measurements.txt](./measurements.txt): the data file generated by the Python script [./create_measurements.py](./create_measurements.py). Not checked in to Git. To generate, see the [How to Run](#how-to-run) section above.
- [./correct_results.txt](./correct_results.txt): the "official" output file generated by the baseline Java implementation [./CalculateAverage_baseline.java](./CalculateAverage_baseline.java), the reference solution to compare all others against. Not checked in to Git. To generate, see the [How to Run](#how-to-run) section above.
- [./weather_stations.csv](./weather_stations.csv): the CSV file used as input for the Python script [./create_measurements.py](./create_measurements.py).
- [./create_measurements.py](./create_measurements.py): the Python script used to generate the data file [./measurements.txt](./measurements.txt).
- [./CalculateAverage_baseline.java](./CalculateAverage_baseline.java): the baseline Java implementation used to generate the "official" output file [./correct_results.txt](./correct_results.txt).

## License

The code in this repository is licensed under the MIT license, see file [./LICENSE](./LICENSE).
Except the Python script [./create_measurements.py](./create_measurements.py) to generate the data, and the base Java implementation [./CalculateAverage_baseline.java](./CalculateAverage_baseline.java) which are licensed under the Apache 2.0 license.
