package com.powerliq.events;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.UUID;

public record PositionCreatedV1(
        UUID positionId,
        UUID portfolioId,
        String side,
        BigDecimal contractPriceGbpPerMWh,
        BigDecimal volumeMWh,
        Instant deliveryStart,
        Instant deliveryEnd,
        Instant createdAt) implements PowerLiqEvent {
}
