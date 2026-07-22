# Results

Store comparison summaries here after benchmark runs.

Do not treat a single duration as a result. A useful result names:

- hypothesis
- independent variable
- controlled fields
- command
- manifest path
- benchmark output path
- profile paths
- syscall trace path
- observed effect
- bottleneck
- boundary where the effect disappears

Required profiling coverage:

- CPU profile collected and interpreted.
- Heap profile collected and interpreted.
- CPU flamegraph generated.
- Go execution trace inspected for scheduler, syscall, and GC behavior.
- Syscall trace collected with `write()` count, total bytes, average write size, and syscall duration.
- Escape-analysis output reviewed when allocation counts change.
- Runtime diagnostics reviewed when GC or scheduler behavior may affect the result.

A result that lacks any of those items must say so in the result summary.
