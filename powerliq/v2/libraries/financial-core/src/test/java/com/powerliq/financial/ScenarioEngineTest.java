package com.powerliq.financial;

import static com.powerliq.financial.TestFixtures.marketPrice;
import static com.powerliq.financial.TestFixtures.money;
import static com.powerliq.financial.TestFixtures.position;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

import java.math.BigDecimal;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;

class ScenarioEngineTest {
    private final ScenarioEngine engine = new ScenarioEngine(new ValuationEngine(), new LiquidityCalculator());

    @Test
    void twentyPercentPriceIncreaseRevaluesThroughTheSameEngine() {
        MarketPrice originalPrice = marketPrice("100");
        Map<SettlementInterval, MarketPrice> prices = Map.of(TestFixtures.INTERVAL, originalPrice);

        ScenarioResult result = engine.run(
                List.of(position(PositionSide.SHORT, "80", "100")),
                prices,
                money("200000"),
                money("100000"),
                new PercentageShock(new BigDecimal("0.20")));

        assertEquals(money("198000"), result.baseline().liquidityHeadroom());
        assertEquals(money("196000"), result.scenario().liquidityHeadroom());
        assertEquals(money("-2000"), result.liquidityDifference());
        assertEquals(new PriceGbpPerMWh(new BigDecimal("100")), originalPrice.price());
    }

    @Test
    void rejectsShockBelowMinusOneHundredPercent() {
        assertThrows(
                IllegalArgumentException.class,
                () -> new PercentageShock(new BigDecimal("-1.00001")));
    }

    @Test
    void rejectsUnboundedPositiveShock() {
        assertThrows(
                IllegalArgumentException.class,
                () -> new PercentageShock(new BigDecimal("10.00001")));
    }
}
