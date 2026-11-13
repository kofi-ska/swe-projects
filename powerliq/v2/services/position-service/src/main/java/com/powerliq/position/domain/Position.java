package com.powerliq.position.domain;

import com.powerliq.financial.PositionSide;
import java.math.BigDecimal;
import java.time.Instant;
import java.time.LocalDate;
import java.util.UUID;

public record Position(
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
}
