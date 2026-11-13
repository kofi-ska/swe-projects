package com.powerliq.position.application;

import com.powerliq.api.contracts.ServiceException;
import com.powerliq.events.EventEnvelope;
import com.powerliq.events.PositionCreatedV1;
import com.powerliq.events.PositionSettledV1;
import com.powerliq.financial.EnergyMWh;
import com.powerliq.financial.PriceGbpPerMWh;
import com.powerliq.financial.SettlementInterval;
import com.powerliq.position.domain.Position;
import com.powerliq.position.domain.PositionStatus;
import com.powerliq.position.persistence.IdempotencyRepository;
import com.powerliq.position.persistence.OutboxRepository;
import com.powerliq.position.persistence.PositionRepository;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.time.Clock;
import java.time.Instant;
import java.util.HexFormat;
import java.util.List;
import java.util.UUID;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class PositionApplicationService {
    private static final String SOURCE = "position-service";
    private final PositionRepository positions;
    private final IdempotencyRepository idempotency;
    private final OutboxRepository outbox;
    private final Clock clock;

    public PositionApplicationService(
            PositionRepository positions,
            IdempotencyRepository idempotency,
            OutboxRepository outbox,
            Clock clock) {
        this.positions = positions;
        this.idempotency = idempotency;
        this.outbox = outbox;
        this.clock = clock;
    }

    @Transactional
    public Position create(String idempotencyKey, UUID correlationId, CreatePositionCommand command) {
        validateKey(idempotencyKey);
        SettlementInterval interval;
        PriceGbpPerMWh price;
        EnergyMWh volume;
        try {
            interval = SettlementInterval.of(command.settlementDate(), command.settlementPeriod());
            price = new PriceGbpPerMWh(command.contractPriceGbpPerMWh());
            volume = new EnergyMWh(command.volumeMWh());
        } catch (IllegalArgumentException exception) {
            throw new ServiceException(PositionErrorCode.POSITION_INVALID, exception.getMessage());
        }

        String requestHash = hash(command);
        UUID positionId = UUID.randomUUID();
        Instant now = Instant.now(clock);
        if (!idempotency.claim(idempotencyKey, requestHash, positionId, now)) {
            IdempotencyRepository.IdempotencyRecord existing = idempotency.find(idempotencyKey)
                    .orElseThrow(() -> new IllegalStateException("Committed idempotency record is missing"));
            if (!existing.requestHash().equals(requestHash)) {
                throw new ServiceException(PositionErrorCode.IDEMPOTENCY_CONFLICT,
                        "Idempotency key was already used with a different request");
            }
            return positions.findById(existing.positionId())
                    .orElseThrow(() -> new IllegalStateException("Committed idempotent position is missing"));
        }

        Position position = new Position(positionId, command.portfolioId(), command.side(), price.value(),
                volume.value(), interval.settlementDate(), interval.settlementPeriod(), interval.deliveryStart(),
                interval.deliveryEnd(), PositionStatus.ACTIVE, now);
        positions.insert(position);
        PositionCreatedV1 payload = new PositionCreatedV1(position.id(), position.portfolioId(),
                position.side().name(), position.contractPriceGbpPerMWh(), position.volumeMWh(),
                position.deliveryStart(), position.deliveryEnd(), position.createdAt());
        outbox.append(new EventEnvelope<>(UUID.randomUUID(), "position.created", 1, now, SOURCE,
                correlationId, null, position.id(), position.id().toString(), payload));
        return position;
    }

    @Transactional(readOnly = true)
    public Position findById(UUID id) {
        return positions.findById(id).orElseThrow(() -> new ServiceException(
                PositionErrorCode.POSITION_NOT_FOUND, "Position was not found"));
    }

    @Transactional(readOnly = true)
    public List<Position> findAll(int limit, int offset) {
        if (limit < 1 || limit > 200 || offset < 0) {
            throw new ServiceException(PositionErrorCode.POSITION_INVALID,
                    "limit must be between 1 and 200 and offset must not be negative");
        }
        return positions.findAll(limit, offset);
    }

    @Transactional
    public Position settle(UUID id, UUID correlationId) {
        if (!positions.settle(id)) {
            Position existing = positions.findById(id).orElseThrow(() -> new ServiceException(
                    PositionErrorCode.POSITION_NOT_FOUND, "Position was not found"));
            if (existing.status() == PositionStatus.SETTLED) {
                throw new ServiceException(PositionErrorCode.POSITION_ALREADY_SETTLED,
                        "Position is already settled");
            }
            throw new ServiceException(PositionErrorCode.INVALID_POSITION_TRANSITION,
                    "Position cannot transition to settled");
        }
        Instant now = Instant.now(clock);
        outbox.append(new EventEnvelope<>(UUID.randomUUID(), "position.settled", 1, now, SOURCE,
                correlationId, null, id, id.toString(), new PositionSettledV1(id, now)));
        return positions.findById(id).orElseThrow();
    }

    private void validateKey(String key) {
        if (key == null || key.isBlank()) {
            throw new ServiceException(PositionErrorCode.IDEMPOTENCY_KEY_REQUIRED,
                    "Idempotency-Key header is required");
        }
        if (key.length() > 128) {
            throw new ServiceException(PositionErrorCode.IDEMPOTENCY_KEY_INVALID,
                    "Idempotency-Key must not exceed 128 characters");
        }
    }

    private String hash(CreatePositionCommand command) {
        String canonical = String.join("|", command.portfolioId().toString(), command.side().name(),
                command.contractPriceGbpPerMWh().stripTrailingZeros().toPlainString(),
                command.volumeMWh().stripTrailingZeros().toPlainString(), command.settlementDate().toString(),
                Integer.toString(command.settlementPeriod()));
        try {
            byte[] digest = MessageDigest.getInstance("SHA-256")
                    .digest(canonical.getBytes(StandardCharsets.UTF_8));
            return HexFormat.of().formatHex(digest);
        } catch (NoSuchAlgorithmException exception) {
            throw new IllegalStateException("SHA-256 is unavailable", exception);
        }
    }
}
