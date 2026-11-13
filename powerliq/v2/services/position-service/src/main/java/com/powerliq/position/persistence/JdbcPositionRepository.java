package com.powerliq.position.persistence;

import com.powerliq.financial.PositionSide;
import com.powerliq.position.domain.Position;
import com.powerliq.position.domain.PositionStatus;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Timestamp;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

@Repository
public class JdbcPositionRepository implements PositionRepository {
    private final JdbcTemplate jdbc;

    public JdbcPositionRepository(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    @Override
    public void insert(Position position) {
        jdbc.update("""
                INSERT INTO positions
                  (id, portfolio_id, side, contract_price, volume_mwh, settlement_date,
                   settlement_period, delivery_start, delivery_end, status, created_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """, position.id(), position.portfolioId(), position.side().name(),
                position.contractPriceGbpPerMWh(), position.volumeMWh(), position.settlementDate(),
                position.settlementPeriod(), Timestamp.from(position.deliveryStart()),
                Timestamp.from(position.deliveryEnd()), position.status().name(), Timestamp.from(position.createdAt()));
    }

    @Override
    public Optional<Position> findById(UUID id) {
        return jdbc.query("SELECT * FROM positions WHERE id = ?", this::map, id).stream().findFirst();
    }

    @Override
    public List<Position> findAll(int limit, int offset) {
        return jdbc.query("SELECT * FROM positions ORDER BY created_at, id LIMIT ? OFFSET ?",
                this::map, limit, offset);
    }

    @Override
    public boolean settle(UUID id) {
        return jdbc.update("UPDATE positions SET status = 'SETTLED' WHERE id = ? AND status = 'ACTIVE'", id) == 1;
    }

    private Position map(ResultSet row, int rowNumber) throws SQLException {
        return new Position(row.getObject("id", UUID.class), row.getObject("portfolio_id", UUID.class),
                PositionSide.valueOf(row.getString("side")), row.getBigDecimal("contract_price"),
                row.getBigDecimal("volume_mwh"), row.getObject("settlement_date", java.time.LocalDate.class),
                row.getInt("settlement_period"), row.getTimestamp("delivery_start").toInstant(),
                row.getTimestamp("delivery_end").toInstant(), PositionStatus.valueOf(row.getString("status")),
                row.getTimestamp("created_at").toInstant());
    }
}
