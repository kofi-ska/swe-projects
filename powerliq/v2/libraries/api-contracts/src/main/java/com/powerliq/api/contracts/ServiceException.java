package com.powerliq.api.contracts;

import java.util.Objects;

public class ServiceException extends RuntimeException {
    private final ErrorDescriptor descriptor;

    public ServiceException(ErrorDescriptor descriptor, String message) {
        super(message);
        this.descriptor = Objects.requireNonNull(descriptor, "descriptor");
    }

    public ErrorDescriptor descriptor() {
        return descriptor;
    }
}
