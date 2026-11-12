package com.powerliq.application;

import java.util.UUID;

public final class InvalidPositionTransitionException extends RuntimeException {
    public InvalidPositionTransitionException(UUID id) {
        super("position is not active: " + id);
    }
}
