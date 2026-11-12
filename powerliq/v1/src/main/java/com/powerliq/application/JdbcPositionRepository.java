package com.powerliq.application;

import com.powerliq.financial.EnergyMWh;
import com.powerliq.financial.Position;
import com.powerliq.financial.PositionSide;
import com.powerliq.financial.PositionStatus;
import com.powerliq.financial.PriceGbpPerMWh;
import com.powerliq.financial.SettlementInterval;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Timestamp;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.stereotype.Repository;

@Repository
public class JdbcPositionRepository implements PositionRepository {
    private static final int MAX_POSITIONS = 1_000;
    private static final long CAPACITY_LOCK = 8_104_771L;
    private final JdbcTemplate jdbc;
    private final RowMapper<Position> mapper = this::map;

    public JdbcPositionRepository(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    @Override
    public Position save(Position position) {
        jdbc.query("SELECT pg_advisory_xact_lock(?)", resultSet -> null, CAPACITY_LOCK);
        Integer count = jdbc.queryForObject("SELECT count(*) FROM positions", Integer.class);
        if (count != null && count >= MAX_POSITIONS) {
            throw new ResourceLimitExceededException("position capacity has been reached");
        }
        jdbc.update("""
                INSERT INTO positions
                  (id, portfolio_id, side, contract_price, volume_mwh, settlement_date,
                   settlement_period, delivery_start, delivery_end, status, created_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """, position.id(), position.portfolioId(), position.side().name(),
                position.contractPrice().value(), position.volume().value(),
                position.settlementInterval().settlementDate(), position.settlementInterval().settlementPeriod(),
                Timestamp.from(position.settlementInterval().deliveryStart()),
                Timestamp.from(position.settlementInterval().deliveryEnd()),
                position.status().name(), Timestamp.from(position.createdAt()));
        return position;
    }

    @Override
    public List<Position> findAll() {
        return jdbc.query("SELECT * FROM positions ORDER BY created_at, id", mapper);
    }

    @Override
    public List<Position> findByPortfolioId(UUID portfolioId) {
        return jdbc.query("SELECT * FROM positions WHERE portfolio_id = ? ORDER BY created_at, id",
                mapper, portfolioId);
    }

    @Override
    public Optional<Position> findById(UUID id) {
        return jdbc.query("SELECT * FROM positions WHERE id = ?", mapper, id).stream().findFirst();
    }

    @Override
    public Position settle(UUID id) {
        int changed = jdbc.update(
                "UPDATE positions SET status = 'SETTLED' WHERE id = ? AND status = 'ACTIVE'", id);
        if (changed == 1) return findById(id).orElseThrow(() -> new PositionNotFoundException(id));
        if (findById(id).isEmpty()) throw new PositionNotFoundException(id);
        throw new InvalidPositionTransitionException(id);
    }

    private Position map(ResultSet result, int rowNumber) throws SQLException {
        SettlementInterval interval = new SettlementInterval(
                result.getObject("settlement_date", java.time.LocalDate.class),
                result.getInt("settlement_period"),
                result.getTimestamp("delivery_start").toInstant(),
                result.getTimestamp("delivery_end").toInstant());
        return new Position(result.getObject("id", UUID.class), result.getObject("portfolio_id", UUID.class),
                PositionSide.valueOf(result.getString("side")),
                new PriceGbpPerMWh(result.getBigDecimal("contract_price")),
                new EnergyMWh(result.getBigDecimal("volume_mwh")), interval,
                PositionStatus.valueOf(result.getString("status")),
                result.getTimestamp("created_at").toInstant());
    }
}
