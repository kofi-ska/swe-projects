package com.powerliq.position.application;

import com.powerliq.api.contracts.ErrorDescriptor;

public enum PositionErrorCode implements ErrorDescriptor {
    POSITION_INVALID(400, false),
    IDEMPOTENCY_KEY_REQUIRED(400, false),
    IDEMPOTENCY_KEY_INVALID(400, false),
    POSITION_NOT_FOUND(404, false),
    IDEMPOTENCY_CONFLICT(409, false),
    INVALID_POSITION_TRANSITION(409, false),
    POSITION_ALREADY_SETTLED(409, false),
    POSITION_CAPACITY_EXCEEDED(429, true),
    POSITION_WRITE_CONFLICT(409, true),
    POSITION_PERSISTENCE_FAILED(503, true);

    private final int httpStatus;
    private final boolean retryable;

    PositionErrorCode(int httpStatus, boolean retryable) {
        this.httpStatus = httpStatus;
        this.retryable = retryable;
    }

    @Override
    public String code() {
        return name();
    }

    @Override
    public int httpStatus() {
        return httpStatus;
    }

    @Override
    public boolean retryable() {
        return retryable;
    }
}
