package com.powerliq.application;

import com.powerliq.financial.ScenarioResult;
import java.time.Instant;

public record ScenarioSnapshot(
        ScenarioResult result,
        DataStatus dataStatus,
        Instant marketDataAsOf,
        Instant calculatedAt) {
}
