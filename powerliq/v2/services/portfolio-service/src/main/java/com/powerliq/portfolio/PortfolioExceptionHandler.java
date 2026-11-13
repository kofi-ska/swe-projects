package com.powerliq.portfolio;

import com.powerliq.api.contracts.ApiError;
import com.powerliq.api.contracts.ServiceException;
import java.time.Clock;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

@RestControllerAdvice
public class PortfolioExceptionHandler {
    private final Clock clock; public PortfolioExceptionHandler(Clock clock) { this.clock = clock; }
    @ExceptionHandler(ServiceException.class) ResponseEntity<ApiError> service(ServiceException e) {
        return ResponseEntity.status(e.descriptor().httpStatus()).body(error(e.descriptor().code(), e.getMessage(), e.descriptor().retryable()));
    }
    @ExceptionHandler(MethodArgumentNotValidException.class) ResponseEntity<ApiError> validation() {
        return ResponseEntity.badRequest().body(error("PORTFOLIO_INVALID", "Request validation failed", false));
    }
    @ExceptionHandler(Exception.class) ResponseEntity<ApiError> unexpected() {
        return ResponseEntity.internalServerError().body(error("INTERNAL_ERROR", "An unexpected error occurred", false));
    }
    private ApiError error(String code, String message, boolean retryable) {
        return new ApiError(code, message, "portfolio-service", UUID.randomUUID().toString(), Instant.now(clock), retryable, List.of());
    }
}
