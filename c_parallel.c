// SPDX-FileCopyrightText:  Copyright 2024 Roland Csaszar
// SPDX-License-Identifier: MIT
//
// Project:  1-billion-row-challenge
// File:     c_parallel.c
// Date:     23.Mar.2024
//
// =============================================================================

#include <errno.h>
#include <math.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// Non-portable, Unix* specific.
#include <pthread.h>
#include <sys/mman.h>

struct StationTemperatures_t {
  int64_t* temp_sum;
  uint64_t* count;
  int64_t* min;
  int64_t* max;
};
typedef struct StationTemperatures_t StationTemperatures;

struct Chunk_t {
  size_t start_idx;
  size_t end_idx;
};
typedef struct Chunk_t Chunk;

struct MapStruct_t {
  char name[100];
  size_t idx;
};
typedef struct MapStruct_t MapStruct;

struct ThreadData_t {
  char const* data;
  size_t data_size;
};
typedef struct ThreadData_t ThreadData;

struct Result_t {
  StationTemperatures temp;
  MapStruct* idx_map;
};
typedef struct Result_t Result;

#define NUM_BITS ((uint32_t)16)
#define MASK ((uint32_t)((1 << NUM_BITS) - 1))
#define FNV_PRIME ((uint32_t)16777619)
#define FNV_OFFSET_BASIS ((uint32_t)2166136261)

uint32_t fnv_hash(char* s) {
  uint32_t hash = FNV_OFFSET_BASIS;

  while (*s) {
    hash ^= (uint32_t)*s;
    hash *= FNV_PRIME;
    s++;
  }

  return hash & MASK;
}

Chunk const* generate_chunk_indices(size_t num_threads,
                                    size_t data_size,
                                    char const* data,
                                    size_t chunk_size) {
  Chunk* chunk_list = calloc(num_threads, sizeof *chunk_list);
  chunk_list[0].start_idx = 0;
  chunk_list[0].end_idx = data_size - 1;

  size_t read_off = chunk_size;
  for (size_t cpu_idx = 1; cpu_idx < num_threads; cpu_idx++) {
    char const* newline_ptr = memchr(&data[read_off], '\n', 150);
    if (newline_ptr == 0) {
      chunk_list[cpu_idx].end_idx = data_size - 1;
      break;
    }
    size_t newline_idx = newline_ptr - data;
    chunk_list[cpu_idx - 1].end_idx = newline_idx;
    chunk_list[cpu_idx].start_idx = newline_idx + 1;
    chunk_list[cpu_idx].end_idx = data_size - 1;
    read_off += chunk_size;
  }
  return chunk_list;
}

void* process_chunk(void* thread_data) {
  char const* data = ((ThreadData*)thread_data)->data;
  size_t data_size = ((ThreadData*)thread_data)->data_size;
  Result* result = calloc(1, sizeof *result);
  result->temp.count = calloc(10000, sizeof *result->temp.count);
  result->temp.temp_sum = calloc(10000, sizeof *result->temp.temp_sum);
  result->temp.min = calloc(10000, sizeof *result->temp.min);
  result->temp.max = calloc(10000, sizeof *result->temp.max);
  result->idx_map = calloc(MASK + 1, sizeof *result->idx_map);

  size_t station_idx = 0;
  char station_name[100];
  char const* content = data;
  char const* data_end = data + data_size;
  while (data_end - content > 0) {
    size_t semicolon_idx = 1;
    station_name[0] = content[0];
    char curr_byte = content[1];
    uint32_t name_hash = FNV_OFFSET_BASIS;
    while (curr_byte != ';') {
      station_name[semicolon_idx] = curr_byte;
      name_hash ^= (uint32_t)curr_byte;
      name_hash *= FNV_PRIME;
      semicolon_idx++;
      curr_byte = content[semicolon_idx];
    }
    station_name[semicolon_idx] = '\0';
    name_hash &= MASK;
    int64_t temperature = 0;
    int64_t negate = 1;
    if (content[semicolon_idx + 1] == '-') {
      negate = -1;
      content += semicolon_idx + 2;
    } else {
      content += semicolon_idx + 1;
    }
    if (content[1] == '.') {
      temperature =
          negate * ((int64_t)content[0] * 10 + (int64_t)content[2] - 528);
      content += 4;
    } else {
      temperature =
          negate * ((int64_t)content[0] * 100 + (int64_t)content[1] * 10 +
                    (int64_t)content[3] - 5328);
      content += 5;
    }

    for (size_t i = name_hash; i < MASK + 1; i++) {
      if (result->idx_map[i].name[0] == 0) {
        memcpy(result->idx_map[i].name, station_name, semicolon_idx + 1);
        result->idx_map[i].idx = station_idx;
        result->temp.temp_sum[station_idx] = temperature;
        result->temp.count[station_idx] = 1;
        result->temp.min[station_idx] = temperature;
        result->temp.max[station_idx] = temperature;
        station_idx++;
        break;
      }
      if (strcmp(result->idx_map[i].name, station_name) == 0) {
        size_t idx = result->idx_map[i].idx;
        result->temp.temp_sum[idx] += temperature;
        result->temp.count[idx]++;
        result->temp.min[idx] = result->temp.min[idx] > temperature
                                    ? temperature
                                    : result->temp.min[idx];
        result->temp.max[idx] = result->temp.max[idx] < temperature
                                    ? temperature
                                    : result->temp.max[idx];
        break;
      }
    }  // for i

  }  // while content < data_end
  return result;
}

