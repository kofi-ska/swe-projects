package com.powerliq.api;

import com.powerliq.application.CreatePositionCommand;
import com.powerliq.financial.PositionSide;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Positive;
import jakarta.validation.constraints.DecimalMax;
import java.math.BigDecimal;
import java.time.LocalDate;
import java.util.UUID;

public record PositionRequest(
        @NotNull UUID portfolioId,
        @NotNull PositionSide side,
        @NotNull BigDecimal contractPriceGbpPerMWh,
        @NotNull @Positive @DecimalMax("1000000") BigDecimal volumeMWh,
        @NotNull LocalDate settlementDate,
        @Positive int settlementPeriod) {

    CreatePositionCommand toCommand() {
        return new CreatePositionCommand(portfolioId, side, contractPriceGbpPerMWh, volumeMWh,
                settlementDate, settlementPeriod);
    }
}
