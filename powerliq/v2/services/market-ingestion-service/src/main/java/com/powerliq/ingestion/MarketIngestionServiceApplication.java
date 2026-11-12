package com.powerliq.ingestion;

import java.net.http.HttpClient;
import java.time.Clock;
import java.time.Duration;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.annotation.Bean;
import org.springframework.scheduling.annotation.EnableScheduling;

@SpringBootApplication
@EnableScheduling
public class MarketIngestionServiceApplication {
    public static void main(String[] args) {
        SpringApplication.run(MarketIngestionServiceApplication.class, args);
    }
    @Bean Clock clock() { return Clock.systemUTC(); }
    @Bean HttpClient httpClient() { return HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(3)).build(); }
}
