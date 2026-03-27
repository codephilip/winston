# Polymr

Personal multi-agent hub. Talk to specialized AI agents through Slack or the web dashboard.

## Agents

- **Marketing** — competitive intelligence, SEO, content generation
- **Pentester** — authorized security testing via Kali VM
- **YouTube** — trend research, script writing, SEO optimization
- **Voice** — speech-to-text and text-to-speech via ElevenLabs

## Architecture

```
Browser / Slack
  |  HTTPS
Cloudflare (DDoS, TLS, Access MFA)
  |  Encrypted tunnel
Go Router (:8080)
  |- /slack/*   Slack webhooks (HMAC verified)
  |- /api/*     Agent API (Basic Auth)
  |- /health    Health check
  \- /*         Next.js frontend (:3000)
```

## Quick Start

```bash
make deps                 # install Go + npm dependencies
cp .env.example .env      # configure credentials
make build                # build Go binary
cd web && npm run build   # build frontend
make run                  # start the router
```

For development with hot reload:

```bash
make dev       # Go router (terminal 1)
make frontend  # Next.js dev server (terminal 2)
```

## Docs

- [Deployment](docs/DEPLOYMENT.md) — setup, services, operations
- [Security](docs/SECURITY.md) — threat model, security layers, hardening
