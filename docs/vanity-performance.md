# Vanity performance

Measured on 2026-09-07 with Go 1.26.6, Windows amd64, Intel Core i7-11700
(8 cores / 16 logical CPUs), NVIDIA GeForce GT 730 and Intel UHD Graphics 750.
Numbers below are medians of three runs. They describe this machine and its
installed drivers; they are not guarantees for other hardware or workloads.

## Before and after

| Benchmark | Before | After | Change |
| --- | ---: | ---: | ---: |
| CPU search, 10 workers, 10 million candidates/run | 67.29 M/s | 175.64 M/s | 2.61x |
| GT 730 dispatch, 262,144 candidates | 184.53 M/s | 194.65 M/s | +5.5% |
| UHD 750 dispatch, 262,144 candidates | 280.66 M/s | 285.17 M/s | +1.6% |

The CPU comparison includes scheduling, key generation and matching. The GPU
microbenchmarks use 64 templates and 4,096 timestamps per template; they exclude
driver compilation, random key generation, warm-up and cleanup. Both versions
use the same fixed work size, suffix scope, all digits, retention threshold 16
and initial best 8. The existing single-fingerprint reference path remains
around 72–74 ns/hash; mining now uses a separate batch path.

## Complete search throughput

These measurements include driver compilation, key generation, scheduling and
CPU verification. Each run searches exactly one billion candidates with a
30-day timestamp range, suffix scope, digits `018`, retention threshold 16,
automatic GPU batch timing and no artifact/database callback.

| Backend | CPU workers | Median candidates/second |
| --- | ---: | ---: |
| CPU | 8 | 153.04 M/s |
| Both GPUs | 0 | 540.28 M/s |
| Both GPUs + CPU | 4 | 610.15 M/s |
| Both GPUs + CPU | 8 | 636.10 M/s |
| Both GPUs + CPU | 12 | **676.74 M/s** |
| Both GPUs + CPU | 16 | 649.88 M/s |

CPU and integrated GPU share system resources. Increasing CPU worker count
does not always improve total throughput. The tested combination favors 12:

```powershell
go build -trimpath -o .\build\gpgenie.exe ./cmd/gpgenie
.\build\gpgenie.exe --config .\config.json vanity `
  --backend hybrid --workers 12 --gpu-devices all `
  --gpu-key-batch 0 --gpu-work-items 0
```

The command retains the configured threshold/digits/defaults and searches
until cancellation. To measure without a long mining session, add
`--min-run 16 --max-attempts 1000000000 --resume=false` and use a separate
output directory/checkpoint. Benchmark functions below avoid artifact and
database writes entirely.

## Implementation

- The CPU path pre-pads each public-key template once and batches 1,024
  timestamps. On amd64 with SHA and SSE4.1 instructions, two independent
  SHA-NI lanes overlap their instruction latency. They emit only the 64-bit
  key IDs needed for filtering. Other architectures, unsupported CPUs and
  `-tags=purego` use the standard library. There are no hot-path allocations.
- Every CPU batch hit is rehashed with `crypto/sha1` before delivery, as GPU
  hits already were. Random Ed25519 private material still comes from
  `crypto/rand`; matching does not change key entropy or the OpenPGP format.
- Suffix matching uses a word XOR and trailing-zero count. A cheap word filter
  rejects impossible suffix/any-position matches before calculating metadata.
  The shared best counter is consulted on potential improvements rather than
  on every hash. All retained hits are delivered even below an earlier best.
- OpenCL SHA-1 rounds are explicitly unrolled with scalar schedule registers.
  The long key ID is derived from the state after round 77: the last three
  rounds only change digest words that mining does not inspect. Full retained
  fingerprints are always computed through the reference implementation.
- GPU global winner atomics run only for improvements over the dispatch's
  immutable baseline. The host resets/reads the eight-byte result header and
  reads hit entries only when present. A blocking queue read already waits for
  execution, so an extra `clFinish` is unnecessary.
- Automatic dispatch sizing starts at up to 1 Mi candidates, grows by at most
  2x per dispatch, targets 100 ms and caps at 64 Mi candidates. Each device is
  measured independently. Low retention thresholds cap work to protect the
  hit buffer. Kernel-specific workgroup limits are respected. Partially filled
  final budgets do not launch entirely inactive template rows.
- Key preparation stops when the attempt budget is exhausted. A key batch
  larger than the work limit is reduced. Cancellation is checked during key
  generation and between CPU batches. Worker errors are drained before search
  completion. Each retained keyring receives its own packet header so two
  timestamps sharing Ed25519 material cannot overwrite each other's metadata.

## Reproduce and validate

```powershell
# Same before/after microbenchmarks; run without concurrent benchmark jobs.
go test ./internal/key/vanity -run '^$' `
  -bench 'Benchmark(SearchTenMillionCandidates|OpenCLDispatch)$' `
  -benchtime=1s -count=3

# Complete CPU / both-GPU / hybrid combinations, one billion candidates each.
go test ./internal/key/vanity -run '^$' `
  -bench BenchmarkSearchBackends -benchtime=1x -count=3

go test ./...
go vet ./...
go test -tags=purego ./internal/key/vanity ./cmd
go generate ./internal/key/vanity
```

The SIMD oracle tests cover every valid one-block template length, empty/odd
batch sizes, sentinels around output memory and timestamp carry/wrap boundaries.
Matcher tests compare random and constructed inputs to a hexadecimal-string
reference. Both detected GPUs are checked against the complete CPU hit set for
both scopes, sparse digit masks, thresholds, resumed best scores and partial
workgroups. Synthetic GPU inputs cover all digits, positions and run lengths
1–16. Search tests cover retention, cancellation, exact budgets and callback
errors; OpenPGP/GnuPG round trips remain part of the suite.

The local Windows environment has CGO disabled and no C compiler, so the race
detector cannot run here. The repository's Linux CI runs `go test -race`.
Cross-compilation checks cover Linux amd64 and Windows arm64; runtime GPU
validation above covers the two Windows devices listed at the top.
