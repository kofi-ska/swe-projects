package com.powerliq.config;

import jakarta.validation.Valid;
import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.Max;
import java.math.BigDecimal;
import java.net.URI;
import java.time.Duration;
import java.util.UUID;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.validation.annotation.Validated;

@Validated
@ConfigurationProperties(prefix = "powerliq")
public record PowerLiqProperties(
        @Valid Portfolio portfolio,
        @Valid Elexon elexon,
        @Valid MarketData marketData,
        @Valid Security security) {
    public PowerLiqProperties {
        if (portfolio == null) portfolio = Portfolio.defaults();
        if (elexon == null) elexon = Elexon.defaults();
        if (marketData == null) marketData = MarketData.defaults();
        if (security == null) security = Security.defaults();
    }

    public record Portfolio(
            @NotNull UUID id,
            @NotNull @DecimalMin("0.00") BigDecimal availableCashGbp,
            @NotNull @DecimalMin("0.00") BigDecimal liquidityBufferGbp) {
        static Portfolio defaults() {
            return new Portfolio(
                    UUID.fromString("10000000-0000-0000-0000-000000000001"),
                    new BigDecimal("250000.00"), new BigDecimal("100000.00"));
        }
    }

    public record Elexon(
            @NotNull URI baseUrl,
            @NotNull Duration connectTimeout,
            @NotNull Duration readTimeout,
            @Min(1024) @Max(52428800) int maxResponseBytes) {
        static Elexon defaults() {
            return new Elexon(URI.create("https://data.elexon.co.uk/bmrs/api/v1"),
                    Duration.ofSeconds(3), Duration.ofSeconds(10), 5_242_880);
        }
    }

    public record MarketData(@NotNull Duration maxAge) {
        static MarketData defaults() {
            return new MarketData(Duration.ofMinutes(90));
        }
    }

    public record Security(@NotBlank String adminKey) {
        static Security defaults() {
            return new Security("local-dev-key");
        }
    }
}
