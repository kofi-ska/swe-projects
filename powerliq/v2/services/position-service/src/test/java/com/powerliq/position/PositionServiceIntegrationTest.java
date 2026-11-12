package com.powerliq.position;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.header;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.time.LocalDate;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.Callable;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

@SpringBootTest
@AutoConfigureMockMvc
@Testcontainers
class PositionServiceIntegrationTest {
    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine");

    @DynamicPropertySource
    static void database(DynamicPropertyRegistry registry) {
        registry.add("spring.datasource.url", POSTGRES::getJdbcUrl);
        registry.add("spring.datasource.username", POSTGRES::getUsername);
        registry.add("spring.datasource.password", POSTGRES::getPassword);
    }

    @Autowired MockMvc mvc;
    @Autowired ObjectMapper json;
    @Autowired JdbcTemplate jdbc;

    @BeforeEach
    void cleanDatabase() {
        jdbc.update("DELETE FROM outbox_events");
        jdbc.update("DELETE FROM idempotency_records");
        jdbc.update("DELETE FROM positions");
    }

    @Test
    void createsOnceAndReturnsSamePositionForAnIdenticalRetry() throws Exception {
        String key = "create-once";
        MvcResult first = create(key, request("10.500"), 201);
        MvcResult retry = create(key, request("10.5"), 201);

        assertThat(id(first)).isEqualTo(id(retry));
        assertThat(jdbc.queryForObject("SELECT count(*) FROM positions", Long.class)).isEqualTo(1);
        assertThat(jdbc.queryForObject("SELECT count(*) FROM outbox_events", Long.class)).isEqualTo(1);
    }

    @Test
    void rejectsReusingAKeyForDifferentBusinessData() throws Exception {
        create("conflict", request("10"), 201);
        mvc.perform(post("/api/v2/positions").header("Idempotency-Key", "conflict")
                        .contentType("application/json").content(request("11")))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code").value("IDEMPOTENCY_CONFLICT"))
                .andExpect(jsonPath("$.retryable").value(false));
    }

    @Test
    void concurrentIdenticalRequestsCreateOnePositionAndOneEvent() throws Exception {
        try (var executor = Executors.newFixedThreadPool(8)) {
            List<Callable<MvcResult>> calls = new ArrayList<>();
            for (int index = 0; index < 16; index++) {
                calls.add(() -> create("concurrent-key", request("50"), 201));
            }
            List<Future<MvcResult>> results = executor.invokeAll(calls);
            List<UUID> ids = new ArrayList<>();
            for (Future<MvcResult> result : results) ids.add(id(result.get()));
            assertThat(ids).containsOnly(ids.getFirst());
        }
        assertThat(jdbc.queryForObject("SELECT count(*) FROM positions", Long.class)).isEqualTo(1);
        assertThat(jdbc.queryForObject("SELECT count(*) FROM outbox_events", Long.class)).isEqualTo(1);
    }

    @Test
    void settlementIsGuardedAndEmitsExactlyOneEvent() throws Exception {
        UUID positionId = id(create("settle", request("75"), 201));
        mvc.perform(post("/api/v2/positions/{id}/settlement", positionId))
                .andExpect(status().isOk()).andExpect(jsonPath("$.status").value("SETTLED"));
        mvc.perform(post("/api/v2/positions/{id}/settlement", positionId))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code").value("POSITION_ALREADY_SETTLED"));
        assertThat(jdbc.queryForObject("SELECT count(*) FROM outbox_events", Long.class)).isEqualTo(2);
    }

    @Test
    void errorsAreSanitizedStructuredAndCorrelated() throws Exception {
        String correlationId = UUID.randomUUID().toString();
        mvc.perform(post("/api/v2/positions").header("X-Correlation-ID", correlationId)
                        .contentType("application/json").content(request("10")))
                .andExpect(status().isBadRequest())
                .andExpect(header().string("X-Correlation-ID", correlationId))
                .andExpect(jsonPath("$.code").value("IDEMPOTENCY_KEY_REQUIRED"))
                .andExpect(jsonPath("$.service").value("position-service"))
                .andExpect(jsonPath("$.correlationId").value(correlationId));

        mvc.perform(get("/api/v2/positions/{id}", UUID.randomUUID()))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.code").value("POSITION_NOT_FOUND"));
    }

    private MvcResult create(String key, String body, int expectedStatus) throws Exception {
        return mvc.perform(post("/api/v2/positions").header("Idempotency-Key", key)
                        .contentType("application/json").content(body))
                .andExpect(status().is(expectedStatus)).andReturn();
    }

    private UUID id(MvcResult result) throws Exception {
        JsonNode body = json.readTree(result.getResponse().getContentAsByteArray());
        return UUID.fromString(body.required("id").asText());
    }

    private String request(String price) {
        return """
                {"portfolioId":"%s","side":"LONG","contractPriceGbpPerMWh":%s,
                 "volumeMWh":12.5,"settlementDate":"%s","settlementPeriod":1}
                """.formatted(UUID.fromString("00000000-0000-0000-0000-000000000001"), price,
                LocalDate.now().plusDays(1));
    }
}
