package com.powerliq.application;

import com.powerliq.config.PowerLiqProperties;
import com.powerliq.financial.LiquidityCalculator;
import com.powerliq.financial.LiquidityResult;
import com.powerliq.financial.MoneyGbp;
import com.powerliq.financial.PercentageShock;
import com.powerliq.financial.PortfolioValuation;
import com.powerliq.financial.Position;
import com.powerliq.financial.ScenarioEngine;
import com.powerliq.financial.ScenarioResult;
import com.powerliq.financial.ValuationEngine;
import java.math.BigDecimal;
import java.time.Clock;
import java.time.Instant;
import java.util.Comparator;
import java.util.Map;
import java.util.List;
import java.util.UUID;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Isolation;
import org.springframework.transaction.annotation.Transactional;

@Service
public class LiquidityApplicationService {
    private final PositionRepository positions;
    private final MarketPriceRepository prices;
    private final PowerLiqProperties properties;
    private final Clock clock;
    private final ValuationEngine valuationEngine = new ValuationEngine();
    private final LiquidityCalculator liquidityCalculator = new LiquidityCalculator();
    private final ScenarioEngine scenarioEngine = new ScenarioEngine(valuationEngine, liquidityCalculator);

    public LiquidityApplicationService(
            PositionRepository positions, MarketPriceRepository prices,
            PowerLiqProperties properties, Clock clock) {
        this.positions = positions;
        this.prices = prices;
        this.properties = properties;
        this.clock = clock;
    }

    @Transactional(readOnly = true, isolation = Isolation.REPEATABLE_READ)
    public LiquiditySnapshot liquidity(UUID portfolioId) {
        validatePortfolio(portfolioId);
        List<Position> portfolioPositions = positions.findByPortfolioId(portfolioId);
        Map<com.powerliq.financial.SettlementInterval, com.powerliq.financial.MarketPrice> priceSnapshot =
                prices.snapshot();
        PortfolioValuation valuation = valuationEngine.value(portfolioPositions, priceSnapshot);
        Instant now = Instant.now(clock);
        return new LiquiditySnapshot(
                liquidityCalculator.calculate(valuation, availableCash(), liquidityBuffer()),
                dataStatus(portfolioPositions, priceSnapshot, now),
                marketDataAsOf(portfolioPositions, priceSnapshot), now);
    }

    @Transactional(readOnly = true, isolation = Isolation.REPEATABLE_READ)
    public ScenarioSnapshot scenario(UUID portfolioId, BigDecimal shock) {
        validatePortfolio(portfolioId);
        List<Position> portfolioPositions = positions.findByPortfolioId(portfolioId);
        Map<com.powerliq.financial.SettlementInterval, com.powerliq.financial.MarketPrice> priceSnapshot =
                prices.snapshot();
        Instant now = Instant.now(clock);
        return new ScenarioSnapshot(
                scenarioEngine.run(portfolioPositions, priceSnapshot,
                        availableCash(), liquidityBuffer(), new PercentageShock(shock)),
                dataStatus(portfolioPositions, priceSnapshot, now),
                marketDataAsOf(portfolioPositions, priceSnapshot), now);
    }

    private void validatePortfolio(UUID portfolioId) {
        if (!properties.portfolio().id().equals(portfolioId)) {
            throw new PortfolioNotFoundException(portfolioId);
        }
    }

    private MoneyGbp availableCash() {
        return new MoneyGbp(properties.portfolio().availableCashGbp());
    }

    private MoneyGbp liquidityBuffer() {
        return new MoneyGbp(properties.portfolio().liquidityBufferGbp());
    }

    private DataStatus dataStatus(
            List<Position> portfolioPositions,
            Map<com.powerliq.financial.SettlementInterval, com.powerliq.financial.MarketPrice> priceSnapshot,
            Instant now) {
        List<com.powerliq.financial.MarketPrice> used = usedPrices(portfolioPositions, priceSnapshot);
        if (used.isEmpty()) return DataStatus.NOT_REQUIRED;
        Instant threshold = now.minus(properties.marketData().maxAge());
        return used.stream().allMatch(price -> !price.retrievedAt().isBefore(threshold))
                ? DataStatus.FRESH : DataStatus.STALE;
    }

    private Instant marketDataAsOf(
            List<Position> portfolioPositions,
            Map<com.powerliq.financial.SettlementInterval, com.powerliq.financial.MarketPrice> priceSnapshot) {
        return usedPrices(portfolioPositions, priceSnapshot).stream()
                .map(price -> price.settlementInterval().deliveryEnd())
                .max(Comparator.naturalOrder()).orElse(null);
    }

    private List<com.powerliq.financial.MarketPrice> usedPrices(
            List<Position> portfolioPositions,
            Map<com.powerliq.financial.SettlementInterval, com.powerliq.financial.MarketPrice> priceSnapshot) {
        return portfolioPositions.stream()
                .filter(position -> position.status() == com.powerliq.financial.PositionStatus.ACTIVE)
                .map(position -> priceSnapshot.get(position.settlementInterval()))
                .filter(java.util.Objects::nonNull)
                .toList();
    }
}
