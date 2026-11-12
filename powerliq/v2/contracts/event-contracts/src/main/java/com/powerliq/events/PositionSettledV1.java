package com.powerliq.events;

import java.time.Instant;
import java.util.UUID;

public record PositionSettledV1(UUID positionId, Instant settledAt) implements PowerLiqEvent {
}
