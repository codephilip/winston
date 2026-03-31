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
- `/health` is the only public endpoint (returns status, uptime, component counts).

### Rate Limiting

- API: 100 req/min per client IP
- Auth: 10 req/min per IP (brute force protection)
- IP extracted from `Cf-Connecting-Ip` header (real client IP behind Cloudflare)

### Input Sanitization

- 13 prompt injection patterns filtered (e.g., "ignore previous instructions", "DAN mode")
- 4000 character input limit on all Slack and API inputs
- Bot messages (`bot_id` present) and non-plain subtypes are ignored to prevent loops
- Slack message responses truncated to 3000 characters (Slack's limit)

### Agent Containment

- Configurable per-agent execution timeout (default 10 minutes)
- Configurable per-agent turn limit (default 25 turns)
- `--dangerously-skip-permissions` required for headless operation (single-user only)

### Audit

- JSON append-only audit log at `~/Library/Logs/polymr-audit.log`
- Failed auth attempts logged with IP and attempted username
- Security headers: X-Content-Type-Options, X-Frame-Options, X-XSS-Protection, Referrer-Policy, CSP
- Ops notifications to Slack on startup, shutdown, frontend status changes, model/prompt changes

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
| `SLACK_OWNER_ID` | `.env`, router plist |
| `ELEVENLABS_API_KEY` | `.env`, router plist |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | `.env`, router plist |
| `YOUTUBE_API_KEY` | `.env`, router plist |
| `NANO_BANANA_API_KEY` | `.env`, router plist |
| `KALI_VM_SSH_KEY` | `~/.ssh/kali_vm` |
| Cloudflare tunnel credentials | `~/.cloudflared/` |

### Rotation

1. **Slack:** api.slack.com/apps > OAuth & Permissions > Regenerate. Basic Information > Regenerate signing secret.
2. **ElevenLabs:** elevenlabs.io > Settings > API Keys > Regenerate.
3. **Google:** console.cloud.google.com > Credentials > Regenerate client secret.
4. Update `.env`, plist, and restart services.

## File Permissions

| File | Permissions |
|------|-------------|
| `.env` | `600` |
| `com.polymr.router.plist` | `600` |
| `~/.cloudflared/*.json` | `600` |
| `~/.config/winston/*.json` | `600` |
| FileVault disk encryption | ON |
