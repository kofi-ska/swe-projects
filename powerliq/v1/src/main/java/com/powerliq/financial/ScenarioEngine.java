package com.powerliq.financial;

import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.stream.Collectors;

public final class ScenarioEngine {
    private final ValuationEngine valuationEngine;
    private final LiquidityCalculator liquidityCalculator;

    public ScenarioEngine(ValuationEngine valuationEngine, LiquidityCalculator liquidityCalculator) {
        this.valuationEngine = Objects.requireNonNull(valuationEngine, "valuationEngine must not be null");
        this.liquidityCalculator = Objects.requireNonNull(liquidityCalculator, "liquidityCalculator must not be null");
    }

    public ScenarioResult run(
            List<Position> positions,
            Map<SettlementInterval, MarketPrice> prices,
            MoneyGbp availableCash,
            MoneyGbp liquidityBuffer,
            PercentageShock shock) {
        Objects.requireNonNull(shock, "shock must not be null");

        PortfolioValuation baselineValuation = valuationEngine.value(positions, prices);
        LiquidityResult baseline = liquidityCalculator.calculate(
                baselineValuation, availableCash, liquidityBuffer);

        Map<SettlementInterval, MarketPrice> shockedPrices = prices.entrySet().stream()
                .collect(Collectors.toUnmodifiableMap(
                        Map.Entry::getKey,
                        entry -> shock(entry.getValue(), shock)));
        PortfolioValuation scenarioValuation = valuationEngine.value(positions, shockedPrices);
        LiquidityResult scenario = liquidityCalculator.calculate(
                scenarioValuation, availableCash, liquidityBuffer);

        return new ScenarioResult(
                shock,
                baseline,
                scenario,
                scenario.liquidityHeadroom().subtract(baseline.liquidityHeadroom()));
    }

    private MarketPrice shock(MarketPrice price, PercentageShock shock) {
        return new MarketPrice(
                price.provider(),
                price.settlementInterval(),
                price.price().apply(shock),
                price.sourceTimestamp(),
                price.retrievedAt());
    }
}
