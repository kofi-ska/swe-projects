package com.powerliq.financial;

import static com.powerliq.financial.TestFixtures.marketPrice;
import static com.powerliq.financial.TestFixtures.money;
import static com.powerliq.financial.TestFixtures.position;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;

class LiquidityCalculatorTest {
    private final ValuationEngine valuationEngine = new ValuationEngine();
    private final LiquidityCalculator calculator = new LiquidityCalculator();

    @Test
    void classifiesHealthyAtTheBufferBoundary() {
        LiquidityResult result = calculate("102000");
        assertEquals(money("100000"), result.liquidityHeadroom());
        assertEquals(LiquidityStatus.HEALTHY, result.status());
    }

    @Test
    void classifiesWarningBelowTheBuffer() {
        LiquidityResult result = calculate("101999.99");
        assertEquals(money("99999.99"), result.liquidityHeadroom());
        assertEquals(LiquidityStatus.WARNING, result.status());
    }

    @Test
    void zeroHeadroomIsWarningNotShortfall() {
        LiquidityResult result = calculate("2000");
        assertEquals(MoneyGbp.ZERO, result.liquidityHeadroom());
        assertEquals(LiquidityStatus.WARNING, result.status());
    }

    @Test
    void negativeHeadroomIsShortfall() {
        LiquidityResult result = calculate("1999.99");
        assertEquals(money("-0.01"), result.liquidityHeadroom());
        assertEquals(LiquidityStatus.SHORTFALL, result.status());
    }

    @Test
    void liquidityPlusCollateralAlwaysEqualsCash() {
        LiquidityResult result = calculate("250000");
        MoneyGbp reconstructedCash = result.liquidityHeadroom().add(result.valuation().estimatedCollateral());
        assertEquals(result.availableCash(), reconstructedCash);
    }

    private LiquidityResult calculate(String cash) {
        PortfolioValuation valuation = valuationEngine.value(
                List.of(position(PositionSide.SHORT, "80", "100")),
                Map.of(TestFixtures.INTERVAL, marketPrice("100")));
        return calculator.calculate(valuation, money(cash), money("100000"));
    }
}
