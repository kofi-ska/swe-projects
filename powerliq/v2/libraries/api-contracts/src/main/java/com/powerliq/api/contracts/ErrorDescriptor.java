package com.powerliq.api.contracts;

public interface ErrorDescriptor {
    String code();

    int httpStatus();

    boolean retryable();
}
