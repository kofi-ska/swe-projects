package com.powerliq.ingestion;
import com.powerliq.api.contracts.ErrorDescriptor;
public enum IngestionErrorCode implements ErrorDescriptor {
 INGESTION_INVALID_RANGE(400,false), INGESTION_RUN_NOT_FOUND(404,false), INGESTION_ALREADY_RUNNING(409,true),
 INGESTION_NOT_CANCELLABLE(409,false), ELEXON_TIMEOUT(504,true), ELEXON_UNAVAILABLE(503,true),
 INGESTION_PERSISTENCE_FAILED(503,true);
 private final int status; private final boolean retryable;
 IngestionErrorCode(int status, boolean retryable){this.status=status;this.retryable=retryable;}
 public String code(){return name();} public int httpStatus(){return status;} public boolean retryable(){return retryable;}
}
