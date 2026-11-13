package com.powerliq.marketdata;
import com.powerliq.events.*;import java.time.Instant;import java.util.List;import org.springframework.http.ResponseEntity;import org.springframework.web.bind.annotation.*;
@RestController public class MarketDataController{
 private final MarketDataService service;public MarketDataController(MarketDataService service){this.service=service;}
 @PostMapping("/internal/v2/events/market-price-revised")public ResponseEntity<Void> consume(@RequestBody EventEnvelope<MarketPriceRevisionV1> event){return service.consume(event)?ResponseEntity.accepted().build():ResponseEntity.noContent().build();}
 @GetMapping("/api/v2/market-prices/latest")public MarketPrice latest(@RequestParam String provider,@RequestParam Instant deliveryStart){return service.latest(provider,deliveryStart);}
 @GetMapping("/api/v2/market-prices")public List<MarketPrice> history(@RequestParam String provider,@RequestParam Instant from,@RequestParam Instant to,@RequestParam(defaultValue="500")int limit){return service.history(provider,from,to,limit);}
}
