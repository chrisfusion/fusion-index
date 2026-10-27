# pg_dump refuses to dump a server with a newer major version, so PG_MAJOR must be
# >= the newest Postgres server this image backs up (older servers are fine).
ARG PG_MAJOR=18

# Stage 1: Build
FROM golang:1.25-trixie AS build
WORKDIR /build

COPY go.mod go.sum ./
# Dependencies are vendored (offline builds): no `go mod download`.
COPY vendor/ vendor/

COPY cmd/ cmd/
COPY internal/ internal/
COPY migrations/ migrations/
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o fusion-index ./cmd/server \
 && mkdir -p /out/data/artifacts

# Stage 2: pg_dump/psql from the official (Debian) postgres image — no PGDG apt repo
# needed. Collects the binaries plus every shared library they link against, except
# glibc itself, which the runtime image (same Debian release) already ships.
FROM postgres:${PG_MAJOR}-trixie AS pgclient
ARG PG_MAJOR
RUN mkdir -p /pgclient/bin /pgclient/lib \
 && cp /usr/lib/postgresql/${PG_MAJOR}/bin/pg_dump /usr/lib/postgresql/${PG_MAJOR}/bin/psql /pgclient/bin/ \
 && ldd /pgclient/bin/* | awk '$3 ~ /^\// {print $3}' | sort -u \
    | grep -vE '/(libc|libm|libresolv|libpthread|libdl|ld-linux[^/]*)\.so' \
    | xargs -I{} cp -L {} /pgclient/lib/ \
 && LD_LIBRARY_PATH=/pgclient/lib /pgclient/bin/pg_dump --version

# Stage 3: Runtime — distroless: glibc + CA certificates, no shell, no package manager.
# Debian 13 (trixie), same release as the postgres image above, so the copied libs
# match its glibc. No shell means no ldconfig: LD_LIBRARY_PATH points at the libs.
# Runs as nonroot (65532); STORAGE_FS_ROOT is pre-created writable for the
# FILESYSTEM backend without a PVC.
FROM gcr.io/distroless/base-debian13:nonroot
WORKDIR /app

COPY --from=pgclient /pgclient/bin/ /usr/local/bin/
COPY --from=pgclient /pgclient/lib/ /usr/local/lib/pgclient/
ENV LD_LIBRARY_PATH=/usr/local/lib/pgclient

COPY --from=build --chown=65532:65532 /out/data/ /data/
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
