package com.powerliq.position.persistence;

import com.powerliq.events.EventEnvelope;
import com.powerliq.events.PowerLiqEvent;

public interface OutboxRepository {
    void append(EventEnvelope<? extends PowerLiqEvent> event);
}
