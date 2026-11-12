package com.powerliq.api;

import com.powerliq.application.MarketDataService;
import com.powerliq.marketdata.MarketDataCheck;
import java.util.Map;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping(ApiRoutes.V1_MARKET_DATA)
public class MarketDataController {
    private final MarketDataService service;

    public MarketDataController(MarketDataService service) {
        this.service = service;
    }

    @GetMapping("/status")
    public MarketDataCheck status() {
        return service.check();
    }

    @PostMapping("/sync")
    public Map<String, Integer> sync() {
        return Map.of("recordsProcessed", service.syncActivePositions());
    }
}
