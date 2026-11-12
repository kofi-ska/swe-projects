package com.powerliq.financial;

import java.time.Duration;
import java.time.Instant;
import java.time.LocalDate;
import java.time.ZoneId;
import java.util.Objects;

public record SettlementInterval(
        LocalDate settlementDate,
        int settlementPeriod,
        Instant deliveryStart,
        Instant deliveryEnd) {

    public static final ZoneId GB_MARKET_ZONE = ZoneId.of("Europe/London");
    private static final Duration PERIOD_LENGTH = Duration.ofMinutes(30);

    public SettlementInterval {
        Objects.requireNonNull(settlementDate, "settlementDate must not be null");
        Objects.requireNonNull(deliveryStart, "deliveryStart must not be null");
        Objects.requireNonNull(deliveryEnd, "deliveryEnd must not be null");
        Instant dayStart = settlementDate.atStartOfDay(GB_MARKET_ZONE).toInstant();
        Instant nextDayStart = settlementDate.plusDays(1).atStartOfDay(GB_MARKET_ZONE).toInstant();
        long periodCount = Duration.between(dayStart, nextDayStart).dividedBy(PERIOD_LENGTH);
        if (settlementPeriod < 1 || settlementPeriod > periodCount) {
            throw new IllegalArgumentException(
                    "settlementPeriod must be between 1 and " + periodCount + " for " + settlementDate);
        }
        Instant expectedStart = dayStart.plus(PERIOD_LENGTH.multipliedBy(settlementPeriod - 1L));
        if (!deliveryStart.equals(expectedStart) || !deliveryEnd.equals(expectedStart.plus(PERIOD_LENGTH))) {
            throw new IllegalArgumentException("delivery times do not match the settlement date and period");
        }
    }

    public static SettlementInterval of(LocalDate settlementDate, int settlementPeriod) {
        Objects.requireNonNull(settlementDate, "settlementDate must not be null");
        Instant dayStart = settlementDate.atStartOfDay(GB_MARKET_ZONE).toInstant();
        Instant nextDayStart = settlementDate.plusDays(1).atStartOfDay(GB_MARKET_ZONE).toInstant();
        Instant start = dayStart.plus(PERIOD_LENGTH.multipliedBy(settlementPeriod - 1L));
        return new SettlementInterval(settlementDate, settlementPeriod, start, start.plus(PERIOD_LENGTH));
    }

    public static long periodsOn(LocalDate settlementDate) {
        Instant start = settlementDate.atStartOfDay(GB_MARKET_ZONE).toInstant();
        Instant end = settlementDate.plusDays(1).atStartOfDay(GB_MARKET_ZONE).toInstant();
        return Duration.between(start, end).dividedBy(PERIOD_LENGTH);
    }

    public static SettlementInterval fromDeliveryStart(Instant deliveryStart) {
        Objects.requireNonNull(deliveryStart, "deliveryStart must not be null");
        LocalDate date = deliveryStart.atZone(GB_MARKET_ZONE).toLocalDate();
        Instant dayStart = date.atStartOfDay(GB_MARKET_ZONE).toInstant();
        Duration elapsed = Duration.between(dayStart, deliveryStart);
        if (elapsed.isNegative() || elapsed.toSeconds() % PERIOD_LENGTH.toSeconds() != 0) {
            throw new IllegalArgumentException("deliveryStart must align to a GB half-hour settlement boundary");
        }
        int period = Math.toIntExact(elapsed.dividedBy(PERIOD_LENGTH)) + 1;
        return of(date, period);
    }
}
