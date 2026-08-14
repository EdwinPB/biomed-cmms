# Build stage: compile the requested binary (api, migrate, or seed) from the
# Go workspace. The image is shared by the api service and the one-off
# migrate/seed services via the BINARY build arg.
FROM golang:1.26-alpine AS builder

ARG BINARY=api

WORKDIR /src

# Go workspace layout: go.work at the repo root references ./apps/api.
COPY go.work ./
COPY apps/api/ apps/api/

WORKDIR /src/apps/api
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/${BINARY} ./cmd/${BINARY}

# Runtime stage: minimal Alpine image, non-root user. Includes wget for the
# Compose healthcheck against /health.
FROM alpine:3.20

ARG BINARY=api

RUN apk add --no-cache ca-certificates wget \
    && addgroup -S app && adduser -S -G app app

COPY --from=builder /out/${BINARY} /out/${BINARY}

# The API serves plain HTTP; TLS termination is delegated to a reverse proxy.
EXPOSE 8080

# Config comes from the environment. Migrations are applied separately by the
# migrate command and are never run automatically by this image.
# ARG is not expanded inside ENTRYPOINT, so mirror it into ENV (ENV
# substitution does happen at build time) and exec the binary.
ARG BINARY=api
ENV BINARY=${BINARY}

USER app

ENTRYPOINT ["/bin/sh", "-c", "exec /out/${BINARY} \"$@\"", "api"]
