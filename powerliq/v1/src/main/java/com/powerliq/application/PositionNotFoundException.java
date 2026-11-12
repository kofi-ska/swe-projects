package com.powerliq.application;

import java.util.UUID;

public final class PositionNotFoundException extends RuntimeException {
    public PositionNotFoundException(UUID id) {
        super("position not found: " + id);
    }
}
