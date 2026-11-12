package com.powerliq.position.api;

import com.powerliq.api.contracts.ApiError;
import com.powerliq.api.contracts.ErrorDescriptor;
import com.powerliq.api.contracts.FieldViolation;
import com.powerliq.api.contracts.ServiceException;
import jakarta.servlet.http.HttpServletRequest;
import java.time.Clock;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.dao.DataAccessException;
import org.springframework.http.ResponseEntity;
import org.springframework.validation.FieldError;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.http.converter.HttpMessageNotReadableException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

@RestControllerAdvice
public class PositionExceptionHandler {
    private static final Logger LOG = LoggerFactory.getLogger(PositionExceptionHandler.class);
    private static final String SERVICE = "position-service";
    private final Clock clock;

    public PositionExceptionHandler(Clock clock) {
        this.clock = clock;
    }

    @ExceptionHandler(ServiceException.class)
    ResponseEntity<ApiError> serviceError(ServiceException exception, HttpServletRequest request) {
        return response(exception.descriptor(), exception.getMessage(), List.of(), request);
    }

    @ExceptionHandler(MethodArgumentNotValidException.class)
    ResponseEntity<ApiError> validationError(MethodArgumentNotValidException exception, HttpServletRequest request) {
        List<FieldViolation> details = exception.getBindingResult().getFieldErrors().stream()
                .map(this::violation)
                .toList();
        return response(com.powerliq.position.application.PositionErrorCode.POSITION_INVALID,
                "Request validation failed", details, request);
    }

    @ExceptionHandler(HttpMessageNotReadableException.class)
    ResponseEntity<ApiError> malformedRequest(HttpMessageNotReadableException exception, HttpServletRequest request) {
        return response(com.powerliq.position.application.PositionErrorCode.POSITION_INVALID,
                "Request body is malformed or contains an unsupported value", List.of(), request);
    }

    @ExceptionHandler(DataAccessException.class)
    ResponseEntity<ApiError> persistenceError(DataAccessException exception, HttpServletRequest request) {
        LOG.error("Position persistence failure; correlationId={}", correlationId(request), exception);
        return response(com.powerliq.position.application.PositionErrorCode.POSITION_PERSISTENCE_FAILED,
                "Position data is temporarily unavailable", List.of(), request);
    }

    @ExceptionHandler(Exception.class)
    ResponseEntity<ApiError> unexpectedError(Exception exception, HttpServletRequest request) {
        LOG.error("Unhandled position-service failure; correlationId={}", correlationId(request), exception);
        ErrorDescriptor descriptor = new ErrorDescriptor() {
            public String code() { return "INTERNAL_ERROR"; }
            public int httpStatus() { return 500; }
            public boolean retryable() { return false; }
        };
        return response(descriptor, "An unexpected error occurred", List.of(), request);
    }

    private ResponseEntity<ApiError> response(
            ErrorDescriptor descriptor, String message, List<FieldViolation> details, HttpServletRequest request) {
        ApiError error = new ApiError(descriptor.code(), message, SERVICE, correlationId(request).toString(),
                Instant.now(clock), descriptor.retryable(), details);
        return ResponseEntity.status(descriptor.httpStatus()).body(error);
    }

    private FieldViolation violation(FieldError error) {
        return new FieldViolation(error.getField(), error.getDefaultMessage());
    }

    private UUID correlationId(HttpServletRequest request) {
        Object value = request.getAttribute(CorrelationIdFilter.ATTRIBUTE);
        return value instanceof UUID id ? id : UUID.randomUUID();
    }
}
