package com.powerliq.portfolio;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.powerliq.api.contracts.ServiceException;
import java.math.BigDecimal;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

@SpringBootTest
@Testcontainers
class PortfolioServiceIntegrationTest {
    @Container static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine");
    @DynamicPropertySource static void database(DynamicPropertyRegistry registry) {
        registry.add("spring.datasource.url", POSTGRES::getJdbcUrl);
        registry.add("spring.datasource.username", POSTGRES::getUsername);
        registry.add("spring.datasource.password", POSTGRES::getPassword);
    }
    @Autowired PortfolioService service; @Autowired JdbcTemplate jdbc;
    @BeforeEach void clean() { jdbc.update("DELETE FROM outbox_events"); jdbc.update("DELETE FROM portfolios"); }

    @Test void createsAndUpdatesWithOptimisticConcurrencyAndAtomicEvents() {
        UUID id = UUID.randomUUID(); UUID correlation = UUID.randomUUID();
        Portfolio created = service.create(id, "Trading", new BigDecimal("1000"), new BigDecimal("100"), correlation);
        Portfolio updated = service.updateSettings(id, new BigDecimal("1200"), new BigDecimal("150"), 1, correlation);
        assertThat(created.configurationVersion()).isEqualTo(1);
        assertThat(updated.configurationVersion()).isEqualTo(2);
        assertThat(jdbc.queryForObject("SELECT count(*) FROM outbox_events", Long.class)).isEqualTo(2);
        assertThatThrownBy(() -> service.updateSettings(id, BigDecimal.TEN, BigDecimal.ONE, 1, correlation))
                .isInstanceOf(ServiceException.class)
                .extracting(e -> ((ServiceException)e).descriptor().code())
                .isEqualTo("CONFIGURATION_VERSION_CONFLICT");
        assertThat(jdbc.queryForObject("SELECT count(*) FROM outbox_events", Long.class)).isEqualTo(2);
    }

    @Test void rejectsAnUnsafeLiquidityBufferBeforeWriting() {
        assertThatThrownBy(() -> service.create(UUID.randomUUID(), "Unsafe", BigDecimal.TEN,
                new BigDecimal("11"), UUID.randomUUID())).isInstanceOf(ServiceException.class);
        assertThat(jdbc.queryForObject("SELECT count(*) FROM portfolios", Long.class)).isZero();
    }
}
