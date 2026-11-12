package com.powerliq.api;

import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.DecimalMax;
import jakarta.validation.constraints.NotNull;
import java.math.BigDecimal;

public record ScenarioRequest(
        @NotNull @DecimalMin("-1.00") @DecimalMax("10.00") BigDecimal percentageShock) {
}
