package com.powerliq.application;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.powerliq.config.PowerLiqProperties;
import com.powerliq.financial.EnergyMWh;
import com.powerliq.financial.MarketPrice;
import com.powerliq.financial.Position;
import com.powerliq.financial.PositionSide;
import com.powerliq.financial.PositionStatus;
import com.powerliq.financial.PriceGbpPerMWh;
import com.powerliq.financial.SettlementInterval;
import com.powerliq.marketdata.ElexonClient;
import java.math.BigDecimal;
import java.net.URI;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.LocalDate;
import java.time.ZoneOffset;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.Test;

class MarketDataServiceTest {
    private static final Instant NOW = Instant.parse("2026-11-12T12:00:00Z");
    private final ElexonClient client = mock(ElexonClient.class);
    private final PositionStore positions = new PositionStore();
    private final MarketPriceStore prices = new MarketPriceStore();
    private final MarketDataService service = new MarketDataService(
            client, positions, prices, properties(), Clock.fixed(NOW, ZoneOffset.UTC));

    @Test
    void repeatedStatusChecksShareOneBoundedUpstreamCall() {
        SettlementInterval interval = SettlementInterval.fromDeliveryStart(NOW.minus(Duration.ofMinutes(30)));
        when(client.fetchMarketIndexPrices(any(), any())).thenReturn(List.of(price(interval)));

        service.check();
        service.check();

        verify(client, times(1)).fetchMarketIndexPrices(any(), any());
    }

    @Test
    void syncRetainsOnlyIntervalsRequiredByActivePositions() {
        SettlementInterval required = SettlementInterval.of(LocalDate.of(2026, 11, 12), 20);
        SettlementInterval unrelated = SettlementInterval.of(LocalDate.of(2026, 11, 12), 21);
        positions.save(position(required));
        when(client.fetchMarketIndexPrices(any(), any())).thenReturn(List.of(price(required), price(unrelated)));

        assertEquals(1, service.syncActivePositions());
        assertEquals(1, prices.snapshot().size());
        assertEquals(true, prices.snapshot().containsKey(required));
    }

    @Test
    void syncRejectsAnUnboundedDateRangeBeforeCallingElexon() {
        positions.save(position(SettlementInterval.of(LocalDate.of(2026, 1, 1), 1)));
        positions.save(position(SettlementInterval.of(LocalDate.of(2026, 3, 1), 1)));

        assertThrows(ResourceLimitExceededException.class, service::syncActivePositions);
        verify(client, times(0)).fetchMarketIndexPrices(any(), any());
    }

    private Position position(SettlementInterval interval) {
        return new Position(UUID.randomUUID(), properties().portfolio().id(), PositionSide.LONG,
                new PriceGbpPerMWh(new BigDecimal("100")), new EnergyMWh(BigDecimal.ONE), interval,
                PositionStatus.ACTIVE, NOW);
    }

    private MarketPrice price(SettlementInterval interval) {
        return new MarketPrice("APXMIDP", interval, new PriceGbpPerMWh(new BigDecimal("110")), null, NOW);
    }

    private PowerLiqProperties properties() {
        return new PowerLiqProperties(
                new PowerLiqProperties.Portfolio(
                        UUID.fromString("10000000-0000-0000-0000-000000000001"),
                        new BigDecimal("250000"), new BigDecimal("100000")),
                new PowerLiqProperties.Elexon(
                        URI.create("https://example.test"), Duration.ofSeconds(1), Duration.ofSeconds(1), 4096),
                new PowerLiqProperties.MarketData(Duration.ofMinutes(90)),
                new PowerLiqProperties.Security("test-admin-key"));
    }
}
