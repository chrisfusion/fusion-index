# Stage 1: Build
FROM golang:1.25-alpine AS build
WORKDIR /build

COPY go.mod go.sum ./
# Dependencies are vendored (offline builds): no `go mod download`.
COPY vendor/ vendor/

COPY cmd/ cmd/
COPY internal/ internal/
COPY migrations/ migrations/
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o fusion-index ./cmd/server

# Stage 2: Runtime
FROM alpine:3.19
WORKDIR /app

RUN apk add --no-cache ca-certificates postgresql16-client

COPY --from=build /build/fusion-index .
COPY --from=build /build/migrations ./migrations

EXPOSE 8080

ENV DB_HOST=localhost
ENV DB_PORT=5432
ENV DB_NAME=fusion_index
ENV DB_USERNAME=fusion
ENV DB_PASSWORD=fusion
ENV STORAGE_BACKEND=FILESYSTEM
ENV STORAGE_FS_ROOT=/data/artifacts

ENTRYPOINT ["./fusion-index"]
