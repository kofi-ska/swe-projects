package com.powerliq.ingestion;
import com.powerliq.api.contracts.ApiError;import com.powerliq.api.contracts.ServiceException;
import java.time.Clock;import java.time.Instant;import java.util.List;import java.util.UUID;
import org.springframework.http.ResponseEntity;import org.springframework.web.bind.annotation.*;
@RestControllerAdvice public class IngestionExceptionHandler {
 private final Clock clock;public IngestionExceptionHandler(Clock clock){this.clock=clock;}
 @ExceptionHandler(ServiceException.class)ResponseEntity<ApiError> service(ServiceException e){return ResponseEntity.status(e.descriptor().httpStatus()).body(new ApiError(e.descriptor().code(),e.getMessage(),"market-ingestion-service",UUID.randomUUID().toString(),Instant.now(clock),e.descriptor().retryable(),List.of()));}
 @ExceptionHandler(Exception.class)ResponseEntity<ApiError> unexpected(){return ResponseEntity.internalServerError().body(new ApiError("INTERNAL_ERROR","An unexpected error occurred","market-ingestion-service",UUID.randomUUID().toString(),Instant.now(clock),false,List.of()));}
}
