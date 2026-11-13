package com.powerliq.ingestion;
import jakarta.validation.Valid;
import jakarta.validation.constraints.NotNull;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import org.springframework.web.bind.annotation.*;
@RestController @RequestMapping("/api/v2/ingestion-runs")
public class IngestionController {
 private final IngestionService service; public IngestionController(IngestionService service){this.service=service;}
 public record Backfill(@NotNull Instant from,@NotNull Instant to){}
 @PostMapping("/backfills") public IngestionRun backfill(@Valid @RequestBody Backfill r,@RequestHeader(value="X-Correlation-ID",required=false)UUID correlation){return service.run(IngestionRun.Type.BACKFILL,r.from(),r.to(),correlation==null?UUID.randomUUID():correlation);}
 @PostMapping("/{id}/cancellation") public IngestionRun cancel(@PathVariable UUID id){return service.cancel(id);}
 @GetMapping("/{id}") public IngestionRun find(@PathVariable UUID id){return service.find(id);}
 @GetMapping public List<IngestionRun> history(@RequestParam(defaultValue="50")int limit){return service.history(limit);}
}
