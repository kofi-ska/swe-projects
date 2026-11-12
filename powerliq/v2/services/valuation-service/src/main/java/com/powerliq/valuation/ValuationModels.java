package com.powerliq.valuation;
import com.powerliq.financial.PositionSide;import java.math.BigDecimal;import java.time.*;import java.util.*;
public final class ValuationModels{private ValuationModels(){}
 public record PositionInput(UUID id,UUID portfolioId,PositionSide side,BigDecimal contractPriceGbpPerMWh,BigDecimal volumeMWh,LocalDate settlementDate,int settlementPeriod,Instant createdAt){}
 public record PriceInput(String provider,LocalDate settlementDate,int settlementPeriod,BigDecimal priceGbpPerMWh,Instant sourceTimestamp,Instant retrievedAt){}
 public record Request(UUID portfolioId,Instant asOf,List<PositionInput> positions,List<PriceInput> prices){}
 public record PositionResult(UUID positionId,BigDecimal mtmGbp,BigDecimal collateralGbp){}
 public record Result(UUID id,UUID portfolioId,Instant asOf,Instant calculatedAt,String inputHash,List<PositionResult> positions,BigDecimal portfolioMtmGbp,BigDecimal estimatedCollateralGbp){}
}
