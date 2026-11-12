package com.powerliq.api;

import com.powerliq.financial.LiquidityResult;
import com.powerliq.financial.LiquidityStatus;
import com.powerliq.application.DataStatus;
import com.powerliq.application.LiquiditySnapshot;
import java.math.BigDecimal;
import java.time.Instant;
import java.util.UUID;

public record LiquidityResponse(
        UUID portfolioId,
        BigDecimal portfolioMtmGbp,
        BigDecimal estimatedCollateralGbp,
        BigDecimal availableCashGbp,
        BigDecimal liquidityHeadroomGbp,
        BigDecimal liquidityBufferGbp,
        LiquidityStatus liquidityStatus,
        boolean valuationComplete,
        DataStatus dataStatus,
        Instant marketDataAsOf,
        Instant calculatedAt) {

    static LiquidityResponse from(UUID portfolioId, LiquiditySnapshot snapshot) {
        LiquidityResult result = snapshot.result();
        return new LiquidityResponse(portfolioId, result.valuation().portfolioMtm().value(),
                result.valuation().estimatedCollateral().value(), result.availableCash().value(),
                result.liquidityHeadroom().value(), result.liquidityBuffer().value(), result.status(), true,
                snapshot.dataStatus(), snapshot.marketDataAsOf(), snapshot.calculatedAt());
    }
}
