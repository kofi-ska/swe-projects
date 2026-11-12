package com.powerliq.financial;

import static com.powerliq.financial.TestFixtures.marketPrice;
import static com.powerliq.financial.TestFixtures.money;
import static com.powerliq.financial.TestFixtures.position;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

import java.util.List;
import java.util.Map;
import java.util.UUID;
import org.junit.jupiter.api.Test;

class ValuationEngineTest {
    private final ValuationEngine engine = new ValuationEngine();

    @Test
    void longPositionGainsWhenMarketPriceExceedsContractPrice() {
        PositionValuation result = engine.value(position(PositionSide.LONG, "80", "100"), marketPrice("100"));

        assertEquals(money("2000"), result.mtm());
        assertEquals(MoneyGbp.ZERO, result.collateral());
    }

    @Test
    void shortPositionHasOppositeMtmAndRequiresCollateral() {
        PositionValuation result = engine.value(position(PositionSide.SHORT, "80", "100"), marketPrice("100"));

        assertEquals(money("-2000"), result.mtm());
        assertEquals(money("2000"), result.collateral());
    }

    @Test
    void equalContractAndMarketPriceProducesZeroMtm() {
        PositionValuation result = engine.value(position(PositionSide.LONG, "85.50", "72.125"), marketPrice("85.50"));
        assertEquals(MoneyGbp.ZERO, result.mtm());
    }

    @Test
    void portfolioMtmIsSumOfPositionMtmButCollateralIsGrossLoss() {
        Position longPosition = position(PositionSide.LONG, "80", "100");
        Position shortPosition = position(PositionSide.SHORT, "90", "100");

        PortfolioValuation result = engine.value(
                List.of(longPosition, shortPosition),
                Map.of(TestFixtures.INTERVAL, marketPrice("100")));

        assertEquals(money("1000"), result.portfolioMtm());
        assertEquals(money("1000"), result.estimatedCollateral());
    }

    @Test
    void missingPriceCannotProduceAValuation() {
        assertThrows(
                IncompleteValuationException.class,
                () -> engine.value(List.of(position(PositionSide.LONG, "80", "100")), Map.of()));
    }

    @Test
    void settledPositionsAreExcluded() {
        Position active = position(PositionSide.LONG, "80", "100");
        Position settled = new Position(
                active.id(), active.portfolioId(), active.side(), active.contractPrice(), active.volume(),
                active.settlementInterval(), PositionStatus.SETTLED, active.createdAt());

        PortfolioValuation result = engine.value(List.of(settled), Map.of());

        assertEquals(MoneyGbp.ZERO, result.portfolioMtm());
        assertEquals(MoneyGbp.ZERO, result.estimatedCollateral());
        assertEquals(0, result.positions().size());
    }

    @Test
    void rejectsDuplicatePositionIdsToPreventDoubleCounting() {
        Position position = position(PositionSide.LONG, "80", "100");

        assertThrows(
                IllegalArgumentException.class,
                () -> engine.value(
                        List.of(position, position),
                        Map.of(TestFixtures.INTERVAL, marketPrice("100"))));
    }

    @Test
    void rejectsPositionsFromDifferentPortfolios() {
        Position first = position(PositionSide.LONG, "80", "100");
        Position second = new Position(
                UUID.randomUUID(), UUID.randomUUID(), first.side(), first.contractPrice(), first.volume(),
                first.settlementInterval(), first.status(), first.createdAt());

        assertThrows(
                IllegalArgumentException.class,
                () -> engine.value(
                        List.of(first, second),
                        Map.of(TestFixtures.INTERVAL, marketPrice("100"))));
    }
}
