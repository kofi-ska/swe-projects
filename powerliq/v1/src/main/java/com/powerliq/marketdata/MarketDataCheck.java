package com.powerliq.marketdata;

import java.time.Instant;

public record MarketDataCheck(
        MarketDataStatus status,
        boolean fresh,
        int recordsReceived,
        Instant checkedAt,
        Instant marketDataAsOf,
        String message) {
}
