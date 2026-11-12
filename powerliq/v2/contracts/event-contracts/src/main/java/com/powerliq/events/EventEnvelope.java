package com.powerliq.events;

import java.time.Instant;
import java.util.Objects;
import java.util.UUID;

public record EventEnvelope<T extends PowerLiqEvent>(
        UUID eventId,
        String eventType,
        int eventVersion,
        Instant occurredAt,
        String source,
        UUID correlationId,
        UUID causationId,
        UUID aggregateId,
        String partitionKey,
        T payload) {

    public EventEnvelope {
        Objects.requireNonNull(eventId, "eventId");
        requireText(eventType, "eventType");
        if (eventVersion < 1) throw new IllegalArgumentException("eventVersion must be positive");
        Objects.requireNonNull(occurredAt, "occurredAt");
        requireText(source, "source");
        Objects.requireNonNull(correlationId, "correlationId");
        Objects.requireNonNull(aggregateId, "aggregateId");
        requireText(partitionKey, "partitionKey");
        Objects.requireNonNull(payload, "payload");
    }

    private static void requireText(String value, String field) {
        if (value == null || value.isBlank()) throw new IllegalArgumentException(field + " must not be blank");
    }
}
