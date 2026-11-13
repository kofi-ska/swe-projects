package com.powerliq.position.api;

import com.powerliq.position.application.PositionApplicationService;
import jakarta.servlet.http.HttpServletRequest;
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
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v2/positions")
public class PositionController {
    private final PositionApplicationService service;

    public PositionController(PositionApplicationService service) {
        this.service = service;
    }

    @PostMapping
    public ResponseEntity<PositionResponse> create(
            @RequestHeader(value = "Idempotency-Key", required = false) String idempotencyKey,
            @Valid @RequestBody CreatePositionRequest request,
            HttpServletRequest httpRequest) {
        PositionResponse response = PositionResponse.from(service.create(
                idempotencyKey, correlationId(httpRequest), request.toCommand()));
        return ResponseEntity.created(URI.create("/api/v2/positions/" + response.id())).body(response);
    }

    @GetMapping("/{positionId}")
    public PositionResponse find(@PathVariable UUID positionId) {
        return PositionResponse.from(service.findById(positionId));
    }

    @GetMapping
    public List<PositionResponse> list(
            @RequestParam(defaultValue = "50") int limit,
            @RequestParam(defaultValue = "0") int offset) {
        return service.findAll(limit, offset).stream().map(PositionResponse::from).toList();
    }

    @PostMapping("/{positionId}/settlement")
    public PositionResponse settle(@PathVariable UUID positionId, HttpServletRequest request) {
        return PositionResponse.from(service.settle(positionId, correlationId(request)));
    }

    private UUID correlationId(HttpServletRequest request) {
        return (UUID) request.getAttribute(CorrelationIdFilter.ATTRIBUTE);
    }
}
