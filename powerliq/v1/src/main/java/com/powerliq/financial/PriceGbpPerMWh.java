package com.powerliq.financial;

import java.math.BigDecimal;

public record PriceGbpPerMWh(BigDecimal value) {
    public PriceGbpPerMWh {
        value = FinancialRules.requireAndScale(value, FinancialRules.PRICE_SCALE, "price");
    }

    public PriceGbpPerMWh apply(PercentageShock shock) {
        BigDecimal multiplier = BigDecimal.ONE.add(shock.value(), FinancialRules.CALCULATION_CONTEXT);
        return new PriceGbpPerMWh(value.multiply(multiplier, FinancialRules.CALCULATION_CONTEXT));
    }
}
