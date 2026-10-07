# Multi-stage build from source: web → app → runtime (design §14).
# For the goreleaser pipeline see Dockerfile.release (same runtime layout,
# packages the prebuilt binary instead).

# ---- stage 1: frontend ----
FROM node:24-alpine AS web
WORKDIR /build
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- stage 2: backend ----
FROM golang:1.26-alpine AS app
ARG VERSION=dev
# override with --build-arg GOPROXY=https://goproxy.cn,direct when needed
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY web/embed.go web/embed.go
COPY --from=web /build/dist web/dist
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/runemarket ./cmd/runemarket

# ---- stage 3: runtime ----
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --chmod=755 --from=app /out/runemarket /usr/local/bin/runemarket
WORKDIR /app
VOLUME /app/data
EXPOSE 8080
ENTRYPOINT ["runemarket"]
