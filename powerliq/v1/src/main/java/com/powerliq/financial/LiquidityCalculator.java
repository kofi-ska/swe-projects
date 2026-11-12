package com.powerliq.financial;

import java.util.Objects;

public final class LiquidityCalculator {
    public LiquidityResult calculate(
            PortfolioValuation valuation,
            MoneyGbp availableCash,
            MoneyGbp liquidityBuffer) {
        Objects.requireNonNull(valuation, "valuation must not be null");
        Objects.requireNonNull(availableCash, "availableCash must not be null");
        Objects.requireNonNull(liquidityBuffer, "liquidityBuffer must not be null");

        MoneyGbp headroom = availableCash.subtract(valuation.estimatedCollateral());
        LiquidityStatus status;
        if (headroom.isNegative()) {
            status = LiquidityStatus.SHORTFALL;
        } else if (headroom.compareTo(liquidityBuffer) < 0) {
            status = LiquidityStatus.WARNING;
        } else {
            status = LiquidityStatus.HEALTHY;
        }
        return new LiquidityResult(valuation, availableCash, liquidityBuffer, headroom, status);
    }
}
