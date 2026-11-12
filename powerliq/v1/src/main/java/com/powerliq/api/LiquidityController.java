package com.powerliq.api;

import com.powerliq.application.LiquidityApplicationService;
import jakarta.validation.Valid;
import java.util.UUID;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping(ApiRoutes.V1_PORTFOLIOS)
public class LiquidityController {
    private final LiquidityApplicationService service;

    public LiquidityController(LiquidityApplicationService service) {
        this.service = service;
    }

    @GetMapping("/liquidity")
    public LiquidityResponse liquidity(@PathVariable UUID portfolioId) {
        return LiquidityResponse.from(portfolioId, service.liquidity(portfolioId));
    }

    @PostMapping("/scenarios")
    public ScenarioResponse scenario(
            @PathVariable UUID portfolioId,
            @Valid @RequestBody ScenarioRequest request) {
        return ScenarioResponse.from(portfolioId, service.scenario(portfolioId, request.percentageShock()));
    }
}
