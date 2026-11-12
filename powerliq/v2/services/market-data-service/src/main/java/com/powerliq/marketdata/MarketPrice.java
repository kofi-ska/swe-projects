package com.powerliq.marketdata;
import java.math.BigDecimal;import java.time.Instant;import java.util.UUID;
public record MarketPrice(UUID eventId,String provider,Instant deliveryStart,Instant deliveryEnd,
 BigDecimal priceGbpPerMWh,String sourceRevision,Instant sourceTimestamp,Instant retrievedAt){}
