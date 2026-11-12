package com.powerliq.application;

import com.powerliq.financial.Position;
import com.powerliq.financial.PositionStatus;
import java.util.Comparator;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.Semaphore;

public class PositionStore implements PositionRepository {
    private static final int DEFAULT_MAX_POSITIONS = 1_000;
    private final ConcurrentHashMap<UUID, Position> positions = new ConcurrentHashMap<>();
    private final Semaphore capacity;

    public PositionStore() {
        this(DEFAULT_MAX_POSITIONS);
    }

    PositionStore(int maxPositions) {
        if (maxPositions < 1) throw new IllegalArgumentException("maxPositions must be positive");
        this.capacity = new Semaphore(maxPositions);
    }

    public Position save(Position position) {
        if (!capacity.tryAcquire()) {
            throw new ResourceLimitExceededException("position capacity has been reached");
        }
        Position existing = positions.putIfAbsent(position.id(), position);
        if (existing != null) {
            capacity.release();
            throw new IllegalStateException("duplicate position id");
        }
        return position;
    }

    public List<Position> findAll() {
        return positions.values().stream()
                .sorted(Comparator.comparing(Position::createdAt).thenComparing(Position::id))
                .toList();
    }

    public List<Position> findByPortfolioId(UUID portfolioId) {
        return findAll().stream().filter(position -> position.portfolioId().equals(portfolioId)).toList();
    }

    public Optional<Position> findById(UUID id) {
        return Optional.ofNullable(positions.get(id));
    }

    public Position settle(UUID id) {
        Position result = positions.compute(id, (ignored, current) -> {
            if (current == null) throw new PositionNotFoundException(id);
            if (current.status() != PositionStatus.ACTIVE) throw new InvalidPositionTransitionException(id);
            return new Position(current.id(), current.portfolioId(), current.side(), current.contractPrice(),
                    current.volume(), current.settlementInterval(), PositionStatus.SETTLED, current.createdAt());
        });
        return result;
    }
}
