package com.powerliq.application;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

import com.powerliq.financial.Position;
import com.powerliq.financial.PositionSide;
import java.math.BigDecimal;
import java.time.Clock;
import java.time.Instant;
import java.time.LocalDate;
import java.time.ZoneOffset;
import java.util.UUID;
import org.junit.jupiter.api.Test;

class PositionServiceConcurrencyTest {
    private final PositionStore store = new PositionStore();
    private final PositionService service = new PositionService(
            store, new InMemoryIdempotencyRepository(),
            Clock.fixed(Instant.parse("2026-11-12T12:00:00Z"), ZoneOffset.UTC));
    private final CreatePositionCommand command = new CreatePositionCommand(
            UUID.fromString("10000000-0000-0000-0000-000000000001"), PositionSide.LONG,
            new BigDecimal("80"), new BigDecimal("100"), LocalDate.of(2026, 11, 12), 20);

    @Test
    void sameKeyWithDifferentRequestIsRejected() {
        service.create("same-key", command);
        CreatePositionCommand different = new CreatePositionCommand(command.portfolioId(), PositionSide.SHORT,
                command.contractPriceGbpPerMWh(), command.volumeMWh(), command.settlementDate(),
                command.settlementPeriod());

        assertThrows(IdempotencyConflictException.class, () -> service.create("same-key", different));
    }

    @Test
    void onlyOneConcurrentSettlementCanSucceed() {
        Position position = service.create("create-key", command);
        service.settle(position.id());

        assertThrows(InvalidPositionTransitionException.class, () -> service.settle(position.id()));
    }

    @Test
    void positionCapacityIsHardBounded() {
        PositionStore boundedStore = new PositionStore(1);
        PositionService boundedService = new PositionService(
                boundedStore, new InMemoryIdempotencyRepository(),
                Clock.fixed(Instant.parse("2026-11-12T12:00:00Z"), ZoneOffset.UTC));
        boundedService.create("first", command);

        assertThrows(ResourceLimitExceededException.class,
                () -> boundedService.create("second", command));
        assertEquals(1, boundedStore.findAll().size());
    }
}
