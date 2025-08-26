# syntax=docker/dockerfile:1
FROM golang:1.24 AS builder
ARG VERSION
ARG COMMIT
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -v -ldflags "-X 'main.Version=${VERSION}' -X 'main.Commit=${COMMIT}'" -o acm-secret-discovery-server .

FROM alpine:latest
LABEL org.opencontainers.image.authors="Sixt SE"
LABEL org.opencontainers.image.source=https://github.com/Sixt/acm-secret-discovery-server
LABEL org.opencontainers.image.description="ACM Secret Discovery Server is an Envoy SDS interface server that provides TLS certificates to Envoy proxy using AWS Certificate Manager (ACM)."
LABEL org.opencontainers.image.licenses="Apache-2.0"
WORKDIR /app
COPY --from=builder /app/acm-secret-discovery-server .
USER nobody

ENTRYPOINT ["/app/acm-secret-discovery-server"]
