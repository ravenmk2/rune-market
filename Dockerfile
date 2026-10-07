# Release image: copies the goreleaser-built binary, never recompiles (§14).
FROM alpine:3.21

RUN apk add --no-cache ca-certificates

COPY runemarket /usr/local/bin/runemarket
RUN chmod +x /usr/local/bin/runemarket

WORKDIR /app
VOLUME /app/data
EXPOSE 8080

ENTRYPOINT ["runemarket"]
