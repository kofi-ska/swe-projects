package com.powerliq.financial;

import java.util.Objects;

public record LiquidityResult(
        PortfolioValuation valuation,
        MoneyGbp availableCash,
        MoneyGbp liquidityBuffer,
        MoneyGbp liquidityHeadroom,
        LiquidityStatus status) {

    public LiquidityResult {
        Objects.requireNonNull(valuation, "valuation must not be null");
        Objects.requireNonNull(availableCash, "availableCash must not be null");
        Objects.requireNonNull(liquidityBuffer, "liquidityBuffer must not be null");
        Objects.requireNonNull(liquidityHeadroom, "liquidityHeadroom must not be null");
        Objects.requireNonNull(status, "status must not be null");
        if (availableCash.isNegative()) {
            throw new IllegalArgumentException("availableCash must not be negative");
        }
        if (liquidityBuffer.isNegative()) {
            throw new IllegalArgumentException("liquidityBuffer must not be negative");
        }
    }
}
