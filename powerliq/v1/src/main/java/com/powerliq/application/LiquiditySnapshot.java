package com.powerliq.application;

import com.powerliq.financial.LiquidityResult;
import java.time.Instant;

public record LiquiditySnapshot(
        LiquidityResult result,
        DataStatus dataStatus,
        Instant marketDataAsOf,
        Instant calculatedAt) {
}
