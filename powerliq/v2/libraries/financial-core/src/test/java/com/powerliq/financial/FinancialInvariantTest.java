package com.powerliq.financial;

import static com.powerliq.financial.TestFixtures.marketPrice;
import static com.powerliq.financial.TestFixtures.money;
import static com.powerliq.financial.TestFixtures.position;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;

import java.math.BigDecimal;
import java.util.Random;
import org.junit.jupiter.api.Test;

class FinancialInvariantTest {
    private final ValuationEngine engine = new ValuationEngine();

    @Test
    void longAndShortMtmAreOppositesAcrossManyInputs() {
        Random random = new Random(42);

        for (int i = 0; i < 1_000; i++) {
            String contract = BigDecimal.valueOf(random.nextDouble(-100, 500)).toPlainString();
            String market = BigDecimal.valueOf(random.nextDouble(-100, 500)).toPlainString();
            String volume = BigDecimal.valueOf(random.nextDouble(0.001, 2_000)).toPlainString();

            PositionValuation longValue = engine.value(position(PositionSide.LONG, contract, volume), marketPrice(market));
            PositionValuation shortValue = engine.value(position(PositionSide.SHORT, contract, volume), marketPrice(market));

            assertEquals(longValue.mtm(), shortValue.mtm().negate());
            assertFalse(longValue.collateral().isNegative());
            assertFalse(shortValue.collateral().isNegative());
        }
    }

    @Test
    void decimalValuesUseDocumentedHalfEvenRounding() {
        PositionValuation result = engine.value(
                position(PositionSide.LONG, "0", "1"),
                marketPrice("1.005"));

        assertEquals(money("1.00"), result.mtm());
    }
}
