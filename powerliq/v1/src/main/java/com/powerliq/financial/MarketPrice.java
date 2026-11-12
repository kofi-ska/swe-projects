package com.powerliq.financial;

import java.time.Instant;
import java.util.Objects;

public record MarketPrice(
        String provider,
        SettlementInterval settlementInterval,
        PriceGbpPerMWh price,
        Instant sourceTimestamp,
        Instant retrievedAt) {

    public MarketPrice {
        if (provider == null || provider.isBlank()) {
            throw new IllegalArgumentException("provider must not be blank");
        }
        Objects.requireNonNull(settlementInterval, "settlementInterval must not be null");
        Objects.requireNonNull(price, "price must not be null");
        Objects.requireNonNull(retrievedAt, "retrievedAt must not be null");
    }
}
