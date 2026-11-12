package com.powerliq.portfolio;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.powerliq.api.contracts.ServiceException;
import com.powerliq.events.EventEnvelope;
import com.powerliq.events.PortfolioLiquiditySettingsChangedV1;
import java.math.BigDecimal;
import java.sql.Timestamp;
import java.time.Clock;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class PortfolioService {
    private final JdbcTemplate jdbc; private final ObjectMapper json; private final Clock clock;
    public PortfolioService(JdbcTemplate jdbc, ObjectMapper json, Clock clock) {
        this.jdbc = jdbc; this.json = json; this.clock = clock;
    }

    @Transactional
    public Portfolio create(UUID id, String name, BigDecimal cash, BigDecimal buffer, UUID correlationId) {
        validate(name, cash, buffer); Instant now = Instant.now(clock);
        try {
            jdbc.update("INSERT INTO portfolios(id,name,available_cash_gbp,liquidity_buffer_gbp,configuration_version,created_at,updated_at) VALUES(?,?,?,?,1,?,?)",
                    id, name.strip(), cash, buffer, Timestamp.from(now), Timestamp.from(now));
        } catch (DuplicateKeyException exception) {
            throw new ServiceException(PortfolioErrorCode.PORTFOLIO_ALREADY_EXISTS, "Portfolio already exists");
        }
        append(id, cash, buffer, 1, now, correlationId); return find(id);
    }

    @Transactional
    public Portfolio updateSettings(UUID id, BigDecimal cash, BigDecimal buffer, long expectedVersion, UUID correlationId) {
        validate("valid", cash, buffer); Instant now = Instant.now(clock);
        int changed = jdbc.update("UPDATE portfolios SET available_cash_gbp=?,liquidity_buffer_gbp=?,configuration_version=configuration_version+1,updated_at=? WHERE id=? AND configuration_version=?",
                cash, buffer, Timestamp.from(now), id, expectedVersion);
        if (changed == 0) {
            if (!exists(id)) throw new ServiceException(PortfolioErrorCode.PORTFOLIO_NOT_FOUND, "Portfolio was not found");
            throw new ServiceException(PortfolioErrorCode.CONFIGURATION_VERSION_CONFLICT, "Portfolio settings changed; reload and retry");
        }
        Portfolio result = find(id); append(id, cash, buffer, result.configurationVersion(), now, correlationId); return result;
    }

    @Transactional(readOnly = true) public Portfolio find(UUID id) {
        return jdbc.query("SELECT * FROM portfolios WHERE id=?", (r,n) -> new Portfolio(r.getObject("id", UUID.class),
                r.getString("name"), r.getBigDecimal("available_cash_gbp"), r.getBigDecimal("liquidity_buffer_gbp"),
                r.getLong("configuration_version"), r.getTimestamp("created_at").toInstant(),
                r.getTimestamp("updated_at").toInstant()), id).stream().findFirst()
                .orElseThrow(() -> new ServiceException(PortfolioErrorCode.PORTFOLIO_NOT_FOUND, "Portfolio was not found"));
    }

    @Transactional(readOnly = true) public List<Portfolio> list(int limit, int offset) {
        if (limit < 1 || limit > 200 || offset < 0) throw new ServiceException(PortfolioErrorCode.PORTFOLIO_INVALID, "Invalid pagination");
        return jdbc.query("SELECT * FROM portfolios ORDER BY id LIMIT ? OFFSET ?", (r,n) -> new Portfolio(
                r.getObject("id", UUID.class), r.getString("name"), r.getBigDecimal("available_cash_gbp"),
                r.getBigDecimal("liquidity_buffer_gbp"), r.getLong("configuration_version"),
                r.getTimestamp("created_at").toInstant(), r.getTimestamp("updated_at").toInstant()), limit, offset);
    }

    private boolean exists(UUID id) { return Boolean.TRUE.equals(jdbc.queryForObject("SELECT EXISTS(SELECT 1 FROM portfolios WHERE id=?)", Boolean.class, id)); }
    private void validate(String name, BigDecimal cash, BigDecimal buffer) {
        if (name == null || name.isBlank() || name.length() > 120 || cash == null || buffer == null || cash.signum() < 0 || buffer.signum() < 0 || buffer.compareTo(cash) > 0)
            throw new ServiceException(PortfolioErrorCode.PORTFOLIO_INVALID, "Name is required and liquidity buffer must be between zero and available cash");
    }
    private void append(UUID id, BigDecimal cash, BigDecimal buffer, long version, Instant now, UUID correlationId) {
        try {
            var event = new EventEnvelope<>(UUID.randomUUID(), "portfolio.liquidity-settings.changed", 1, now,
                    "portfolio-service", correlationId, null, id, id.toString(),
                    new PortfolioLiquiditySettingsChangedV1(id, cash, buffer, version, now));
            jdbc.update("INSERT INTO outbox_events(event_id,aggregate_id,event_type,event_version,partition_key,correlation_id,payload,occurred_at) VALUES(?,?,?,?,?,?,?::jsonb,?)",
                    event.eventId(), id, event.eventType(), 1, event.partitionKey(), correlationId,
                    json.writeValueAsString(event), Timestamp.from(now));
        } catch (Exception exception) { throw new IllegalStateException("Unable to persist portfolio event", exception); }
    }
}
