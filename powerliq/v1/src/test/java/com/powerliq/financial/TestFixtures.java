package com.powerliq.financial;

import java.math.BigDecimal;
import java.time.Instant;
import java.time.LocalDate;
import java.util.UUID;

final class TestFixtures {
    static final UUID PORTFOLIO_ID = UUID.fromString("10000000-0000-0000-0000-000000000001");
    static final SettlementInterval INTERVAL = SettlementInterval.of(LocalDate.of(2026, 1, 15), 20);

    private TestFixtures() {
    }

    static Position position(PositionSide side, String contractPrice, String volume) {
        return new Position(
                UUID.randomUUID(),
                PORTFOLIO_ID,
                side,
                new PriceGbpPerMWh(new BigDecimal(contractPrice)),
                new EnergyMWh(new BigDecimal(volume)),
                INTERVAL,
                PositionStatus.ACTIVE,
                Instant.parse("2026-01-01T00:00:00Z"));
    }

    static MarketPrice marketPrice(String price) {
        return new MarketPrice(
                "APXMIDP",
                INTERVAL,
                new PriceGbpPerMWh(new BigDecimal(price)),
                Instant.parse("2026-01-15T10:05:00Z"),
                Instant.parse("2026-01-15T10:06:00Z"));
    }

    static MoneyGbp money(String amount) {
        return new MoneyGbp(new BigDecimal(amount));
    }
}