void sum_results(size_t num_threads,
                 pthread_t* thread,
                 MapStruct* sum_idx_map,
                 StationTemperatures* sum_data) {
  size_t station_idx = 0;
  for (size_t thread_id = 0; thread_id < num_threads; thread_id++) {
    Result const* result = 0;
    int ret_val = pthread_join(thread[thread_id], (void**)&result);
    if (ret_val != 0) {
      fprintf(stderr, "Error joining thread: %s\n", strerror(ret_val));
      exit(EXIT_FAILURE);
    }
    for (size_t res_idx = 0; res_idx < MASK + 1; res_idx++) {
      MapStruct station = result->idx_map[res_idx];
      if (station.name[0] == 0) {
        continue;
      }
      size_t idx = station.idx;
      uint32_t name_hash = fnv_hash(station.name);
      for (size_t i = name_hash; i < MASK + 1; i++) {
        if (sum_idx_map[i].name[0] == 0) {
          sum_idx_map[i].idx = station_idx;
          memcpy(sum_idx_map[i].name, station.name, 100);
          sum_data->temp_sum[station_idx] = result->temp.temp_sum[idx];
          sum_data->count[station_idx] = result->temp.count[idx];
          sum_data->min[station_idx] = result->temp.min[idx];
          sum_data->max[station_idx] = result->temp.max[idx];
          station_idx++;
          break;
        }
        if (strcmp(sum_idx_map[i].name, station.name) == 0) {
          size_t st_idx = sum_idx_map[i].idx;
          sum_data->temp_sum[st_idx] += result->temp.temp_sum[idx];
          sum_data->count[st_idx] += result->temp.count[idx];
          sum_data->min[st_idx] = sum_data->min[st_idx] > result->temp.min[idx]
                                      ? result->temp.min[idx]
                                      : sum_data->min[st_idx];
          sum_data->max[st_idx] = sum_data->max[st_idx] < result->temp.max[idx]
                                      ? result->temp.max[idx]
                                      : sum_data->max[st_idx];
          break;
        }
      }
    }
    free((void*)result->temp.temp_sum);
    free((void*)result->temp.count);
    free((void*)result->temp.min);
    free((void*)result->temp.max);
    free((void*)result->idx_map);
    free((void*)result);
  }
}

int compare(void const* a, void const* b) {
  char const* str_a = ((MapStruct const*)a)->name;
  char const* str_b = ((MapStruct const*)b)->name;
  if (str_a == 0 && str_b == 0) {
    return 0;
  }
  if (str_a == 0) {
    return -1;
  }
  if (str_b == 0) {
    return 1;
  }
  return strcmp(str_a, str_b);
}

