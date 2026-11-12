package com.powerliq;

import com.powerliq.config.PowerLiqProperties;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.context.properties.EnableConfigurationProperties;

@SpringBootApplication
@EnableConfigurationProperties(PowerLiqProperties.class)
public class PowerLiqApplication {
    public static void main(String[] args) {
        SpringApplication.run(PowerLiqApplication.class, args);
    }
}
