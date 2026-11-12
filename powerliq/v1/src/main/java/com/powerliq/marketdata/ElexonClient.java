package com.powerliq.marketdata;

import com.powerliq.financial.MarketPrice;
import java.time.Instant;
import java.util.List;

public interface ElexonClient {
    List<MarketPrice> fetchMarketIndexPrices(Instant from, Instant to);
}
