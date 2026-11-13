package com.powerliq.ingestion;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.powerliq.api.contracts.ServiceException;
import com.powerliq.events.EventEnvelope;
import com.powerliq.events.MarketPriceRevisionV1;
import java.net.http.HttpTimeoutException;
import java.sql.Timestamp;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.CancellationException;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.dao.TransientDataAccessException;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

@Service
public class IngestionService {
 private final JdbcTemplate jdbc; private final ElexonClient source; private final ObjectMapper json; private final Clock clock;private final Duration maxBackfill;private final int dbRetryAttempts;private final long dbRetryBaseDelayMs;
 public IngestionService(JdbcTemplate jdbc,ElexonClient source,ObjectMapper json,Clock clock,@Value("${powerliq.ingestion.max-backfill-days:31}")long maxBackfillDays,@Value("${powerliq.ingestion.db-retry-attempts:3}")int dbRetryAttempts,@Value("${powerliq.ingestion.db-retry-base-delay-ms:25}")long dbRetryBaseDelayMs){if(maxBackfillDays<1||maxBackfillDays>366||dbRetryAttempts<1||dbRetryAttempts>10||dbRetryBaseDelayMs<1||dbRetryBaseDelayMs>10000)throw new IllegalArgumentException("Ingestion operational bounds are invalid");this.jdbc=jdbc;this.source=source;this.json=json;this.clock=clock;this.maxBackfill=Duration.ofDays(maxBackfillDays);this.dbRetryAttempts=dbRetryAttempts;this.dbRetryBaseDelayMs=dbRetryBaseDelayMs;}

