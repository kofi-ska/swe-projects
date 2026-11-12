package com.powerliq.application;

import com.powerliq.financial.MarketPrice;
import com.powerliq.financial.SettlementInterval;
import java.util.Collection;
import java.util.Map;
import java.util.Set;

public interface MarketPriceRepository {
    int retainAndPut(Set<SettlementInterval> requiredIntervals, Collection<MarketPrice> incoming);
    Map<SettlementInterval, MarketPrice> snapshot();
}
