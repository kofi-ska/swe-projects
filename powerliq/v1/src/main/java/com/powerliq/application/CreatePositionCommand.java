package com.powerliq.application;

import com.powerliq.financial.PositionSide;
import java.math.BigDecimal;
import java.time.LocalDate;
import java.util.UUID;

public record CreatePositionCommand(
        UUID portfolioId,
        PositionSide side,
        BigDecimal contractPriceGbpPerMWh,
        BigDecimal volumeMWh,
        LocalDate settlementDate,
        int settlementPeriod) {
}
