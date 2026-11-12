# High error rate

1. Break failures down by `outcome`; invalid requests and client cancellations are excluded from availability.
2. For `overloaded`, follow the queue-saturation runbook.
3. For `timeout`, inspect queue and processing spans against `REQUEST_TIMEOUT`.
4. For `internal_error`, inspect sampled error traces and recent configuration or image changes. Prompts and generated tokens must never be copied into incident notes.
