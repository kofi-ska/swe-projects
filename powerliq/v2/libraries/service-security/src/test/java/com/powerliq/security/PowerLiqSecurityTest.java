package com.powerliq.security;

import static org.springframework.security.test.web.servlet.request.SecurityMockMvcRequestPostProcessors.jwt;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import org.junit.jupiter.api.Test;
import org.springframework.boot.SpringBootConfiguration;
import org.springframework.boot.autoconfigure.EnableAutoConfiguration;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.context.annotation.Bean;
import org.springframework.security.oauth2.jwt.JwtDecoder;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RestController;

@SpringBootTest(properties = {"powerliq.security.enabled=true", "spring.security.oauth2.resourceserver.jwt.jwk-set-uri=https://invalid.test/jwks"})
@AutoConfigureMockMvc
class PowerLiqSecurityTest {
    @Autowired MockMvc mvc;

    @Test void requiresAuthentication() throws Exception {
        mvc.perform(get("/api/v2/resource")).andExpect(status().isUnauthorized());
    }
    @Test void separatesReadWriteAndInternalScopes() throws Exception {
        mvc.perform(get("/api/v2/resource").with(jwt().authorities(() -> "SCOPE_powerliq/read"))).andExpect(status().isOk());
        mvc.perform(post("/api/v2/resource").with(jwt().authorities(() -> "SCOPE_powerliq/read"))).andExpect(status().isForbidden());
        mvc.perform(post("/api/v2/resource").with(jwt().authorities(() -> "SCOPE_powerliq/write"))).andExpect(status().isOk());
        mvc.perform(post("/internal/v2/event").with(jwt().authorities(() -> "SCOPE_powerliq/write"))).andExpect(status().isForbidden());
        mvc.perform(post("/internal/v2/event").with(jwt().authorities(() -> "SCOPE_powerliq/internal"))).andExpect(status().isOk());
    }
    @Test void exposesHealthWithoutCredentials() throws Exception {
        // This isolated library test has no Actuator endpoint; 404 proves the request passed security.
        mvc.perform(get("/actuator/health")).andExpect(status().isNotFound());
    }

    @SpringBootConfiguration
    @EnableAutoConfiguration
    static class TestApplication {
        @Bean TestController controller(){return new TestController();}
    }
    @RestController static class TestController {
        @GetMapping("/api/v2/resource") String read(){return "ok";}
        @PostMapping("/api/v2/resource") String write(){return "ok";}
        @PostMapping("/internal/v2/event") String internal(){return "ok";}
    }
}
