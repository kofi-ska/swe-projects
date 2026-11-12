package com.powerliq.events;

import java.math.BigDecimal;
import java.time.Instant;

public record MarketPriceRevisionV1(
        String provider,
        Instant deliveryStart,
        Instant deliveryEnd,
        BigDecimal priceGbpPerMWh,
        String sourceRevision,
        Instant sourceTimestamp,
        Instant retrievedAt) implements PowerLiqEvent {
}
