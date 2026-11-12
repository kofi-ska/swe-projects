package com.powerliq.application;

import com.powerliq.config.PowerLiqProperties;
import com.powerliq.financial.MarketPrice;
import com.powerliq.financial.Position;
import com.powerliq.financial.PositionStatus;
import com.powerliq.financial.SettlementInterval;
import com.powerliq.marketdata.ElexonClient;
import com.powerliq.marketdata.ElexonUnavailableException;
import com.powerliq.marketdata.MarketDataCheck;
import com.powerliq.marketdata.MarketDataStatus;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.Comparator;
import java.util.List;
import java.util.Set;
import java.util.stream.Collectors;
import org.springframework.stereotype.Service;

@Service
public class MarketDataService {
    private static final Duration STATUS_CACHE_TTL = Duration.ofSeconds(30);
    private static final Duration MAX_SYNC_WINDOW = Duration.ofDays(31);
    private final ElexonClient elexonClient;
    private final PositionRepository positions;
    private final MarketPriceRepository prices;
    private final PowerLiqProperties properties;
    private final Clock clock;
    private final Object statusLock = new Object();
    private volatile MarketDataCheck cachedStatus;

    public MarketDataService(ElexonClient elexonClient, PositionRepository positions,
            MarketPriceRepository prices, PowerLiqProperties properties, Clock clock) {
        this.elexonClient = elexonClient;
        this.positions = positions;
        this.prices = prices;
        this.properties = properties;
        this.clock = clock;
    }

    public MarketDataCheck check() {
        Instant checkedAt = Instant.now(clock);
        MarketDataCheck current = cachedStatus;
        if (isCached(current, checkedAt)) return current;
        synchronized (statusLock) {
            current = cachedStatus;
            if (isCached(current, checkedAt)) return current;
            cachedStatus = performCheck(checkedAt);
            return cachedStatus;
        }
    }

    private MarketDataCheck performCheck(Instant checkedAt) {
        try {
            List<MarketPrice> result = elexonClient.fetchMarketIndexPrices(checkedAt.minus(Duration.ofHours(2)), checkedAt);
            if (result.isEmpty()) {
                return new MarketDataCheck(MarketDataStatus.MISSING, false, 0, checkedAt, null,
                        "Elexon was reachable but returned no Market Index records");
            }
            Instant asOf = result.stream()
                    .map(price -> price.settlementInterval().deliveryEnd())
                    .max(Comparator.naturalOrder()).orElse(checkedAt);
            boolean fresh = !asOf.isBefore(checkedAt.minus(properties.marketData().maxAge()));
            return new MarketDataCheck(fresh ? MarketDataStatus.AVAILABLE : MarketDataStatus.STALE,
                    fresh, result.size(), checkedAt, asOf, "Elexon Market Index request succeeded");
        } catch (ElexonUnavailableException exception) {
            return new MarketDataCheck(MarketDataStatus.UNREACHABLE, false, 0, checkedAt, null,
                    exception.getMessage());
        }
    }

    private boolean isCached(MarketDataCheck check, Instant now) {
        return check != null && check.checkedAt().isAfter(now.minus(STATUS_CACHE_TTL));
    }

    public int syncActivePositions() {
        List<Position> active = positions.findAll().stream()
                .filter(position -> position.status() == PositionStatus.ACTIVE)
                .toList();
        if (active.isEmpty()) return 0;
        Instant from = active.stream().map(position -> position.settlementInterval().deliveryStart())
                .min(Comparator.naturalOrder()).orElseThrow();
        Instant to = active.stream().map(position -> position.settlementInterval().deliveryEnd())
                .max(Comparator.naturalOrder()).orElseThrow();
        if (Duration.between(from, to).compareTo(MAX_SYNC_WINDOW) > 0) {
            throw new ResourceLimitExceededException("active positions span more than the 31-day sync limit");
        }
        Set<SettlementInterval> requiredIntervals = active.stream()
                .map(Position::settlementInterval)
                .collect(Collectors.toUnmodifiableSet());
        List<MarketPrice> matchingPrices = elexonClient.fetchMarketIndexPrices(from, to).stream()
                .filter(price -> requiredIntervals.contains(price.settlementInterval()))
                .toList();
        return prices.retainAndPut(requiredIntervals, matchingPrices);
    }
}
