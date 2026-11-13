package com.powerliq.position.persistence;

import com.powerliq.position.domain.Position;
import java.util.List;
import java.util.Optional;
import java.util.UUID;

public interface PositionRepository {
    void insert(Position position);

    Optional<Position> findById(UUID id);

    List<Position> findAll(int limit, int offset);

    boolean settle(UUID id);
}
