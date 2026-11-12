package com.powerliq.application;

import com.powerliq.financial.MarketPrice;
import com.powerliq.financial.PriceGbpPerMWh;
import com.powerliq.financial.SettlementInterval;
import java.sql.Timestamp;
import java.util.Collection;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Set;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;
import org.springframework.transaction.annotation.Transactional;

@Repository
public class JdbcMarketPriceRepository implements MarketPriceRepository {
    private final JdbcTemplate jdbc;

    public JdbcMarketPriceRepository(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    @Override
    @Transactional
    public int retainAndPut(Set<SettlementInterval> requiredIntervals, Collection<MarketPrice> incoming) {
        jdbc.update("DELETE FROM market_prices");
        for (MarketPrice price : incoming) {
            jdbc.update("""
                    INSERT INTO market_prices
                      (provider, settlement_date, settlement_period, delivery_start, delivery_end,
                       price, source_timestamp, retrieved_at)
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                    ON CONFLICT (provider, settlement_date, settlement_period) DO UPDATE SET
                      delivery_start = EXCLUDED.delivery_start,
                      delivery_end = EXCLUDED.delivery_end,
                      price = EXCLUDED.price,
                      source_timestamp = EXCLUDED.source_timestamp,
                      retrieved_at = EXCLUDED.retrieved_at
                    """, price.provider(), price.settlementInterval().settlementDate(),
                    price.settlementInterval().settlementPeriod(),
                    Timestamp.from(price.settlementInterval().deliveryStart()),
                    Timestamp.from(price.settlementInterval().deliveryEnd()), price.price().value(),
                    price.sourceTimestamp() == null ? null : Timestamp.from(price.sourceTimestamp()),
                    Timestamp.from(price.retrievedAt()));
        }
        return incoming.size();
    }

    @Override
    public Map<SettlementInterval, MarketPrice> snapshot() {
        Map<SettlementInterval, MarketPrice> result = new LinkedHashMap<>();
        jdbc.query("SELECT * FROM market_prices ORDER BY delivery_start", row -> {
            SettlementInterval interval = new SettlementInterval(
                    row.getObject("settlement_date", java.time.LocalDate.class), row.getInt("settlement_period"),
                    row.getTimestamp("delivery_start").toInstant(),
                    row.getTimestamp("delivery_end").toInstant());
            Timestamp source = row.getTimestamp("source_timestamp");
            result.put(interval, new MarketPrice(row.getString("provider"), interval,
                    new PriceGbpPerMWh(row.getBigDecimal("price")),
                    source == null ? null : source.toInstant(),
                    row.getTimestamp("retrieved_at").toInstant()));
        });
        return Map.copyOf(result);
    }
}
