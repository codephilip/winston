# Security

## Architecture

```
User (browser/phone)
  |  HTTPS
Cloudflare Edge (DDoS, TLS, Access MFA)
  |  Encrypted tunnel
Go Router (:8080) on Mac Mini
  |- /slack/*   Slack webhooks (HMAC verified)
  |- /api/*     Agent API (Basic Auth + rate limit + audit)
  \- /*         Next.js frontend (Basic Auth)
                  |
                Claude CLI subprocess
```

## Security Layers

### Network

- Zero inbound ports. Cloudflare tunnel is outbound-only.
- All traffic encrypted end-to-end (browser to Cloudflare to tunnel).
- Machine has no public IP. Not scannable.

### Authentication

- **Cloudflare Access** on `personal.polymr.io` requires email OTP before any page load (24h sessions).
- **HTTP Basic Auth** on all `/api/*` and frontend routes. Constant-time comparison via `crypto/subtle`.
- **Slack webhooks** bypass Basic Auth but are verified by HMAC-SHA256 signing secret with 5-minute replay protection.
- `/health` is the only public endpoint (returns `{"status":"ok"}`).

### Rate Limiting

- API: 30 req/min per client IP
- Auth: 15 req/min per IP (brute force protection)
- IP extracted from `Cf-Connecting-Ip` header (real client IP behind Cloudflare)

### Input Sanitization

- 13 prompt injection patterns filtered (e.g., "ignore previous instructions", "DAN mode")
- 4000 character input limit on all Slack inputs
- Bot messages (`bot_id` present) are ignored to prevent loops

### Agent Containment

- 5-minute execution timeout per agent run
- 25 turn limit per run
- `--dangerously-skip-permissions` required for headless operation (single-user only)

### Audit

- JSON append-only audit log at `~/Library/Logs/polymr-audit.log`
- Failed auth attempts logged with IP and attempted username
- Security headers: X-Content-Type-Options, X-Frame-Options, X-XSS-Protection, Referrer-Policy, CSP

## Domain & DNS

- **Registrar:** Hostinger (transfer lock ON, expires 2028)
- **Nameservers:** Cloudflare

### Recommended

- Enable DNSSEC (Cloudflare dashboard, then add DS record at Hostinger)
- Add CAA records restricting cert issuance to Cloudflare's CAs
- Enable 2FA on Cloudflare and Hostinger accounts (authenticator app, not SMS)

## Secrets Management

All secrets live in `.env` and the router LaunchAgent plist. Both are excluded from git.

| Secret | Location |
|--------|----------|
| `POLYMR_USER` / `POLYMR_PASS` | `.env`, `web/.env.local`, router plist |
| `SLACK_BOT_TOKEN` | `.env`, router plist |
| `SLACK_SIGNING_SECRET` | `.env`, router plist |
| `ELEVENLABS_API_KEY` | `.env`, router plist |
| Cloudflare tunnel credentials | `~/.cloudflared/` |

### Rotation

1. **Slack:** api.slack.com/apps > OAuth & Permissions > Regenerate. Basic Information > Regenerate signing secret.
2. **ElevenLabs:** elevenlabs.io > Settings > API Keys > Regenerate.
3. Update `.env`, plist, and restart services.

## File Permissions

| File | Permissions |
|------|-------------|
| `.env` | `600` |
| `com.polymr.router.plist` | `600` |
| `~/.cloudflared/*.json` | `600` |
| FileVault disk encryption | ON |
