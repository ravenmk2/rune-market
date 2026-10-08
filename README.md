# RuneMarket

[![Test](https://github.com/ravenmk2/rune-market/actions/workflows/test.yml/badge.svg)](https://github.com/ravenmk2/rune-market/actions/workflows/test.yml)
[![Release](https://img.shields.io/github/v/release/ravenmk2/rune-market)](https://github.com/ravenmk2/rune-market/releases)
[![License](https://img.shields.io/github/license/ravenmk2/rune-market)](LICENSE)

Marketplace for Agent Skills and DESIGN.md.

## QuickStart

Run with Docker:

```bash
docker run -d --name rune-market \
  --restart unless-stopped \
  -p 8080:8080 \
  -v rune-market-data:/app/data \
  ghcr.io/ravenmk2/rune-market:latest
```

Then open http://localhost:8080 — the setup wizard will walk you through creating the admin account and initial settings.
