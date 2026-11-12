package com.powerliq.application;

public final class ResourceLimitExceededException extends RuntimeException {
    public ResourceLimitExceededException(String message) {
        super(message);
    }
}
