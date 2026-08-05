# Queue saturation

User impact is increased time to first token followed by explicit `503` responses.

1. Confirm queue utilization, rejection rate, active workers, and batch size in the capacity dashboard.
2. Inspect Jaeger traces for time in the queue versus model processing.
3. Check CPU, heap, and goroutine trends before changing capacity.
4. Reduce offered load or add gateway replicas. Increase queue capacity only when memory headroom and acceptable queue latency are demonstrated.
