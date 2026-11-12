package com.powerliq.marketdata;import com.powerliq.api.contracts.ErrorDescriptor;
public enum MarketDataErrorCode implements ErrorDescriptor{
 MARKET_DATA_INVALID_RANGE(400,false),MARKET_PRICE_NOT_FOUND(404,false),EVENT_TYPE_UNSUPPORTED(400,false),EVENT_INVALID(400,false),MARKET_DATA_PERSISTENCE_FAILED(503,true);
 private final int status;private final boolean retryable;MarketDataErrorCode(int s,boolean r){status=s;retryable=r;}public String code(){return name();}public int httpStatus(){return status;}public boolean retryable(){return retryable;}
}
