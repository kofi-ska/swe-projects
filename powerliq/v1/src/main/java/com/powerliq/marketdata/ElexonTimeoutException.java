package com.powerliq.marketdata;

public final class ElexonTimeoutException extends RuntimeException {
    public ElexonTimeoutException(String message, Throwable cause) {
        super(message, cause);
    }
}
