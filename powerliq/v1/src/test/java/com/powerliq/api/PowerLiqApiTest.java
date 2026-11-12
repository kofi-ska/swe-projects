package com.powerliq.api;

import static org.hamcrest.Matchers.is;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.when;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.times;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.header;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.powerliq.financial.MarketPrice;
import com.powerliq.financial.PriceGbpPerMWh;
import com.powerliq.financial.SettlementInterval;
import com.powerliq.marketdata.ElexonClient;
import java.math.BigDecimal;
import java.time.Instant;
import java.util.List;
import java.util.Set;
import java.util.concurrent.Callable;
import java.util.concurrent.Executors;
import java.util.stream.Collectors;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

@SpringBootTest
@AutoConfigureMockMvc
@Testcontainers
class PowerLiqApiTest {
    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:17-alpine");

    @DynamicPropertySource
    static void databaseProperties(DynamicPropertyRegistry registry) {
        registry.add("spring.datasource.url", POSTGRES::getJdbcUrl);
        registry.add("spring.datasource.username", POSTGRES::getUsername);
        registry.add("spring.datasource.password", POSTGRES::getPassword);
    }
    private static final String PORTFOLIO_ID = "10000000-0000-0000-0000-000000000001";
    private static final String POSITION_JSON = """
            {
              "portfolioId": "%s",
              "side": "LONG",
              "contractPriceGbpPerMWh": 80.00,
              "volumeMWh": 100.000,
              "settlementDate": "2026-11-12",
              "settlementPeriod": 20
            }
            """.formatted(PORTFOLIO_ID);

    @Autowired
    private MockMvc mvc;

    @MockitoBean
    private ElexonClient elexonClient;

    @Test
    void positionCreationIsIdempotentAndConflictSafe() throws Exception {
        String first = mvc.perform(post("/api/v1/positions")
                        .header("Idempotency-Key", "api-test-key")
                        .contentType("application/json").content(POSITION_JSON))
                .andExpect(status().isCreated())
                .andExpect(header().string("Location", org.hamcrest.Matchers.startsWith("/api/v1/positions/")))
                .andExpect(jsonPath("$.status", is("ACTIVE")))
                .andReturn().getResponse().getContentAsString();

        mvc.perform(post("/api/v1/positions")
                        .header("Idempotency-Key", "api-test-key")
                        .contentType("application/json").content(POSITION_JSON))
                .andExpect(status().isCreated())
                .andExpect(jsonPath("$.id", is(new com.fasterxml.jackson.databind.ObjectMapper()
                        .readTree(first).path("id").asText())));

        mvc.perform(post("/api/v1/positions")
                        .header("Idempotency-Key", "api-test-key")
                        .contentType("application/json")
                        .content(POSITION_JSON.replace("LONG", "SHORT")))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code", is("CONFLICT")));
    }

    @Test
    void concurrentIdempotentCreatesCommitExactlyOnePosition() throws Exception {
        List<Callable<String>> attempts = java.util.stream.IntStream.range(0, 20)
                .mapToObj(ignored -> (Callable<String>) () -> mvc.perform(post("/api/v1/positions")
                                .header("Idempotency-Key", "concurrent-api-key")
                                .contentType("application/json").content(POSITION_JSON))
                        .andExpect(status().isCreated())
                        .andReturn().getResponse().getContentAsString())
                .toList();

        Set<String> ids;
        try (var executor = Executors.newVirtualThreadPerTaskExecutor()) {
            ids = executor.invokeAll(attempts).stream().map(future -> {
                try {
                    return new com.fasterxml.jackson.databind.ObjectMapper()
                            .readTree(future.get()).path("id").asText();
                } catch (Exception exception) {
                    throw new AssertionError(exception);
                }
            }).collect(Collectors.toSet());
        }

        assertEquals(1, ids.size());
    }

    @Test
    void administrativeWritesRequireAdminKey() throws Exception {
        mvc.perform(post("/api/v1/market-data/sync"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code", is("UNAUTHORIZED")));
    }

    @Test
    void elexonStatusReportsSuccessfulCheck() throws Exception {
        Instant now = Instant.now();
        Instant currentHalfHour = Instant.ofEpochSecond((now.getEpochSecond() / 1800) * 1800);
        SettlementInterval interval = SettlementInterval.fromDeliveryStart(currentHalfHour);
        when(elexonClient.fetchMarketIndexPrices(any(), any())).thenReturn(List.of(
                new MarketPrice("APXMIDP", interval, new PriceGbpPerMWh(new BigDecimal("95")),
                        now, now)));

        mvc.perform(get("/api/v1/market-data/status"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.status", is("AVAILABLE")))
                .andExpect(jsonPath("$.recordsReceived", is(1)));

        mvc.perform(get("/api/v1/market-data/status"))
                .andExpect(status().isOk());
        verify(elexonClient, times(1)).fetchMarketIndexPrices(any(), any());
    }

    @Test
    void missingMarketPriceProducesUnprocessableValuation() throws Exception {
        mvc.perform(post("/api/v1/positions")
                        .header("Idempotency-Key", "missing-price-key")
                        .contentType("application/json").content(POSITION_JSON))
                .andExpect(status().isCreated());

        mvc.perform(get("/api/v1/portfolios/{id}/liquidity", PORTFOLIO_ID))
                .andExpect(status().isUnprocessableEntity())
                .andExpect(jsonPath("$.code", is("VALUATION_INCOMPLETE")));
    }
}
