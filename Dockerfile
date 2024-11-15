# build web
FROM node:22.10.0-alpine3.20 AS web-builder
RUN corepack enable

WORKDIR /app/web

COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY web/ ./
RUN pnpm run build

# build app
FROM golang:1.23-alpine3.20 AS app-builder

ARG VERSION=dev
ARG REVISION=dev
ARG BUILDTIME

RUN apk add --no-cache git build-base tzdata

ENV SERVICE=dashbrr
ENV CGO_ENABLED=0

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
# Copy the built web assets to the web/dist directory for embedding
COPY --from=web-builder /app/web/dist ./web/dist

# Build with embedded assets
RUN go build -ldflags "-s -w \
    -X github.com/autobrr/dashbrr/internal/buildinfo.Version=${VERSION} \
    -X github.com/autobrr/dashbrr/internal/buildinfo.Commit=${REVISION} \
    -X github.com/autobrr/dashbrr/internal/buildinfo.Date=${BUILDTIME}" \
    -o /app/dashbrr cmd/dashbrr/main.go

# build runner
FROM alpine:3.20

LABEL org.opencontainers.image.source="https://github.com/autobrr/dashbrr"

ENV HOME="/config" \
    XDG_CONFIG_HOME="/config" \
    XDG_DATA_HOME="/config"

WORKDIR /config
VOLUME /config

COPY --from=app-builder /app/dashbrr /usr/local/bin/dashbrr

EXPOSE 8080

RUN addgroup -S dashbrr && \
    adduser -S dashbrr -G dashbrr

USER dashbrr

ENTRYPOINT ["dashbrr"]
