package com.powerliq.portfolio;

import jakarta.validation.Valid;
import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;
import java.math.BigDecimal;
import java.net.URI;
import java.util.List;
import java.util.UUID;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

@RestController
@RequestMapping("/api/v2/portfolios")
public class PortfolioController {
    private final PortfolioService service;
    public PortfolioController(PortfolioService service) { this.service = service; }
    public record Create(@NotNull UUID id, @NotBlank @Size(max=120) String name,
                         @NotNull @DecimalMin("0") BigDecimal availableCashGbp,
                         @NotNull @DecimalMin("0") BigDecimal liquidityBufferGbp) { }
    public record Settings(@NotNull @DecimalMin("0") BigDecimal availableCashGbp,
                           @NotNull @DecimalMin("0") BigDecimal liquidityBufferGbp,
                           long expectedVersion) { }
    @PostMapping public ResponseEntity<Portfolio> create(@Valid @RequestBody Create request,
            @RequestHeader(value="X-Correlation-ID", required=false) UUID correlationId) {
        Portfolio result = service.create(request.id(), request.name(), request.availableCashGbp(),
                request.liquidityBufferGbp(), correlation(correlationId));
        return ResponseEntity.created(URI.create("/api/v2/portfolios/" + result.id())).body(result);
    }
    @PutMapping("/{id}/liquidity-settings") public Portfolio settings(@PathVariable UUID id,
            @Valid @RequestBody Settings request, @RequestHeader(value="X-Correlation-ID", required=false) UUID correlationId) {
        return service.updateSettings(id, request.availableCashGbp(), request.liquidityBufferGbp(),
                request.expectedVersion(), correlation(correlationId));
    }
    @GetMapping("/{id}") public Portfolio find(@PathVariable UUID id) { return service.find(id); }
    @GetMapping public List<Portfolio> list(@RequestParam(defaultValue="50") int limit,
            @RequestParam(defaultValue="0") int offset) { return service.list(limit, offset); }
    private UUID correlation(UUID supplied) { return supplied == null ? UUID.randomUUID() : supplied; }
}
