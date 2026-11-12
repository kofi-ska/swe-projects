package com.powerliq.api;

import com.powerliq.application.PositionService;
import com.powerliq.application.PortfolioNotFoundException;
import com.powerliq.config.PowerLiqProperties;
import jakarta.validation.Valid;
import java.net.URI;
import java.util.List;
import java.util.UUID;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping(ApiRoutes.V1_POSITIONS)
public class PositionController {
    private final PositionService service;
    private final PowerLiqProperties properties;

    public PositionController(PositionService service, PowerLiqProperties properties) {
        this.service = service;
        this.properties = properties;
    }

    @PostMapping
    public ResponseEntity<PositionResponse> create(
            @RequestHeader("Idempotency-Key") String idempotencyKey,
            @Valid @RequestBody PositionRequest request) {
        if (!properties.portfolio().id().equals(request.portfolioId())) {
            throw new PortfolioNotFoundException(request.portfolioId());
        }
        PositionResponse response = PositionResponse.from(service.create(idempotencyKey, request.toCommand()));
        return ResponseEntity.created(URI.create(ApiRoutes.V1_POSITIONS + "/" + response.id())).body(response);
    }

    @GetMapping
    public List<PositionResponse> list() {
        return service.findAll().stream().map(PositionResponse::from).toList();
    }

    @PostMapping("/{positionId}/settlement")
    public PositionResponse settle(@PathVariable UUID positionId) {
        return PositionResponse.from(service.settle(positionId));
    }
}
