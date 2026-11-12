package com.powerliq.api;

import com.powerliq.financial.Position;
import com.powerliq.financial.PositionSide;
import com.powerliq.financial.PositionStatus;
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
                position.contractPrice().value(), position.volume().value(),
                position.settlementInterval().settlementDate(), position.settlementInterval().settlementPeriod(),
                position.settlementInterval().deliveryStart(), position.settlementInterval().deliveryEnd(),
                position.status(), position.createdAt());
    }
}
