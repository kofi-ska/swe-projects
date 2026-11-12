package com.powerliq.financial;

import java.util.List;
import java.util.Objects;

public record PortfolioValuation(
        List<PositionValuation> positions,
        MoneyGbp portfolioMtm,
        MoneyGbp estimatedCollateral) {

    public PortfolioValuation {
        positions = List.copyOf(Objects.requireNonNull(positions, "positions must not be null"));
        Objects.requireNonNull(portfolioMtm, "portfolioMtm must not be null");
        Objects.requireNonNull(estimatedCollateral, "estimatedCollateral must not be null");
    }
}
