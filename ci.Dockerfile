# build web
FROM --platform=$BUILDPLATFORM node:22.10.0-alpine3.20 AS web-builder
RUN corepack enable

WORKDIR /app/web

COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY web/ ./
RUN pnpm run build

# build app
FROM --platform=$BUILDPLATFORM golang:1.23-alpine3.20 AS app-builder

ARG VERSION=dev
ARG REVISION=dev
ARG BUILDTIME
ARG TARGETOS
ARG TARGETARCH

# Install minimal build dependencies
RUN apk add --no-cache \
    git \
    tzdata

ENV SERVICE=dashbrr
ENV CGO_ENABLED=0

# Set cross-compilation flags
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
# Copy the built web assets to the web/dist directory for embedding
COPY --from=web-builder /app/web/dist ./web/dist

# Build with embedded assets
RUN go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${REVISION} -X main.date=${BUILDTIME}" -o bin/dashbrr cmd/dashbrr/main.go

# build runner
FROM --platform=$TARGETPLATFORM alpine:latest

LABEL org.opencontainers.image.source="https://github.com/autobrr/dashbrr"

ENV HOME="/config" \
    XDG_CONFIG_HOME="/config" \
    XDG_DATA_HOME="/config"

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

VOLUME /config

COPY --from=app-builder /src/bin/dashbrr /usr/local/bin/

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/dashbrr"]
