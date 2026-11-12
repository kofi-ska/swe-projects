package com.powerliq.application;

import java.time.Instant;
import java.sql.Timestamp;
import java.util.Optional;
import java.util.UUID;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

@Repository
public class JdbcIdempotencyRepository implements IdempotencyRepository {
    private final JdbcTemplate jdbc;

    public JdbcIdempotencyRepository(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    @Override
    public boolean claim(String key, String requestHash, UUID positionId, Instant createdAt) {
        return jdbc.update("""
                INSERT INTO idempotency_records (idempotency_key, request_hash, position_id, created_at)
                VALUES (?, ?, ?, ?)
                ON CONFLICT (idempotency_key) DO NOTHING
                """, key, requestHash, positionId, Timestamp.from(createdAt)) == 1;
    }

    @Override
    public Optional<IdempotencyRecord> find(String key) {
        return jdbc.query("""
                        SELECT request_hash, position_id FROM idempotency_records WHERE idempotency_key = ?
                        """, (result, row) -> new IdempotencyRecord(
                        result.getString("request_hash"), result.getObject("position_id", UUID.class)), key)
                .stream().findFirst();
    }
}
