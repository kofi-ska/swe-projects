package com.powerliq.api.contracts;

import java.time.Instant;
import java.util.List;

public record ApiError(
        String code,
        String message,
        String service,
        String correlationId,
        Instant timestamp,
        boolean retryable,
        List<FieldViolation> details) {

    public ApiError {
        details = details == null ? List.of() : List.copyOf(details);
    }
}
