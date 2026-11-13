package com.powerliq.marketdata;
import static org.assertj.core.api.Assertions.*;import com.powerliq.events.*;import java.math.BigDecimal;import java.time.*;import java.util.UUID;import org.junit.jupiter.api.*;import org.springframework.beans.factory.annotation.Autowired;import org.springframework.boot.test.context.SpringBootTest;import org.springframework.jdbc.core.JdbcTemplate;import org.springframework.test.context.*;import org.testcontainers.containers.PostgreSQLContainer;import org.testcontainers.junit.jupiter.*;
@SpringBootTest @Testcontainers class MarketDataServiceIntegrationTest{
 @Container static final PostgreSQLContainer<?> POSTGRES=new PostgreSQLContainer<>("postgres:16-alpine");@DynamicPropertySource static void db(DynamicPropertyRegistry r){r.add("spring.datasource.url",POSTGRES::getJdbcUrl);r.add("spring.datasource.username",POSTGRES::getUsername);r.add("spring.datasource.password",POSTGRES::getPassword);}
 @Autowired MarketDataService service;@Autowired JdbcTemplate jdbc;Instant start=Instant.parse("2026-01-01T00:00:00Z");
 @BeforeEach void clean(){jdbc.update("DELETE FROM current_market_prices");jdbc.update("DELETE FROM market_price_revisions");}
 @Test void consumesIdempotentlyAndPromotesNewestCorrection(){
  var original=event(UUID.randomUUID(),"50",start,start.plusSeconds(10),"r1");assertThat(service.consume(original)).isTrue();assertThat(service.consume(original)).isFalse();
  service.consume(event(UUID.randomUUID(),"40",start.minusSeconds(1),start.plusSeconds(20),"late-old"));assertThat(service.latest("N2EX",start).priceGbpPerMWh()).isEqualByComparingTo("50");
  service.consume(event(UUID.randomUUID(),"55",start.plusSeconds(1),start.plusSeconds(30),"r2"));assertThat(service.latest("N2EX",start).priceGbpPerMWh()).isEqualByComparingTo("55");
  assertThat(service.history("N2EX",start.minusSeconds(1),start.plusSeconds(1800),100)).hasSize(3);assertThat(jdbc.queryForObject("SELECT count(*) FROM current_market_prices",Long.class)).isEqualTo(1);
 }
 @Test void rejectsUnboundedHistory(){assertThatThrownBy(()->service.history("N2EX",start,start.plus(Duration.ofDays(32)),100)).isInstanceOf(com.powerliq.api.contracts.ServiceException.class);}
 private EventEnvelope<MarketPriceRevisionV1> event(UUID id,String price,Instant source,Instant retrieved,String revision){var payload=new MarketPriceRevisionV1("N2EX",start,start.plusSeconds(1800),new BigDecimal(price),revision,source,retrieved);return new EventEnvelope<>(id,"market-price.revised",1,retrieved,"test",UUID.randomUUID(),null,id,start.toString(),payload);}
}
