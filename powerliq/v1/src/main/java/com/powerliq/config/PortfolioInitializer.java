package com.powerliq.config;

import org.springframework.boot.ApplicationArguments;
import org.springframework.boot.ApplicationRunner;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Component;

@Component
public class PortfolioInitializer implements ApplicationRunner {
    private final JdbcTemplate jdbc;
    private final PowerLiqProperties properties;

    public PortfolioInitializer(JdbcTemplate jdbc, PowerLiqProperties properties) {
        this.jdbc = jdbc;
        this.properties = properties;
    }

    @Override
    public void run(ApplicationArguments args) {
        PowerLiqProperties.Portfolio portfolio = properties.portfolio();
        jdbc.update("""
                INSERT INTO portfolios (id, currency, available_cash, liquidity_buffer)
                VALUES (?, 'GBP', ?, ?)
                ON CONFLICT (id) DO UPDATE SET
                  available_cash = EXCLUDED.available_cash,
                  liquidity_buffer = EXCLUDED.liquidity_buffer
                """, portfolio.id(), portfolio.availableCashGbp(), portfolio.liquidityBufferGbp());
    }
}
