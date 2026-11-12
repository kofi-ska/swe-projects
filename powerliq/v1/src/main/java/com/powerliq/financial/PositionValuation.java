package com.powerliq.financial;

import java.util.Objects;
import java.util.UUID;

public record PositionValuation(UUID positionId, MoneyGbp mtm, MoneyGbp collateral) {
    public PositionValuation {
        Objects.requireNonNull(positionId, "positionId must not be null");
        Objects.requireNonNull(mtm, "mtm must not be null");
        Objects.requireNonNull(collateral, "collateral must not be null");
        if (collateral.isNegative()) {
            throw new IllegalArgumentException("collateral must not be negative");
        }
    }
}
