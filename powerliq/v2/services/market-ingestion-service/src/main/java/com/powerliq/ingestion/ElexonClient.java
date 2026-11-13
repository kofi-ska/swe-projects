package com.powerliq.ingestion;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.math.BigDecimal;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

@Component
public class ElexonClient {
    public record Price(String provider, Instant start, Instant end, BigDecimal value, String revision, Instant sourceTime) { }
    private final HttpClient http; private final ObjectMapper json; private final String baseUrl; private final Duration timeout;
    public ElexonClient(HttpClient http, ObjectMapper json,
            @Value("${powerliq.elexon.base-url:https://data.elexon.co.uk/bmrs/api/v1}") String baseUrl,
            @Value("${powerliq.elexon.request-timeout:10s}") Duration timeout) {
        this.http = http; this.json = json; this.baseUrl = baseUrl; this.timeout = timeout;
    }
    public List<Price> fetch(Instant from, Instant to) throws Exception {
        String query = "from=" + encode(from.toString()) + "&to=" + encode(to.toString()) + "&format=json";
        HttpRequest request = HttpRequest.newBuilder(URI.create(baseUrl + "/balancing/pricing/market-index?" + query))
                .timeout(timeout).header("Accept", "application/json").GET().build();
        HttpResponse<String> response = http.send(request, HttpResponse.BodyHandlers.ofString());
        if (response.statusCode() == 429 || response.statusCode() >= 500) throw new TransientSourceException("Elexon returned " + response.statusCode());
        if (response.statusCode() < 200 || response.statusCode() >= 300) throw new IllegalStateException("Elexon rejected request with " + response.statusCode());
        JsonNode data = json.readTree(response.body()).path("data"); List<Price> result = new ArrayList<>();
        if (!data.isArray()) throw new IllegalStateException("Elexon response has no data array");
        for (JsonNode row : data) {
            Instant start = Instant.parse(row.required("startTime").asText());
            result.add(new Price(row.path("dataProvider").asText("ELEXON"), start, start.plus(Duration.ofMinutes(30)),
                    row.required("price").decimalValue(), row.path("revisionNumber").asText("source"),
                    row.hasNonNull("publishTime") ? Instant.parse(row.get("publishTime").asText()) : start));
        }
        return List.copyOf(result);
    }
    private String encode(String value) { return URLEncoder.encode(value, StandardCharsets.UTF_8); }
    public static class TransientSourceException extends RuntimeException { public TransientSourceException(String message) { super(message); } }
}
