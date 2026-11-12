# syntax=docker/dockerfile:1.7
ARG SERVICE
FROM maven:3.9.11-eclipse-temurin-21-alpine AS build
ARG SERVICE
WORKDIR /workspace
COPY pom.xml mvnw mvnw.cmd ./
COPY .mvn .mvn
COPY contracts contracts
COPY libraries libraries
COPY services services
COPY tests tests
RUN ./mvnw -q -pl "services/${SERVICE}" -am -DskipTests package
RUN cp "services/${SERVICE}/target/${SERVICE}-0.1.0-SNAPSHOT.jar" /workspace/application.jar

FROM eclipse-temurin:21-jre-alpine
RUN addgroup -S powerliq && adduser -S -G powerliq -u 10001 powerliq
WORKDIR /app
COPY --from=build --chown=powerliq:powerliq /workspace/application.jar application.jar
USER 10001
ENV JAVA_TOOL_OPTIONS="-XX:MaxRAMPercentage=70 -XX:+ExitOnOutOfMemoryError -Djava.security.egd=file:/dev/urandom"
EXPOSE 8080
ENTRYPOINT ["java","-jar","/app/application.jar"]
