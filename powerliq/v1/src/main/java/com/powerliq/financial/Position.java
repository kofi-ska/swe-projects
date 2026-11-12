package com.powerliq.financial;

import java.time.Instant;
import java.util.Objects;
import java.util.UUID;

public record Position(
        UUID id,
        UUID portfolioId,
        PositionSide side,
        PriceGbpPerMWh contractPrice,
        EnergyMWh volume,
        SettlementInterval settlementInterval,
        PositionStatus status,
        Instant createdAt) {

    public Position {
        Objects.requireNonNull(id, "id must not be null");
        Objects.requireNonNull(portfolioId, "portfolioId must not be null");
        Objects.requireNonNull(side, "side must not be null");
        Objects.requireNonNull(contractPrice, "contractPrice must not be null");
        Objects.requireNonNull(volume, "volume must not be null");
        Objects.requireNonNull(settlementInterval, "settlementInterval must not be null");
        Objects.requireNonNull(status, "status must not be null");
        Objects.requireNonNull(createdAt, "createdAt must not be null");
    }
}
