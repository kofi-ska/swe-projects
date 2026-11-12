package com.powerliq.security;

import jakarta.servlet.http.HttpServletResponse;
import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.http.HttpMethod;
import org.springframework.security.config.Customizer;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.config.http.SessionCreationPolicy;
import org.springframework.security.web.SecurityFilterChain;

@AutoConfiguration
public class PowerLiqSecurityAutoConfiguration {
    @Bean
    @ConditionalOnProperty(name = "powerliq.security.enabled", havingValue = "true", matchIfMissing = true)
    SecurityFilterChain secured(HttpSecurity http) throws Exception {
        return common(http)
                .authorizeHttpRequests(auth -> auth
                        .requestMatchers("/actuator/health", "/actuator/health/**").permitAll()
                        .requestMatchers("/internal/**").hasAuthority("SCOPE_powerliq.internal")
                        .requestMatchers(HttpMethod.GET, "/api/**").hasAuthority("SCOPE_powerliq.read")
                        .requestMatchers("/api/**").hasAuthority("SCOPE_powerliq.write")
                        .anyRequest().denyAll())
                .oauth2ResourceServer(oauth -> oauth.jwt(Customizer.withDefaults()))
                .exceptionHandling(errors -> errors
                        .authenticationEntryPoint((request, response, exception) -> error(response, 401, "AUTHENTICATION_REQUIRED"))
                        .accessDeniedHandler((request, response, exception) -> error(response, 403, "AUTHORIZATION_DENIED")))
                .build();
    }

    @Bean
    @ConditionalOnProperty(name = "powerliq.security.enabled", havingValue = "false")
    SecurityFilterChain localOnly(HttpSecurity http) throws Exception {
        return common(http).authorizeHttpRequests(auth -> auth.anyRequest().permitAll()).build();
    }

    private HttpSecurity common(HttpSecurity http) throws Exception {
        return http.csrf(csrf -> csrf.disable())
                .sessionManagement(session -> session.sessionCreationPolicy(SessionCreationPolicy.STATELESS))
                .requestCache(cache -> cache.disable())
                .securityContext(context -> context.requireExplicitSave(false));
    }

    private static void error(HttpServletResponse response, int status, String code) throws java.io.IOException {
        response.setStatus(status);
        response.setContentType("application/json");
        response.getWriter().printf("{\"code\":\"%s\",\"message\":\"Access denied\",\"retryable\":false}", code);
    }
}
