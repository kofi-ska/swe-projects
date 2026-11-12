package com.powerliq.position.persistence;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.powerliq.events.EventEnvelope;
import com.powerliq.events.PowerLiqEvent;
import java.sql.Timestamp;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

@Repository
public class JdbcOutboxRepository implements OutboxRepository {
    private final JdbcTemplate jdbc;
    private final ObjectMapper objectMapper;

    public JdbcOutboxRepository(JdbcTemplate jdbc, ObjectMapper objectMapper) {
        this.jdbc = jdbc;
        this.objectMapper = objectMapper;
    }

    @Override
    public void append(EventEnvelope<? extends PowerLiqEvent> event) {
        try {
            jdbc.update("""
                    INSERT INTO outbox_events
                      (event_id, aggregate_id, event_type, event_version, partition_key,
                       correlation_id, causation_id, payload, occurred_at)
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?::jsonb, ?)
                    """, event.eventId(), event.aggregateId(), event.eventType(), event.eventVersion(),
                    event.partitionKey(), event.correlationId(), event.causationId(),
                    objectMapper.writeValueAsString(event), Timestamp.from(event.occurredAt()));
        } catch (JsonProcessingException exception) {
            throw new IllegalStateException("Unable to serialize outbox event", exception);
        }
    }
}
