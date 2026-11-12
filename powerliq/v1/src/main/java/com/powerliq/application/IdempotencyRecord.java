package com.powerliq.application;

import java.util.UUID;

public record IdempotencyRecord(String requestHash, UUID positionId) {
}
