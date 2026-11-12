package com.powerliq.financial;

import java.util.Objects;

public record ScenarioResult(
        PercentageShock shock,
        LiquidityResult baseline,
        LiquidityResult scenario,
        MoneyGbp liquidityDifference) {

    public ScenarioResult {
        Objects.requireNonNull(shock, "shock must not be null");
        Objects.requireNonNull(baseline, "baseline must not be null");
        Objects.requireNonNull(scenario, "scenario must not be null");
        Objects.requireNonNull(liquidityDifference, "liquidityDifference must not be null");
    }
}
