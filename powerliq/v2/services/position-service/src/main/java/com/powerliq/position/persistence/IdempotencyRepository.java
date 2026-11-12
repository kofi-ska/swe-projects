package com.powerliq.position.persistence;

import java.time.Instant;
import java.util.Optional;
import java.util.UUID;

public interface IdempotencyRepository {
    boolean claim(String key, String requestHash, UUID positionId, Instant createdAt);

    Optional<IdempotencyRecord> find(String key);

    record IdempotencyRecord(String requestHash, UUID positionId) {
    }
}
