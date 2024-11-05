# build web
FROM node:20.17.0-alpine3.20 AS web-builder
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
# Copy the built web assets to the backend/web/dist directory for embedding
RUN mkdir -p backend/web
COPY --from=web-builder /web/dist ./backend/web/dist

# Build with embedded assets
RUN go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${REVISION} -X main.date=${BUILDTIME}" -o /app/dashbrr backend/main.go

# build runner
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.source="https://github.com/autobrr/dashbrr"

ENV HOME="/config" \
    XDG_CONFIG_HOME="/config" \
    XDG_DATA_HOME="/config"

WORKDIR /config
VOLUME /config

COPY --from=app-builder /app/dashbrr /dashbrr

EXPOSE 8080

USER 65532:65532
ENTRYPOINT ["/dashbrr", "--db", "./data/dashbrr.db"]
