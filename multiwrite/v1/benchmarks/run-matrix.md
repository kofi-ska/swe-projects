# Run Matrix

Minimum comparisons before accepting a write-path claim:

- `direct` vs `buffered`, `fixed`, 32-byte records, same seed.
- `direct` vs `buffered`, `tiny`, 8-byte records, same seed.
- `buffered`, `fixed`, buffer sizes 4 KiB, 16 KiB, 64 KiB, 256 KiB.
- `buffered`, record sizes 8 B, 32 B, 128 B, 1 KiB, 4 KiB.
- Generator-only benchmark for every workload profile.
- Optional sync benchmark only when the environment and storage policy are recorded.

Controlled fields:

- binary version
- Go version
- OS and architecture
- writer strategy except when it is the independent variable
- workload profile except when it is the independent variable
- seed
- record count
- byte limit
- record size
- durability mode
- CPU and memory envelope
- output, manifest, profile, trace, and syscall artifact paths
