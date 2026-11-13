package com.powerliq.ingestion;
import static org.assertj.core.api.Assertions.*;import static org.mockito.Mockito.*;
import java.math.BigDecimal;import java.time.Instant;import java.time.temporal.ChronoUnit;import java.util.List;import java.util.UUID;
import org.junit.jupiter.api.*;import org.springframework.beans.factory.annotation.Autowired;import org.springframework.boot.test.context.SpringBootTest;import org.springframework.jdbc.core.JdbcTemplate;import org.springframework.test.context.DynamicPropertyRegistry;import org.springframework.test.context.DynamicPropertySource;import org.springframework.test.context.bean.override.mockito.MockitoBean;import org.testcontainers.containers.PostgreSQLContainer;import org.testcontainers.junit.jupiter.*;
@SpringBootTest(properties="powerliq.ingestion.cron=-") @Testcontainers
class IngestionServiceIntegrationTest {
 @Container static final PostgreSQLContainer<?> POSTGRES=new PostgreSQLContainer<>("postgres:16-alpine");
 @DynamicPropertySource static void db(DynamicPropertyRegistry r){r.add("spring.datasource.url",POSTGRES::getJdbcUrl);r.add("spring.datasource.username",POSTGRES::getUsername);r.add("spring.datasource.password",POSTGRES::getPassword);}
 @Autowired IngestionService service;@Autowired JdbcTemplate jdbc;@MockitoBean ElexonClient source;
 @BeforeEach void clean(){jdbc.update("DELETE FROM market_price_events");jdbc.update("DELETE FROM ingestion_runs");reset(source);}
 @Test void recordsHistoryAndDeduplicatesSourceRevisions()throws Exception{
  Instant end=Instant.now().minus(1,ChronoUnit.HOURS).truncatedTo(ChronoUnit.MINUTES);Instant start=end.minus(30,ChronoUnit.MINUTES);
  var price=new ElexonClient.Price("N2EX",start,end,new BigDecimal("82.50"),"r1",start);
  when(source.fetch(start,end)).thenReturn(List.of(price,price));
  IngestionRun run=service.run(IngestionRun.Type.SCHEDULED,start,end,UUID.randomUUID());
  assertThat(run.status()).isEqualTo(IngestionRun.Status.SUCCEEDED);assertThat(run.recordsRead()).isEqualTo(2);assertThat(run.eventsWritten()).isEqualTo(1);
  assertThat(jdbc.queryForObject("SELECT count(*) FROM market_price_events",Long.class)).isEqualTo(1);
 }
 @Test void failedSourceRunIsDurablyVisibleAndRetryable()throws Exception{
  Instant end=Instant.now().minus(1,ChronoUnit.HOURS);Instant start=end.minus(30,ChronoUnit.MINUTES);
  when(source.fetch(start,end)).thenThrow(new ElexonClient.TransientSourceException("busy"));
  assertThatThrownBy(()->service.run(IngestionRun.Type.SCHEDULED,start,end,UUID.randomUUID())).isInstanceOf(com.powerliq.api.contracts.ServiceException.class);
  assertThat(jdbc.queryForObject("SELECT status FROM ingestion_runs",String.class)).isEqualTo("FAILED");
  assertThat(jdbc.queryForObject("SELECT error_code FROM ingestion_runs",String.class)).isEqualTo("ELEXON_UNAVAILABLE");
 }
 @Test void enforcesBackfillBoundAndSingleActiveRun(){
  Instant end=Instant.now().minus(1,ChronoUnit.DAYS);Instant start=end.minus(32,ChronoUnit.DAYS);
  assertThatThrownBy(()->service.run(IngestionRun.Type.BACKFILL,start,end,UUID.randomUUID())).isInstanceOf(com.powerliq.api.contracts.ServiceException.class);
  jdbc.update("INSERT INTO ingestion_runs(id,run_type,status,window_start,window_end,started_at) VALUES(?,'SCHEDULED','RUNNING',?,?,?)",UUID.randomUUID(),java.sql.Timestamp.from(end.minus(30,ChronoUnit.MINUTES)),java.sql.Timestamp.from(end),java.sql.Timestamp.from(Instant.now()));
  assertThatThrownBy(()->service.run(IngestionRun.Type.SCHEDULED,end.minus(30,ChronoUnit.MINUTES),end,UUID.randomUUID())).isInstanceOf(com.powerliq.api.contracts.ServiceException.class);
 }
}
