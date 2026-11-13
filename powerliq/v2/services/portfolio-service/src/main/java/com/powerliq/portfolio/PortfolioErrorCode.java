package com.powerliq.portfolio;

import com.powerliq.api.contracts.ErrorDescriptor;

public enum PortfolioErrorCode implements ErrorDescriptor {
    PORTFOLIO_INVALID(400, false), PORTFOLIO_NOT_FOUND(404, false), PORTFOLIO_ALREADY_EXISTS(409, false),
    CONFIGURATION_VERSION_CONFLICT(409, true), PORTFOLIO_PERSISTENCE_FAILED(503, true);
    private final int status; private final boolean retryable;
    PortfolioErrorCode(int status, boolean retryable) { this.status = status; this.retryable = retryable; }
    public String code() { return name(); } public int httpStatus() { return status; }
    public boolean retryable() { return retryable; }
}
