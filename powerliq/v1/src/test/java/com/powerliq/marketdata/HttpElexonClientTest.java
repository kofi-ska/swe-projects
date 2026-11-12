package com.powerliq.marketdata;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.powerliq.config.PowerLiqProperties;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.math.BigDecimal;
import java.net.InetSocketAddress;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.time.Instant;
import java.util.UUID;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;
import org.springframework.web.client.RestClient;

class HttpElexonClientTest {
    private HttpServer server;

    @AfterEach
    void stopServer() {
        if (server != null) server.stop(0);
    }

    @Test
    void parsesTheObservedElexonResponseShape() throws Exception {
        startServer(200, """
                {"metadata":{"datasets":["MID"]},"data":[
                  {"startTime":"2026-11-12T10:00:00Z","dataProvider":"APXMIDP",
                   "settlementDate":"2026-11-12","settlementPeriod":21,
                   "price":169.42,"volume":3702.650}
                ]}
                """);
        HttpElexonClient client = client(4096);

        var prices = client.fetchMarketIndexPrices(
                Instant.parse("2026-11-12T10:00:00Z"), Instant.parse("2026-11-12T10:30:00Z"));

        assertEquals(1, prices.size());
        assertEquals(new BigDecimal("169.42000"), prices.getFirst().price().value());
        assertEquals(21, prices.getFirst().settlementInterval().settlementPeriod());
    }

    @Test
    void rejectsResponseBeforeReadingBeyondTheConfiguredBound() throws Exception {
        startServer(200, "x".repeat(2048));
        HttpElexonClient client = client(1024);

        assertThrows(ElexonUnavailableException.class, () -> client.fetchMarketIndexPrices(
                Instant.parse("2026-11-12T10:00:00Z"), Instant.parse("2026-11-12T10:30:00Z")));
    }

    @Test
    void rejectsNonSuccessfulUpstreamStatus() throws Exception {
        startServer(503, "unavailable");
        HttpElexonClient client = client(4096);

        assertThrows(ElexonUnavailableException.class, () -> client.fetchMarketIndexPrices(
                Instant.parse("2026-11-12T10:00:00Z"), Instant.parse("2026-11-12T10:30:00Z")));
    }

    private HttpElexonClient client(int maxResponseBytes) {
        PowerLiqProperties properties = new PowerLiqProperties(
                new PowerLiqProperties.Portfolio(UUID.randomUUID(), BigDecimal.TEN, BigDecimal.ONE),
                new PowerLiqProperties.Elexon(
                        URI.create("http://localhost:" + server.getAddress().getPort()),
                        Duration.ofSeconds(1), Duration.ofSeconds(1), maxResponseBytes),
                new PowerLiqProperties.MarketData(Duration.ofMinutes(90)),
                new PowerLiqProperties.Security("test-admin-key"));
        return new HttpElexonClient(RestClient.builder(), properties, new ObjectMapper());
    }

    private void startServer(int status, String body) throws IOException {
        byte[] payload = body.getBytes(StandardCharsets.UTF_8);
        server = HttpServer.create(new InetSocketAddress("localhost", 0), 0);
        server.createContext("/balancing/pricing/market-index", exchange -> {
            exchange.sendResponseHeaders(status, payload.length);
            exchange.getResponseBody().write(payload);
            exchange.close();
        });
        server.start();
    }
}
