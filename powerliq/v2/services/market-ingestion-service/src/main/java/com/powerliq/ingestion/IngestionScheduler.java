package com.powerliq.ingestion;
import com.powerliq.api.contracts.ServiceException;
import java.time.Clock;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.UUID;
import org.slf4j.Logger;import org.slf4j.LoggerFactory;
import org.springframework.scheduling.annotation.Scheduled;import org.springframework.stereotype.Component;
@Component public class IngestionScheduler {
 private static final Logger LOG=LoggerFactory.getLogger(IngestionScheduler.class); private final IngestionService service;private final Clock clock;
 public IngestionScheduler(IngestionService service,Clock clock){this.service=service;this.clock=clock;}
 @Scheduled(cron="${powerliq.ingestion.cron:0 2,32 * * * *}",zone="UTC") void ingestPreviousPeriod(){
  Instant now=Instant.now(clock).truncatedTo(ChronoUnit.MINUTES);long minute=now.getEpochSecond()/60;Instant end=Instant.ofEpochSecond((minute/30)*30*60);Instant start=end.minus(30,ChronoUnit.MINUTES);
  try{service.run(IngestionRun.Type.SCHEDULED,start,end,UUID.randomUUID());}catch(ServiceException e){if(e.descriptor()!=IngestionErrorCode.INGESTION_ALREADY_RUNNING)LOG.error("Scheduled ingestion failed: {}",e.descriptor().code());}
 }
}
