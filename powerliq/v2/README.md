# PowerLiq V2

PowerLiq V2 is the distributed, event-driven evolution of PowerLiq. AI is outside its scope.

This initial increment establishes the Maven reactor and extracts the deterministic V1 financial engine into the framework-independent `financial-core` library. The production and test sources are copied unchanged so V2 begins with the financial behavior already verified in V1.

## Modules

- `libraries/financial-core`: value types, settlement-interval rules, valuation, collateral, liquidity and scenario calculations.

## Build

Java 21 is required.

```shell
./mvnw test
```

Service modules, event contracts, messaging infrastructure and deployment definitions will be introduced in later increments.
