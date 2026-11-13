package com.powerliq.financial;

import java.math.BigDecimal;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.HashSet;
import java.util.Set;
import java.util.UUID;

public final class ValuationEngine {

    public PositionValuation value(Position position, MarketPrice marketPrice) {
        Objects.requireNonNull(position, "position must not be null");
        Objects.requireNonNull(marketPrice, "marketPrice must not be null");
        if (position.status() != PositionStatus.ACTIVE) {
            throw new IllegalArgumentException("only active positions can be valued");
        }
        if (!position.settlementInterval().equals(marketPrice.settlementInterval())) {
            throw new IllegalArgumentException("market price does not match the position settlement interval");
        }

        BigDecimal priceDifference = marketPrice.price().value()
                .subtract(position.contractPrice().value(), FinancialRules.CALCULATION_CONTEXT);
        BigDecimal unroundedMtm = priceDifference
                .multiply(position.volume().value(), FinancialRules.CALCULATION_CONTEXT)
                .multiply(BigDecimal.valueOf(position.side().sign()), FinancialRules.CALCULATION_CONTEXT);
        MoneyGbp mtm = new MoneyGbp(unroundedMtm);
        MoneyGbp collateral = mtm.isNegative() ? mtm.negate() : MoneyGbp.ZERO;
        return new PositionValuation(position.id(), mtm, collateral);
    }

    public PortfolioValuation value(
            List<Position> positions,
            Map<SettlementInterval, MarketPrice> prices) {
        Objects.requireNonNull(positions, "positions must not be null");
        Objects.requireNonNull(prices, "prices must not be null");

        List<PositionValuation> valuations = new ArrayList<>();
        MoneyGbp totalMtm = MoneyGbp.ZERO;
        MoneyGbp totalCollateral = MoneyGbp.ZERO;
        UUID portfolioId = null;
        Set<UUID> seenPositionIds = new HashSet<>();

        for (Position position : positions) {
            Objects.requireNonNull(position, "positions must not contain null");
            if (portfolioId == null) {
                portfolioId = position.portfolioId();
            } else if (!portfolioId.equals(position.portfolioId())) {
                throw new IllegalArgumentException("positions from different portfolios cannot be valued together");
            }
            if (!seenPositionIds.add(position.id())) {
                throw new IllegalArgumentException("duplicate position id: " + position.id());
            }
            if (position.status() != PositionStatus.ACTIVE) {
                continue;
            }
            MarketPrice price = prices.get(position.settlementInterval());
            if (price == null) {
                throw new IncompleteValuationException(
                        "missing market price for " + position.settlementInterval());
            }
            PositionValuation valuation = value(position, price);
            valuations.add(valuation);
            totalMtm = totalMtm.add(valuation.mtm());
            totalCollateral = totalCollateral.add(valuation.collateral());
        }

        return new PortfolioValuation(valuations, totalMtm, totalCollateral);
    }
}
