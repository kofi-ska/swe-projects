package com.powerliq.financial;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

import java.time.Duration;
import java.time.Instant;
import java.time.LocalDate;
import org.junit.jupiter.api.Test;

class SettlementIntervalTest {

    @Test
    void ordinaryDayHasFortyEightPeriods() {
        assertEquals(48, SettlementInterval.periodsOn(LocalDate.of(2026, 2, 10)));
    }

    @Test
    void springClockChangeHasFortySixPeriods() {
        assertEquals(46, SettlementInterval.periodsOn(LocalDate.of(2026, 3, 29)));
    }

    @Test
    void autumnClockChangeHasFiftyPeriods() {
        assertEquals(50, SettlementInterval.periodsOn(LocalDate.of(2026, 10, 25)));
    }

    @Test
    void everyIntervalIsExactlyThirtyMinutesInElapsedTime() {
        SettlementInterval interval = SettlementInterval.of(LocalDate.of(2026, 10, 25), 5);
        assertEquals(Duration.ofMinutes(30), Duration.between(interval.deliveryStart(), interval.deliveryEnd()));
    }

    @Test
    void rejectsPeriodOutsideTheSettlementDay() {
        LocalDate shortDay = LocalDate.of(2026, 3, 29);
        assertThrows(IllegalArgumentException.class, () -> SettlementInterval.of(shortDay, 47));
    }

    @Test
    void rejectsDeliveryTimesThatDoNotMatchThePeriodIdentity() {
        SettlementInterval valid = SettlementInterval.of(LocalDate.of(2026, 2, 10), 10);
        Instant wrongStart = valid.deliveryStart().plus(Duration.ofMinutes(30));

        assertThrows(
                IllegalArgumentException.class,
                () -> new SettlementInterval(
                        valid.settlementDate(), valid.settlementPeriod(), wrongStart,
                        wrongStart.plus(Duration.ofMinutes(30))));
    }
}
