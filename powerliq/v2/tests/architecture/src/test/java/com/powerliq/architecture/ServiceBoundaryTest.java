package com.powerliq.architecture;

import static com.tngtech.archunit.lang.syntax.ArchRuleDefinition.noClasses;

import com.tngtech.archunit.core.domain.JavaClasses;
import com.tngtech.archunit.core.importer.ClassFileImporter;
import java.util.List;
import org.junit.jupiter.api.Test;

class ServiceBoundaryTest {
    private static final List<String> SERVICE_PACKAGES = List.of(
            "com.powerliq.position..",
            "com.powerliq.portfolio..",
            "com.powerliq.ingestion..",
            "com.powerliq.marketdata..",
            "com.powerliq.valuation..");

    private final JavaClasses classes = new ClassFileImporter().importPackages("com.powerliq");

    @Test
    void servicesDoNotImportPeerServices() {
        for (String owner : SERVICE_PACKAGES) {
            String[] peers = SERVICE_PACKAGES.stream().filter(candidate -> !candidate.equals(owner))
                    .toArray(String[]::new);
            noClasses().that().resideInAPackage(owner)
                    .should().dependOnClassesThat().resideInAnyPackage(peers)
                    .check(classes);
        }
    }
}
