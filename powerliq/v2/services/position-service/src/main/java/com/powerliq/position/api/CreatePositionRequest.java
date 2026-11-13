package com.powerliq.position.api;

import com.powerliq.financial.PositionSide;
import com.powerliq.position.application.CreatePositionCommand;
import jakarta.validation.constraints.DecimalMax;
import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotNull;
import java.math.BigDecimal;
import java.time.LocalDate;
import java.util.UUID;

public record CreatePositionRequest(
        @NotNull UUID portfolioId,
        @NotNull PositionSide side,
        @NotNull BigDecimal contractPriceGbpPerMWh,
        @NotNull @DecimalMin(value = "0", inclusive = false) @DecimalMax("1000000") BigDecimal volumeMWh,
        @NotNull LocalDate settlementDate,
        @Min(1) @Max(50) int settlementPeriod) {

    CreatePositionCommand toCommand() {
        return new CreatePositionCommand(portfolioId, side, contractPriceGbpPerMWh, volumeMWh,
                settlementDate, settlementPeriod);
    }
}
