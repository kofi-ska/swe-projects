package com.powerliq.application;

import com.powerliq.financial.Position;
import java.util.List;
import java.util.Optional;
import java.util.UUID;

public interface PositionRepository {
    Position save(Position position);
    List<Position> findAll();
    List<Position> findByPortfolioId(UUID portfolioId);
    Optional<Position> findById(UUID id);
    Position settle(UUID id);
}
