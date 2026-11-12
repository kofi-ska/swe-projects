package com.powerliq.api;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.powerliq.config.PowerLiqProperties;
import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Instant;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import org.springframework.web.filter.OncePerRequestFilter;

@Component
public class AdminEndpointFilter extends OncePerRequestFilter {
    private static final String ADMIN_HEADER = "X-Admin-Key";
    private final byte[] expectedKey;
    private final ObjectMapper objectMapper;

    public AdminEndpointFilter(PowerLiqProperties properties, ObjectMapper objectMapper) {
        this.expectedKey = properties.security().adminKey().getBytes(StandardCharsets.UTF_8);
        this.objectMapper = objectMapper;
    }

    @Override
    protected void doFilterInternal(
            HttpServletRequest request, HttpServletResponse response, FilterChain chain)
            throws ServletException, IOException {
        if (!isAdministrativeWrite(request)) {
            chain.doFilter(request, response);
            return;
        }
        String supplied = request.getHeader(ADMIN_HEADER);
        boolean matches = supplied != null && MessageDigest.isEqual(
                expectedKey, supplied.getBytes(StandardCharsets.UTF_8));
        if (!matches) {
            response.setStatus(HttpServletResponse.SC_UNAUTHORIZED);
            response.setContentType(MediaType.APPLICATION_JSON_VALUE);
            objectMapper.writeValue(response.getOutputStream(),
                    new ApiError("UNAUTHORIZED", "valid X-Admin-Key header required", Instant.now()));
            return;
        }
        chain.doFilter(request, response);
    }

    private boolean isAdministrativeWrite(HttpServletRequest request) {
        if (!"POST".equals(request.getMethod())) return false;
        String path = request.getRequestURI();
        return path.equals(ApiRoutes.V1_MARKET_DATA + "/sync") || path.endsWith("/settlement");
    }
}
