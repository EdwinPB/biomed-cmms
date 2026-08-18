# Root Dockerfile for Render deployment.
# Render uses the Dockerfile's directory as build context, so the existing
# apps/api/Dockerfile cannot reach go.work at the repo root. This file
# copies the full workspace and delegates to the same build logic.
FROM golang:1.26-alpine AS builder

ARG BINARY=api

WORKDIR /src
COPY . .

WORKDIR /src/apps/api
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/${BINARY} ./cmd/${BINARY}

FROM alpine:3.20

ARG BINARY=api

RUN apk add --no-cache ca-certificates wget \
    && addgroup -S app && adduser -S -G app app

COPY --from=builder /out/${BINARY} /out/${BINARY}

EXPOSE 8080

ARG BINARY=api
ENV BINARY=${BINARY}

USER app

ENTRYPOINT ["/bin/sh", "-c", "exec /out/${BINARY} \"$@\"", "api"]