double round_java(double x) {
  double rounded = trunc(x);
  if (x < 0.0 && rounded - x == 0.5) {
    // return
  } else if (fabs(x - rounded) >= 0.5) {
    rounded += (x > 0) - (x < 0);
  }

  // oh, another hardcoded `-0.0` to `0.0` conversion.
  if (rounded == 0) {
    return 0.0;
  }

  return rounded / 10.0;
}

void print_results(MapStruct* sum_idx_map, StationTemperatures const sum_data) {
  qsort(sum_idx_map, (MASK + 1), sizeof *sum_idx_map, compare);
  printf("{");
  bool first = true;
  for (size_t idx = 0; idx < MASK + 1; idx++) {
    if (sum_idx_map[idx].name[0] != 0) {
      if (first) {
        first = false;
      } else {
        printf(", ");
      }
      size_t station_idx = sum_idx_map[idx].idx;
      double mean = (double)(sum_data.temp_sum[station_idx]) /
                    (double)(sum_data.count[station_idx]);
      printf("%s=%.1f/%.1f/%.1f", sum_idx_map[idx].name,
             round_java((double)sum_data.min[station_idx]), round_java(mean),
             round_java((double)sum_data.max[station_idx]));
    }
  }
  printf("}\n");
}

int main(int argc, char* argv[]) {
  if (argc != 2) {
    fprintf(stderr, "Error: no data file to process given! Exiting.\n");
    exit(EXIT_FAILURE);
  }
  char* file_name = argv[1];
  FILE* file = fopen(file_name, "r");
  if (file == NULL) {
    fprintf(stderr, "Error opening file '%s':\n%s\n", file_name,
            strerror(errno));
    exit(EXIT_FAILURE);
  }
  fseek(file, 0, SEEK_END);
  long data_size = ftell(file);
  rewind(file);

  size_t num_threads = 10;
  size_t chunk_size = data_size / num_threads;

  char* data = mmap(NULL, data_size, PROT_READ, MAP_SHARED, fileno(file), 0);
  if (data == MAP_FAILED) {
    fprintf(stderr, "Error mapping file '%s':\n%s\n", file_name,
            strerror(errno));
    exit(EXIT_FAILURE);
  }

  Chunk const* chunk_list =
      generate_chunk_indices(num_threads, data_size, data, chunk_size);

  pthread_t thread[num_threads];
  ThreadData thread_data[num_threads];

  for (size_t thread_id = 0; thread_id < num_threads; thread_id++) {
    thread_data[thread_id].data = &data[chunk_list[thread_id].start_idx];
    thread_data[thread_id].data_size =
        chunk_list[thread_id].end_idx - chunk_list[thread_id].start_idx + 1;
    int ret_val = pthread_create(&thread[thread_id], NULL, process_chunk,
                                 (void*)&thread_data[thread_id]);
    if (ret_val != 0) {
      fprintf(stderr, "Error creating thread: %s\n", strerror(ret_val));
      exit(EXIT_FAILURE);
    }
  }

  StationTemperatures sum_data;
  sum_data.temp_sum = calloc(10000, sizeof *sum_data.temp_sum);
  sum_data.count = calloc(10000, sizeof *sum_data.count);
  sum_data.min = calloc(10000, sizeof *sum_data.min);
  sum_data.max = calloc(10000, sizeof *sum_data.max);
  MapStruct* sum_idx_map = calloc((MASK + 1), sizeof *sum_idx_map);

  free((void*)chunk_list);

  sum_results(num_threads, thread, sum_idx_map, &sum_data);

  print_results(sum_idx_map, sum_data);

  free((void*)sum_data.temp_sum);
  free((void*)sum_data.count);
  free((void*)sum_data.min);
  free((void*)sum_data.max);
  free((void*)sum_idx_map);

  if (munmap(data, data_size) != 0) {
    fprintf(stderr, "Error unmapping file '%s':\n%s\n", file_name,
            strerror(errno));
    exit(EXIT_FAILURE);
  }

  fclose(file);

  exit(EXIT_SUCCESS);
}