 public IngestionRun run(IngestionRun.Type type, Instant from, Instant to, UUID correlationId) {
  validate(type,from,to); UUID id=UUID.randomUUID(); Instant started=Instant.now(clock);
  try { db(() -> jdbc.update("INSERT INTO ingestion_runs(id,run_type,status,window_start,window_end,started_at) VALUES(?,?,'RUNNING',?,?,?)",
          id,type.name(),Timestamp.from(from),Timestamp.from(to),Timestamp.from(started))); }
  catch(DuplicateKeyException e){throw new ServiceException(IngestionErrorCode.INGESTION_ALREADY_RUNNING,"An ingestion run is already active");}
  try {
   List<ElexonClient.Price> prices=source.fetch(from,to); long written=0;
   for(ElexonClient.Price price:prices){
    if(cancelRequested(id)) throw new CancellationException("Cancellation requested");
    written += persist(id,price,correlationId);
   }
   finish(id,"SUCCEEDED",prices.size(),written,null,null); return find(id);
  } catch(CancellationException e){ finish(id,"CANCELLED",0,0,"CANCELLED","Cancellation requested"); return find(id);
  } catch(HttpTimeoutException e){fail(id,"ELEXON_TIMEOUT",e);throw new ServiceException(IngestionErrorCode.ELEXON_TIMEOUT,"Elexon request timed out");
  } catch(ElexonClient.TransientSourceException e){fail(id,"ELEXON_UNAVAILABLE",e);throw new ServiceException(IngestionErrorCode.ELEXON_UNAVAILABLE,"Elexon is temporarily unavailable");
  } catch(InterruptedException e){Thread.currentThread().interrupt();finish(id,"CANCELLED",0,0,"INTERRUPTED","Run interrupted");return find(id);
  } catch(Exception e){fail(id,"INGESTION_FAILED",e);throw new IllegalStateException("Ingestion failed",e);}
 }
 public IngestionRun cancel(UUID id){
  int changed=jdbc.update("UPDATE ingestion_runs SET status='CANCEL_REQUESTED' WHERE id=? AND status='RUNNING'",id);
  if(changed==0){IngestionRun run=find(id);throw new ServiceException(IngestionErrorCode.INGESTION_NOT_CANCELLABLE,"Run is not cancellable in state "+run.status());}
  return find(id);
 }
 public IngestionRun find(UUID id){return jdbc.query("SELECT * FROM ingestion_runs WHERE id=?",(r,n)->new IngestionRun(
   r.getObject("id",UUID.class),IngestionRun.Type.valueOf(r.getString("run_type")),IngestionRun.Status.valueOf(r.getString("status")),
   r.getTimestamp("window_start").toInstant(),r.getTimestamp("window_end").toInstant(),r.getTimestamp("started_at").toInstant(),
   r.getTimestamp("finished_at")==null?null:r.getTimestamp("finished_at").toInstant(),r.getLong("records_read"),r.getLong("events_written"),
   r.getString("error_code"),r.getString("error_message")),id).stream().findFirst().orElseThrow(()->new ServiceException(IngestionErrorCode.INGESTION_RUN_NOT_FOUND,"Ingestion run was not found"));}
 public List<IngestionRun> history(int limit){if(limit<1||limit>200)throw new ServiceException(IngestionErrorCode.INGESTION_INVALID_RANGE,"limit must be 1 to 200");return jdbc.query("SELECT id FROM ingestion_runs ORDER BY started_at DESC LIMIT ?",(r,n)->find(r.getObject(1,UUID.class)),limit);}
 private long persist(UUID runId,ElexonClient.Price p,UUID correlation)throws Exception{
  Instant now=Instant.now(clock); UUID eventId=UUID.randomUUID(); var payload=new MarketPriceRevisionV1(p.provider(),p.start(),p.end(),p.value(),p.revision(),p.sourceTime(),now);
  var event=new EventEnvelope<>(eventId,"market-price.revised",1,now,"market-ingestion-service",correlation,null,eventId,p.start().toString(),payload);
  return db(()->jdbc.update("INSERT INTO market_price_events(event_id,run_id,provider,delivery_start,delivery_end,price_gbp_mwh,source_revision,source_timestamp,payload,occurred_at) VALUES(?,?,?,?,?,?,?,?,?::jsonb,?) ON CONFLICT(provider,delivery_start,source_revision) DO NOTHING",
    eventId,runId,p.provider(),Timestamp.from(p.start()),Timestamp.from(p.end()),p.value(),p.revision(),Timestamp.from(p.sourceTime()),json.writeValueAsString(event),Timestamp.from(now)));
 }
 private boolean cancelRequested(UUID id){return Boolean.TRUE.equals(jdbc.queryForObject("SELECT status='CANCEL_REQUESTED' FROM ingestion_runs WHERE id=?",Boolean.class,id));}
 private void finish(UUID id,String status,long read,long written,String code,String message){db(()->jdbc.update("UPDATE ingestion_runs SET status=?,finished_at=?,records_read=?,events_written=?,error_code=?,error_message=? WHERE id=?",status,Timestamp.from(Instant.now(clock)),read,written,code,message,id));}
 private void fail(UUID id,String code,Exception e){finish(id,"FAILED",0,0,code,truncate(e.getMessage()));}
 private String truncate(String s){if(s==null)return null;return s.length()>500?s.substring(0,500):s;}
 private void validate(IngestionRun.Type type,Instant from,Instant to){if(from==null||to==null||!from.isBefore(to)||to.isAfter(Instant.now(clock).plus(1,ChronoUnit.MINUTES))||(type==IngestionRun.Type.BACKFILL&&Duration.between(from,to).compareTo(maxBackfill)>0))throw new ServiceException(IngestionErrorCode.INGESTION_INVALID_RANGE,"Range must be ordered, not in the future, and within the configured backfill limit");}
 private int db(DbCall call){for(int attempt=1;;attempt++){try{return call.run();}catch(TransientDataAccessException e){if(attempt==dbRetryAttempts)throw e;try{Thread.sleep(dbRetryBaseDelayMs*attempt);}catch(InterruptedException x){Thread.currentThread().interrupt();throw e;}}catch(Exception e){if(e instanceof RuntimeException r)throw r;throw new IllegalStateException(e);}}}
 @FunctionalInterface private interface DbCall{int run()throws Exception;}
}
