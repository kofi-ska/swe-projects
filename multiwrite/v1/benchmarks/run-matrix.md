# Run Matrix

Minimum comparisons before accepting a write-path claim:

- `direct` vs `buffered`, `fixed`, 32-byte records, same seed.
- `direct` vs `buffered`, `tiny`, 8-byte records, same seed.
- `buffered`, `fixed`, buffer sizes 4 KiB, 16 KiB, 64 KiB, 256 KiB.
- `buffered`, record sizes 8 B, 32 B, 128 B, 1 KiB, 4 KiB.
- Generator-only benchmark for every workload profile.
- Optional sync benchmark only when the environment and storage policy are recorded.

Minimum measurement set for each accepted comparison:

- Go benchmark output with `-benchmem`.
- CLI manifest for each workload run.
- CPU profile for each writer.
- Heap profile for each writer.
- CPU flamegraph for each writer.
- Go execution trace for each writer.
- Syscall summary for each writer.
- Escape-analysis note for hot-path allocation changes.
- Runtime diagnostic note for GC or scheduler effects.

Minimum questions answered:

- Did buffering reduce `write()` syscall count?
- Did buffering increase average syscall write size?
- Did system CPU per byte fall?
- Did allocation count or heap growth change?
- Did CPU move from writer/syscall overhead to generator, runtime, filesystem, or storage?
- Did trace show scheduler delay, syscall blocking, or GC as material?
- Did larger buffers plateau?
- Did larger records reduce or remove the buffering advantage?
- Did durability mode change the result?
- Was the result reproduced under the same config and resource envelope?

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

Expected artifact names:

- `artifacts/manifests/<case>.json`
- `artifacts/profiles/<case>.cpu.pprof`
- `artifacts/profiles/<case>.mem.pprof`
- `artifacts/profiles/<case>.cpu.svg`
- `artifacts/traces/<case>.trace`
- `artifacts/syscalls/<case>.txt`
- `results/<case>-summary.md`
