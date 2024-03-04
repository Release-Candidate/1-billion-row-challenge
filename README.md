# 1 Billion Row Challenge

This is my take on the one billion row challenge: [gunnarmorling/1brc at Gibthub](https://github.com/gunnarmorling/1brc?tab=readme-ov-file#rules-and-limits)

- [Relevant Rules and Properties of the Data](#relevant-rules-and-properties-of-the-data)
  - [The Task](#the-task)
  - [Properties We Can Use to Our Advantage](#properties-we-can-use-to-our-advantage)
- [How to Run](#how-to-run)
- [Other Solutions](#other-solutions)
  - [Using Go](#using-go)
- [Benchmarks](#benchmarks)
- [License](#license)

## Relevant Rules and Properties of the Data

The non-Java specific rules and properties from [1brc - Rules and limits](https://github.com/gunnarmorling/1brc?tab=readme-ov-file#rules-and-limits):

- No external library dependencies may be used
- The computation must happen at application runtime, i.e. you cannot process the measurements file at build time (for instance, when using GraalVM) and just bake the result into the binary
- Input value ranges are as follows:
  - Station name: non null UTF-8 string of min length 1 character and max length 100 bytes, containing neither ; nor \n characters. (i.e. this could be 100 one-byte characters, or 50 two-byte characters, etc.)
  - Temperature value: non null double between -99.9 (inclusive) and 99.9 (inclusive), always with one fractional digit
- There is a maximum of 10,000 unique station names
- Line endings in the file are \n characters on all platforms
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
- A maximum of 10,000 unique station names: so we can set all capacities to 10,000 and won't need to reallocate when dynamically growing.
- All float values are in the inclusive interval `[-99.9, 99.9]` and have exactly (always!) one decimal digit. So if we parse these as integers, we get values in the interval `[-990, 990]`. Having a maximum of 10,000 stations implies, that the sums of these integer values are (always) in `[-9,900,000, 9,900,000]`. This is a sub-interval of `[-2^24, 2^24] == [-16,777,216, 16,777,216]`, so 32 bit integers suffice to hold all needed values. A (well, the usual ones) 32 bit float has a mantissa that can hold exactly 23 bits + 1 (in normalized form), so we don't loose precision when converting the integer values to a 32 bit float.

## How to Run

1. Generate the data file [./measurements.txt](./measurements.txt) by running the Python script [./create_measurements.py](./create_measurements.py) on the file [./weather_stations.csv](./weather_stations.csv):

   ```shell
   python3 ./create_measurements.py 1_000_000_000
   ```

   **Warning**: This script takes a long time to run and generates 15GB of data!

## Other Solutions

Official Java implementations: [1BRC - Results](https://github.com/gunnarmorling/1brc?tab=readme-ov-file#results)

Other implementations mentioned on the 1BRC site: [1BRC on the Web](https://github.com/gunnarmorling/1brc?tab=readme-ov-file#1brc-on-the-web)

### Using Go

- [One Billion Rows Challenge in Golang - 19th February 2024](https://www.bytesizego.com/blog/one-billion-row-challenge-go)
- [The One Billion Row Challenge in Go: from 1m45s to 4s in nine solutions - March 2024](https://benhoyt.com/writings/go-1brc/)

## Benchmarks

## License

The code in this repository is licensed under the MIT license, see file [./LICENSE](./LICENSE).
Except the Python script [./create_measurements.py](./create_measurements.py) to generate the data, which is licensed under the Apache 2.0 license.
