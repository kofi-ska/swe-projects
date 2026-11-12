package com.powerliq.application;

import com.powerliq.financial.EnergyMWh;
import com.powerliq.financial.Position;
import com.powerliq.financial.PositionStatus;
import com.powerliq.financial.PriceGbpPerMWh;
import com.powerliq.financial.SettlementInterval;
import java.time.Clock;
import java.time.Instant;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.HexFormat;
import java.util.List;
import java.util.UUID;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class PositionService {
    private final PositionRepository store;
    private final IdempotencyRepository idempotency;
    private final Clock clock;

    public PositionService(PositionRepository store, IdempotencyRepository idempotency, Clock clock) {
        this.store = store;
        this.idempotency = idempotency;
        this.clock = clock;
    }

    @Transactional
    public Position create(String idempotencyKey, CreatePositionCommand command) {
        if (idempotencyKey == null || idempotencyKey.isBlank() || idempotencyKey.length() > 128) {
            throw new IllegalArgumentException("Idempotency-Key must contain between 1 and 128 characters");
        }
        String requestHash = hash(command);
        Instant now = Instant.now(clock);
        UUID positionId = UUID.randomUUID();
        if (!idempotency.claim(idempotencyKey, requestHash, positionId, now)) {
            IdempotencyRecord existing = idempotency.find(idempotencyKey).orElseThrow();
            if (!existing.requestHash().equals(requestHash)) throw new IdempotencyConflictException();
            return store.findById(existing.positionId()).orElseThrow();
        }
        Position created = new Position(positionId, command.portfolioId(), command.side(),
                new PriceGbpPerMWh(command.contractPriceGbpPerMWh()), new EnergyMWh(command.volumeMWh()),
                SettlementInterval.of(command.settlementDate(), command.settlementPeriod()),
                PositionStatus.ACTIVE, now);
        return store.save(created);
    }

    public List<Position> findAll() {
        return store.findAll();
    }

    @Transactional
    public Position settle(UUID id) {
        return store.settle(id);
    }

    private String hash(CreatePositionCommand command) {
        String canonical = String.join("|", command.portfolioId().toString(), command.side().name(),
                command.contractPriceGbpPerMWh().stripTrailingZeros().toPlainString(),
                command.volumeMWh().stripTrailingZeros().toPlainString(), command.settlementDate().toString(),
                Integer.toString(command.settlementPeriod()));
        try {
            return HexFormat.of().formatHex(
                    MessageDigest.getInstance("SHA-256").digest(canonical.getBytes(StandardCharsets.UTF_8)));
        } catch (NoSuchAlgorithmException exception) {
            throw new IllegalStateException("SHA-256 is unavailable", exception);
        }
    }
}
