package com.powerliq.application;

import java.time.Instant;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

final class InMemoryIdempotencyRepository implements IdempotencyRepository {
    private final ConcurrentHashMap<String, IdempotencyRecord> records = new ConcurrentHashMap<>();

    @Override
    public boolean claim(String key, String requestHash, UUID positionId, Instant createdAt) {
        return records.putIfAbsent(key, new IdempotencyRecord(requestHash, positionId)) == null;
    }

    @Override
    public Optional<IdempotencyRecord> find(String key) {
        return Optional.ofNullable(records.get(key));
    }
}
