package com.powerliq.api;

import com.powerliq.financial.ScenarioResult;
import com.powerliq.application.LiquiditySnapshot;
import com.powerliq.application.ScenarioSnapshot;
import java.math.BigDecimal;
import java.util.UUID;

public record ScenarioResponse(
        BigDecimal appliedShock,
        LiquidityResponse baseline,
        LiquidityResponse scenario,
        BigDecimal liquidityDifferenceGbp) {

    static ScenarioResponse from(UUID portfolioId, ScenarioSnapshot snapshot) {
        ScenarioResult result = snapshot.result();
        LiquiditySnapshot baseline = new LiquiditySnapshot(
                result.baseline(), snapshot.dataStatus(), snapshot.marketDataAsOf(), snapshot.calculatedAt());
        LiquiditySnapshot scenario = new LiquiditySnapshot(
                result.scenario(), snapshot.dataStatus(), snapshot.marketDataAsOf(), snapshot.calculatedAt());
        return new ScenarioResponse(result.shock().value(), LiquidityResponse.from(portfolioId, baseline),
                LiquidityResponse.from(portfolioId, scenario), result.liquidityDifference().value());
    }
}
