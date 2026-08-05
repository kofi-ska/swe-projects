# High time to first token

1. Split the latency into queue wait, batch formation, and inference using metrics and Jaeger.
2. If queue wait dominates, inspect saturation and worker utilization.
3. If batch wait dominates at low traffic, reduce `MAX_BATCH_WAIT`.
4. If processing dominates, capture CPU, allocation, and block profiles and compare them with the last baseline.
