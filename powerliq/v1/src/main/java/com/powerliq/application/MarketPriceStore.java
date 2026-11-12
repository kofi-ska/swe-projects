package com.powerliq.application;

import com.powerliq.financial.MarketPrice;
import com.powerliq.financial.SettlementInterval;
import java.util.Collection;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;

public class MarketPriceStore implements MarketPriceRepository {
    private final ConcurrentHashMap<SettlementInterval, MarketPrice> prices = new ConcurrentHashMap<>();

    public synchronized int retainAndPut(
            Set<SettlementInterval> requiredIntervals, Collection<MarketPrice> incoming) {
        prices.keySet().removeIf(interval -> !requiredIntervals.contains(interval));
        incoming.forEach(price -> prices.merge(price.settlementInterval(), price, this::newer));
        return incoming.size();
    }

    public Map<SettlementInterval, MarketPrice> snapshot() {
        return Map.copyOf(prices);
    }

    private MarketPrice newer(MarketPrice current, MarketPrice candidate) {
        if (current.sourceTimestamp() == null) return candidate;
        if (candidate.sourceTimestamp() == null) return current;
        return candidate.sourceTimestamp().isAfter(current.sourceTimestamp()) ? candidate : current;
    }
}
