package com.powerliq.events;

public sealed interface PowerLiqEvent permits
        PositionCreatedV1, PositionSettledV1, PortfolioLiquiditySettingsChangedV1, MarketPriceRevisionV1 {
}
