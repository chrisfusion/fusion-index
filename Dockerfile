# Stage 1: Build
FROM golang:1.25-alpine AS build
WORKDIR /build

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o fusion-index ./cmd/server

# Stage 2: Runtime
FROM alpine:3.19
WORKDIR /app

RUN apk add --no-cache ca-certificates

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
