package com.powerliq.application;

import java.util.UUID;

public final class PortfolioNotFoundException extends RuntimeException {
    public PortfolioNotFoundException(UUID id) {
        super("portfolio not found: " + id);
    }
}
