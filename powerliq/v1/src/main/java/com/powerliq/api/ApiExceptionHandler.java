package com.powerliq.api;

import com.powerliq.application.IdempotencyConflictException;
import com.powerliq.application.InvalidPositionTransitionException;
import com.powerliq.application.PortfolioNotFoundException;
import com.powerliq.application.PositionNotFoundException;
import com.powerliq.application.ResourceLimitExceededException;
import com.powerliq.financial.IncompleteValuationException;
import com.powerliq.marketdata.ElexonUnavailableException;
import com.powerliq.marketdata.ElexonTimeoutException;
import java.time.Instant;
import java.util.concurrent.CancellationException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.core.NestedExceptionUtils;
import org.springframework.dao.DataAccessException;
import org.springframework.http.HttpStatusCode;
import org.springframework.http.converter.HttpMessageNotReadableException;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.ServletRequestBindingException;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;
import org.springframework.web.context.request.async.AsyncRequestNotUsableException;
import org.springframework.web.method.annotation.MethodArgumentTypeMismatchException;

@RestControllerAdvice
public class ApiExceptionHandler {
    private static final Logger LOGGER = LoggerFactory.getLogger(ApiExceptionHandler.class);

    @ExceptionHandler({IllegalArgumentException.class, MethodArgumentNotValidException.class,
            HttpMessageNotReadableException.class, ServletRequestBindingException.class,
            MethodArgumentTypeMismatchException.class})
    ResponseEntity<ApiError> invalidRequest(Exception exception) {
        return error(HttpStatus.BAD_REQUEST, "INVALID_REQUEST", exception.getMessage());
    }

    @ExceptionHandler({PositionNotFoundException.class, PortfolioNotFoundException.class})
    ResponseEntity<ApiError> notFound(RuntimeException exception) {
        return error(HttpStatus.NOT_FOUND, "NOT_FOUND", exception.getMessage());
    }

    @ExceptionHandler({IdempotencyConflictException.class, InvalidPositionTransitionException.class})
    ResponseEntity<ApiError> conflict(RuntimeException exception) {
        return error(HttpStatus.CONFLICT, "CONFLICT", exception.getMessage());
    }

    @ExceptionHandler(IncompleteValuationException.class)
    ResponseEntity<ApiError> incomplete(IncompleteValuationException exception) {
        return error(HttpStatus.UNPROCESSABLE_ENTITY, "VALUATION_INCOMPLETE", exception.getMessage());
    }

    @ExceptionHandler(ElexonUnavailableException.class)
    ResponseEntity<ApiError> upstream(ElexonUnavailableException exception) {
        return error(HttpStatus.BAD_GATEWAY, "ELEXON_UNAVAILABLE", exception.getMessage());
    }

    @ExceptionHandler(ElexonTimeoutException.class)
    ResponseEntity<ApiError> upstreamTimeout(ElexonTimeoutException exception) {
        return error(HttpStatus.GATEWAY_TIMEOUT, "ELEXON_TIMEOUT", exception.getMessage());
    }

    @ExceptionHandler(ResourceLimitExceededException.class)
    ResponseEntity<ApiError> resourceLimit(ResourceLimitExceededException exception) {
        return error(HttpStatus.TOO_MANY_REQUESTS, "RESOURCE_LIMIT_EXCEEDED", exception.getMessage());
    }

    @ExceptionHandler(DataAccessException.class)
    ResponseEntity<ApiError> databaseUnavailable(DataAccessException exception) {
        LOGGER.error("Database operation failed", NestedExceptionUtils.getMostSpecificCause(exception));
        return error(HttpStatus.SERVICE_UNAVAILABLE, "DATABASE_UNAVAILABLE", "database operation failed");
    }

    @ExceptionHandler({CancellationException.class, InterruptedException.class})
    ResponseEntity<ApiError> cancelled(Exception exception) {
        if (exception instanceof InterruptedException) Thread.currentThread().interrupt();
        return ResponseEntity.status(HttpStatusCode.valueOf(499))
                .body(new ApiError("REQUEST_CANCELLED", "request processing was cancelled", Instant.now()));
    }

    @ExceptionHandler(AsyncRequestNotUsableException.class)
    void clientDisconnected(AsyncRequestNotUsableException exception) {
        LOGGER.debug("Client disconnected before the response completed: {}", exception.getMessage());
    }

    @ExceptionHandler(Exception.class)
    ResponseEntity<ApiError> unexpected(Exception exception) {
        Throwable root = NestedExceptionUtils.getMostSpecificCause(exception);
        LOGGER.error("Unhandled API failure", root);
        return error(HttpStatus.INTERNAL_SERVER_ERROR, "INTERNAL_ERROR", "an unexpected error occurred");
    }

    private ResponseEntity<ApiError> error(HttpStatus status, String code, String message) {
        return ResponseEntity.status(status).body(new ApiError(code, message, Instant.now()));
    }
}
