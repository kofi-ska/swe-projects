package com.powerliq.ingestion;

import java.time.Instant;
import java.util.UUID;

public record IngestionRun(UUID id, Type type, Status status, Instant windowStart, Instant windowEnd,
                           Instant startedAt, Instant finishedAt, long recordsRead, long eventsWritten,
                           String errorCode, String errorMessage) {
    public enum Type { SCHEDULED, BACKFILL }
    public enum Status { RUNNING, CANCEL_REQUESTED, SUCCEEDED, FAILED, CANCELLED }
}
