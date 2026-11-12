package com.powerliq.api;

public final class ApiRoutes {
    public static final String API_ROOT = "/api";
    public static final String V1 = API_ROOT + "/v1";
    public static final String V1_POSITIONS = V1 + "/positions";
    public static final String V1_MARKET_DATA = V1 + "/market-data";
    public static final String V1_PORTFOLIOS = V1 + "/portfolios/{portfolioId}";

    private ApiRoutes() {
    }
}
