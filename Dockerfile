# Stage 1: Maven build
FROM maven:3.9-eclipse-temurin-21 AS build
WORKDIR /build

# Cache dependencies first
COPY pom.xml .
RUN --mount=type=cache,id=fusion-index-m2,target=/root/.m2 \
    mvn -f pom.xml dependency:go-offline -q 2>/dev/null || true

COPY src ./src
RUN --mount=type=cache,id=fusion-index-m2,target=/root/.m2 \
    mvn -f pom.xml package -DskipTests -q

# Stage 2: Runtime
FROM eclipse-temurin:21-jre AS runtime
WORKDIR /app

COPY --from=build /build/target/quarkus-app/lib/          ./lib/
COPY --from=build /build/target/quarkus-app/*.jar         ./
COPY --from=build /build/target/quarkus-app/app/          ./app/
COPY --from=build /build/target/quarkus-app/quarkus/      ./quarkus/

EXPOSE 8080

ENV JAVA_OPTS="-Djava.util.logging.manager=org.jboss.logmanager.LogManager"
ENV DB_HOST=localhost
ENV DB_PORT=5432
ENV DB_NAME=fusion_index
ENV DB_USERNAME=fusion
ENV DB_PASSWORD=fusion
ENV STORAGE_BACKEND=FILESYSTEM

ENTRYPOINT ["sh", "-c", "java $JAVA_OPTS -jar quarkus-run.jar"]
