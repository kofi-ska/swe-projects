package com.powerliq.financial;

import java.math.BigDecimal;

public record MoneyGbp(BigDecimal value) implements Comparable<MoneyGbp> {
    public static final MoneyGbp ZERO = new MoneyGbp(BigDecimal.ZERO);

    public MoneyGbp {
        value = FinancialRules.requireAndScale(value, FinancialRules.MONEY_SCALE, "money");
    }

    public MoneyGbp add(MoneyGbp other) {
        return new MoneyGbp(value.add(other.value, FinancialRules.CALCULATION_CONTEXT));
    }

    public MoneyGbp subtract(MoneyGbp other) {
        return new MoneyGbp(value.subtract(other.value, FinancialRules.CALCULATION_CONTEXT));
    }

    public MoneyGbp negate() {
        return new MoneyGbp(value.negate());
    }

    public boolean isNegative() {
        return value.signum() < 0;
    }

    @Override
    public int compareTo(MoneyGbp other) {
        return value.compareTo(other.value);
    }
}
