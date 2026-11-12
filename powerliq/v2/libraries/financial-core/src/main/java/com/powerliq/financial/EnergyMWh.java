package com.powerliq.financial;

import java.math.BigDecimal;

public record EnergyMWh(BigDecimal value) {
    private static final BigDecimal MAXIMUM = new BigDecimal("1000000");

    public EnergyMWh {
        value = FinancialRules.requireAndScale(value, FinancialRules.ENERGY_SCALE, "energy");
        if (value.signum() <= 0) {
            throw new IllegalArgumentException("energy must be greater than zero");
        }
        if (value.compareTo(MAXIMUM) > 0) {
            throw new IllegalArgumentException("energy must not exceed 1000000 MWh");
        }
    }
}
