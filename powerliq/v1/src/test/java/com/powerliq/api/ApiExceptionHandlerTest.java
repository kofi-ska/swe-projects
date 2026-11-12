package com.powerliq.api;

import static org.junit.jupiter.api.Assertions.assertEquals;

import com.powerliq.marketdata.ElexonTimeoutException;
import com.powerliq.application.ResourceLimitExceededException;
import java.net.http.HttpTimeoutException;
import java.util.concurrent.CancellationException;
import org.junit.jupiter.api.Test;
import org.springframework.http.HttpStatus;

class ApiExceptionHandlerTest {
    private final ApiExceptionHandler handler = new ApiExceptionHandler();

    @Test
    void upstreamDeadlineIsDistinguishedFromOtherElexonFailures() {
        var response = handler.upstreamTimeout(new ElexonTimeoutException(
                "timed out", new HttpTimeoutException("timed out")));

        assertEquals(HttpStatus.GATEWAY_TIMEOUT, response.getStatusCode());
        assertEquals("ELEXON_TIMEOUT", response.getBody().code());
    }

    @Test
    void cancellationUsesClientClosedRequestStatus() {
        var response = handler.cancelled(new CancellationException());

        assertEquals(499, response.getStatusCode().value());
        assertEquals("REQUEST_CANCELLED", response.getBody().code());
    }

    @Test
    void boundedResourceExhaustionIsExplicit() {
        var response = handler.resourceLimit(new ResourceLimitExceededException("capacity reached"));

        assertEquals(HttpStatus.TOO_MANY_REQUESTS, response.getStatusCode());
        assertEquals("RESOURCE_LIMIT_EXCEEDED", response.getBody().code());
    }

    @Test
    void unexpectedFailureDoesNotLeakItsMessage() {
        var response = handler.unexpected(new RuntimeException("secret database detail"));

        assertEquals(HttpStatus.INTERNAL_SERVER_ERROR, response.getStatusCode());
        assertEquals("an unexpected error occurred", response.getBody().message());
    }
}
