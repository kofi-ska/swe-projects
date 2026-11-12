package com.powerliq.events;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.UUID;

public record PortfolioLiquiditySettingsChangedV1(
        UUID portfolioId,
        BigDecimal availableCashGbp,
        BigDecimal liquidityBufferGbp,
        long configurationVersion,
        Instant changedAt) implements PowerLiqEvent {
}
