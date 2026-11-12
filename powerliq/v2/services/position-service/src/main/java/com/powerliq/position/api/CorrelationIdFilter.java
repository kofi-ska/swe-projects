package com.powerliq.position.api;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.util.UUID;
import org.springframework.stereotype.Component;
import org.springframework.web.filter.OncePerRequestFilter;

@Component
public class CorrelationIdFilter extends OncePerRequestFilter {
    static final String HEADER = "X-Correlation-ID";
    static final String ATTRIBUTE = CorrelationIdFilter.class.getName() + ".correlationId";

    @Override
    protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain chain)
            throws ServletException, IOException {
        UUID correlationId = parse(request.getHeader(HEADER));
        request.setAttribute(ATTRIBUTE, correlationId);
        response.setHeader(HEADER, correlationId.toString());
        chain.doFilter(request, response);
    }

    private UUID parse(String supplied) {
        if (supplied == null || supplied.isBlank()) return UUID.randomUUID();
        try {
            return UUID.fromString(supplied);
        } catch (IllegalArgumentException ignored) {
            return UUID.randomUUID();
        }
    }
}
