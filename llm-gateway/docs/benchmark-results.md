# Benchmark baseline

Environment: Apple M4 Pro, Darwin arm64, Go 1.25.4. Results are a local baseline and should be regenerated on the target k3s node.

Run:

```bash
go test -bench=. -benchmem ./internal/...
```

The response serialization baseline measured 150 ns/op, 112 B/op, and 2 allocs/op. The instrumented batcher measured 836 ns/op, 1,040 B/op, and 15 allocs/op. Token generation measured 5.32 µs/op, 880 B/op, and 12 allocs/op.

`BenchmarkTokenAggregation` produced:

| Implementation | Time | Bytes | Allocations |
| --- | ---: | ---: | ---: |
| Repeated concatenation | 188,730 ns/op | 1,105,899 B/op | 499 allocs/op |
| Preallocated builder | 2,933 ns/op | 4,864 B/op | 1 alloc/op |

On this machine the builder was about 64× faster and removed 498 allocations per operation. Absolute values vary across machines; the ratio and the repeatable method are the useful evidence. Capture CPU and allocation profiles under the same load before changing the implementation, then rerun the identical benchmark and k6 scenario.
