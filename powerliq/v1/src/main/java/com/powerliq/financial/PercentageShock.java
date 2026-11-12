package com.powerliq.financial;

import java.math.BigDecimal;
import java.util.Objects;

public record PercentageShock(BigDecimal value) {
    private static final BigDecimal MINIMUM = new BigDecimal("-1.00");
    private static final BigDecimal MAXIMUM = new BigDecimal("10.00");

    public PercentageShock {
        Objects.requireNonNull(value, "shock must not be null");
        if (value.compareTo(MINIMUM) < 0) {
            throw new IllegalArgumentException("shock must not be below -1.00");
        }
        if (value.compareTo(MAXIMUM) > 0) {
            throw new IllegalArgumentException("shock must not exceed 10.00");
        }
    }
}
