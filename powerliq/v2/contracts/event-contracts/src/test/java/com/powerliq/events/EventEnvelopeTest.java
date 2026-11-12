package com.powerliq.events;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.UUID;
import org.junit.jupiter.api.Test;

class EventEnvelopeTest {
    @Test
    void preservesVersionedEventMetadata() {
        UUID eventId = UUID.randomUUID();
        UUID correlationId = UUID.randomUUID();
        UUID positionId = UUID.randomUUID();
        Instant now = Instant.parse("2026-11-12T20:00:00Z");
        PositionCreatedV1 payload = new PositionCreatedV1(positionId, UUID.randomUUID(), "LONG",
                new BigDecimal("80.00000"), new BigDecimal("100.000"), now, now.plusSeconds(1800), now);

        EventEnvelope<PositionCreatedV1> envelope = new EventEnvelope<>(eventId, "position.created", 1,
                now, "position-service", correlationId, null, positionId, positionId.toString(), payload);

        assertEquals(eventId, envelope.eventId());
        assertEquals(1, envelope.eventVersion());
        assertEquals(payload, envelope.payload());
    }

    @Test
    void rejectsInvalidEnvelopeMetadata() {
        UUID id = UUID.randomUUID();
        Instant now = Instant.parse("2026-11-12T20:00:00Z");
        PositionSettledV1 payload = new PositionSettledV1(id, now);

        assertThrows(IllegalArgumentException.class,
                () -> new EventEnvelope<>(UUID.randomUUID(), "position.settled", 0, now,
                        "position-service", UUID.randomUUID(), null, id, id.toString(), payload));
    }
}
