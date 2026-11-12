package com.powerliq.financial;

import java.math.BigDecimal;
import java.math.MathContext;
import java.math.RoundingMode;
import java.util.Objects;

final class FinancialRules {
    static final int MONEY_SCALE = 2;
    static final int PRICE_SCALE = 5;
    static final int ENERGY_SCALE = 3;
    static final RoundingMode ROUNDING = RoundingMode.HALF_EVEN;
    static final MathContext CALCULATION_CONTEXT = MathContext.DECIMAL128;

    private FinancialRules() {
    }

    static BigDecimal requireAndScale(BigDecimal value, int scale, String name) {
        Objects.requireNonNull(value, name + " must not be null");
        return value.setScale(scale, ROUNDING);
    }
}
