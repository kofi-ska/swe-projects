package com.powerliq.marketdata;

import com.fasterxml.jackson.databind.JsonNode;
import com.powerliq.config.PowerLiqProperties;
import com.powerliq.financial.MarketPrice;
import com.powerliq.financial.PriceGbpPerMWh;
import com.powerliq.financial.SettlementInterval;
import java.math.BigDecimal;
import java.net.http.HttpClient;
import java.net.http.HttpTimeoutException;
import java.net.SocketTimeoutException;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.LocalDate;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.CancellationException;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.http.client.JdkClientHttpRequestFactory;
import org.springframework.stereotype.Component;
import org.springframework.web.client.RestClient;

@Component
public class HttpElexonClient implements ElexonClient {
    private final RestClient restClient;
    private final Clock clock;
    private final ObjectMapper objectMapper;
    private final int maxResponseBytes;

    public HttpElexonClient(
            RestClient.Builder builder, PowerLiqProperties properties, ObjectMapper objectMapper) {
        HttpClient httpClient = HttpClient.newBuilder()
                .connectTimeout(properties.elexon().connectTimeout())
                .build();
        JdkClientHttpRequestFactory requestFactory = new JdkClientHttpRequestFactory(httpClient);
        requestFactory.setReadTimeout(properties.elexon().readTimeout());
        this.restClient = builder.baseUrl(properties.elexon().baseUrl().toString())
                .requestFactory(requestFactory)
                .build();
        this.clock = Clock.systemUTC();
        this.objectMapper = objectMapper;
        this.maxResponseBytes = properties.elexon().maxResponseBytes();
    }

    @Override
    public List<MarketPrice> fetchMarketIndexPrices(Instant from, Instant to) {
        if (from == null || to == null || !from.isBefore(to)) {
            throw new IllegalArgumentException("from must be before to");
        }
        if (Duration.between(from, to).compareTo(Duration.ofDays(31)) > 0) {
            throw new IllegalArgumentException("Elexon query window must not exceed 31 days");
        }
        if (Thread.currentThread().isInterrupted()) throw new CancellationException("request was cancelled");
        try {
            JsonNode body = restClient.get()
                    .uri(uri -> uri.path("/balancing/pricing/market-index")
                            .queryParam("from", from)
                            .queryParam("to", to)
                            .queryParam("dataProviders", "APXMIDP")
                            .build())
                    .exchange((request, response) -> {
                        if (response.getStatusCode().isError()) {
                            throw new IllegalStateException("Elexon returned HTTP " + response.getStatusCode().value());
                        }
                        byte[] payload = response.getBody().readNBytes(maxResponseBytes + 1);
                        if (payload.length > maxResponseBytes) {
                            throw new IllegalArgumentException("Elexon response exceeds the configured size limit");
                        }
                        return objectMapper.readTree(payload);
                    });
            return parse(body, Instant.now(clock));
        } catch (CancellationException exception) {
            throw exception;
        } catch (RuntimeException exception) {
            if (hasCause(exception, HttpTimeoutException.class) || hasCause(exception, SocketTimeoutException.class)) {
                throw new ElexonTimeoutException("Elexon Market Index request timed out", exception);
            }
            throw new ElexonUnavailableException("Elexon Market Index request failed", exception);
        }
    }

    private List<MarketPrice> parse(JsonNode body, Instant retrievedAt) {
        if (body == null) return List.of();
        JsonNode rows = body.isArray() ? body : body.path("data");
        if (!rows.isArray()) throw new IllegalArgumentException("unexpected Elexon response shape");

        List<MarketPrice> prices = new ArrayList<>();
        for (JsonNode row : rows) {
            SettlementInterval interval = interval(row);
            JsonNode priceNode = row.get("price");
            if (priceNode == null || !priceNode.isNumber()) {
                throw new IllegalArgumentException("Elexon row has no numeric price");
            }
            String provider = row.path("dataProvider").asText("APXMIDP");
            Instant sourceTimestamp = optionalInstant(row, "publishTime", "sourceTimestamp");
            prices.add(new MarketPrice(provider, interval,
                    new PriceGbpPerMWh(new BigDecimal(priceNode.asText())), sourceTimestamp, retrievedAt));
        }
        return List.copyOf(prices);
    }

    private SettlementInterval interval(JsonNode row) {
        JsonNode startNode = row.get("startTime");
        if (startNode != null && startNode.isTextual()) {
            return SettlementInterval.fromDeliveryStart(Instant.parse(startNode.asText()));
        }
        JsonNode dateNode = row.get("settlementDate");
        JsonNode periodNode = row.get("settlementPeriod");
        if (dateNode != null && periodNode != null) {
            return SettlementInterval.of(LocalDate.parse(dateNode.asText()), periodNode.asInt());
        }
        throw new IllegalArgumentException("Elexon row has no settlement interval");
    }

    private Instant optionalInstant(JsonNode row, String... names) {
        for (String name : names) {
            JsonNode value = row.get(name);
            if (value != null && value.isTextual() && !value.asText().isBlank()) {
                return Instant.parse(value.asText());
            }
        }
        return null;
    }

    private boolean hasCause(Throwable throwable, Class<? extends Throwable> type) {
        Throwable current = throwable;
        while (current != null) {
            if (type.isInstance(current)) return true;
            current = current.getCause();
        }
        return false;
    }
}
