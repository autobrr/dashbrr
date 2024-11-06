# build web
FROM --platform=$BUILDPLATFORM node:22.10.0-alpine3.20 AS web-builder
RUN corepack enable

WORKDIR /web

COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY src/ ./src/
COPY public/ ./public/
COPY index.html ./
COPY tsconfig.json tsconfig.node.json tsconfig.app.json vite.config.ts ./
COPY postcss.config.js tailwind.config.js ./
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
# Copy the built web assets to the backend/web/dist directory for embedding
RUN mkdir -p backend/web
COPY --from=web-builder /web/dist ./backend/web/dist

# Build with embedded assets
RUN go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${REVISION} -X main.date=${BUILDTIME}" -o bin/dashbrr backend/main.go

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
