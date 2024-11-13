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
RUN apk add --no-cache git tzdata

ENV SERVICE=dashbrr

WORKDIR /src

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

COPY . ./
COPY --from=web-builder /app/web/dist ./web/dist

ARG VERSION=dev
ARG REVISION=dev
ARG BUILDTIME
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

RUN --network=none --mount=target=. \
    export GOOS=$TARGETOS; \
    export GOARCH=$TARGETARCH; \
    [[ "$GOARCH" == "amd64" ]] && export GOAMD64=$TARGETVARIANT; \
    [[ "$GOARCH" == "arm" ]] && [[ "$TARGETVARIANT" == "v6" ]] && export GOARM=6; \
    [[ "$GOARCH" == "arm" ]] && [[ "$TARGETVARIANT" == "v7" ]] && export GOARM=7; \
    echo $GOARCH $GOOS $GOARM$GOAMD64; \
    go build -ldflags "-s -w \
    -X github.com/autobrr/dashbrr/internal/buildinfo.Version=${VERSION} \
    -X github.com/autobrr/dashbrr/internal/buildinfo.Commit=${REVISION} \
    -X github.com/autobrr/dashbrr/internal/buildinfo.Date=${BUILDTIME}" \
    -o /out/bin/dashbrr cmd/dashbrr/main.go

# build runner
FROM alpine:latest

LABEL org.opencontainers.image.source="https://github.com/autobrr/dashbrr"
LABEL org.opencontainers.image.licenses="GPL-2.0-or-later"
LABEL org.opencontainers.image.base.name="alpine:latest"

COPY --link --from=app-builder /out/bin/dashbrr /usr/local/bin/dashbrr
EXPOSE 8080
