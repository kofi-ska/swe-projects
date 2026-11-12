package com.powerliq.valuation;

import com.powerliq.financial.ValuationEngine;
import java.time.Clock;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.annotation.Bean;

@SpringBootApplication
public class ValuationServiceApplication {
    public static void main(String[] args) {
        SpringApplication.run(ValuationServiceApplication.class, args);
    }
    @Bean Clock clock(){return Clock.systemUTC();}
    @Bean ValuationEngine valuationEngine(){return new ValuationEngine();}
}
