package com.powerliq.position.api;

import com.powerliq.financial.PositionSide;
import com.powerliq.position.domain.Position;
import com.powerliq.position.domain.PositionStatus;
import java.math.BigDecimal;
import java.time.Instant;
import java.time.LocalDate;
import java.util.UUID;

public record PositionResponse(
        UUID id,
        UUID portfolioId,
        PositionSide side,
        BigDecimal contractPriceGbpPerMWh,
        BigDecimal volumeMWh,
        LocalDate settlementDate,
        int settlementPeriod,
        Instant deliveryStart,
        Instant deliveryEnd,
        PositionStatus status,
        Instant createdAt) {

    static PositionResponse from(Position position) {
        return new PositionResponse(position.id(), position.portfolioId(), position.side(),
                position.contractPriceGbpPerMWh(), position.volumeMWh(), position.settlementDate(),
                position.settlementPeriod(), position.deliveryStart(), position.deliveryEnd(),
                position.status(), position.createdAt());
    }
}
