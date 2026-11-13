package com.powerliq.portfolio;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.UUID;

public record Portfolio(UUID id, String name, BigDecimal availableCashGbp, BigDecimal liquidityBufferGbp,
                        long configurationVersion, Instant createdAt, Instant updatedAt) { }
