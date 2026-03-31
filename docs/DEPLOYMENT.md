# Deployment

## Prerequisites

- macOS with Homebrew
- Go 1.25+
- Node.js 20+ and npm
- `cloudflared` (`brew install cloudflared`)
- A configured Cloudflare tunnel (`~/.cloudflared/`)

## Setup

### 1. Install dependencies

```bash
make deps
```

### 2. Configure environment

```bash
cp .env.example .env
chmod 600 .env
```

Create `web/.env.local` with `POLYMR_USER` and `POLYMR_PASS` matching your `.env` values.

See `.env.example` for all available variables. Key additions beyond the basics:

| Variable | Purpose |
|----------|---------|
| `SLACK_OWNER_ID` | Your Slack user ID (e.g., `U0AG558DLQ6`). Used to tag you in scheduled agent results. |
| `ELEVENLABS_API_KEY` | Required for voice chat (STT + TTS). |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Required for Google Calendar sync. |

### 3. Build

```bash
make build               # Go binary -> bin/polymr
cd web && npm run build   # Next.js -> web/.next/
```

## Services

Three macOS LaunchAgents run in `~/Library/LaunchAgents/`:

| Service | Plist | Port |
|---------|-------|------|
| Go router | `com.polymr.router.plist` | 8080 |
| Next.js frontend | `com.polymr.frontend.plist` | 3000 |
| Cloudflare tunnel | `com.cloudflare.cloudflared.plist` | -- |

All are configured with `RunAtLoad` and `KeepAlive` (auto-start on login, auto-restart on crash).

The `install-services.sh` script bakes environment variables (including `SLACK_OWNER_ID`) into the router plist so they're available to the Go process.

### Load services

```bash
UID_NUM=$(id -u)
launchctl bootstrap gui/$UID_NUM ~/Library/LaunchAgents/com.polymr.router.plist
launchctl bootstrap gui/$UID_NUM ~/Library/LaunchAgents/com.polymr.frontend.plist
launchctl bootstrap gui/$UID_NUM ~/Library/LaunchAgents/com.cloudflare.cloudflared.plist
```

### Restart everything

```bash
./scripts/restart.sh
```

Rebuilds Go + Next.js, then bounces all services.

### Restart a single service

```bash
UID_NUM=$(id -u)
launchctl bootout gui/$UID_NUM ~/Library/LaunchAgents/com.polymr.router.plist
launchctl bootstrap gui/$UID_NUM ~/Library/LaunchAgents/com.polymr.router.plist
```

### Check status

```bash
launchctl list | grep -E "polymr|cloudflared"
```

Exit code `0` in the second column = running.

## Persistent Data

The router stores state in `~/.config/winston/`:

| File | Contents | Survives restart? |
|------|----------|------------------|
| `sessions.json` | Active agent sessions (keyed by Slack thread TS) | Yes |
| `schedules.json` | All scheduled agent runs with cron patterns | Yes |

These files are created automatically on first use. Deleting them resets all sessions/schedules.

## Logs

```bash
tail -f ~/Library/Logs/polymr.err.log                    # Go router
tail -f ~/Library/Logs/polymr-frontend.err.log            # Next.js
tail -f ~/Library/Logs/com.cloudflare.cloudflared.err.log  # Tunnel
tail -f ~/Library/Logs/polymr-audit.log                   # Audit (JSON)
```

## Cloudflare Tunnel

Config at `~/.cloudflared/config.yml`:

```yaml
tunnel: <tunnel-uuid>
credentials-file: ~/.cloudflared/<tunnel-uuid>.json

ingress:
  - hostname: personal-api.polymr.io
    service: http://localhost:8080
  - hostname: personal.polymr.io
    service: http://localhost:8080
  - service: http_status:404
```

Both subdomains route to the Go router, which handles host-based routing internally.

## Updating Secrets

1. Update `.env`, `web/.env.local`, and `com.polymr.router.plist`
2. Restart the router service

Never commit `.env`, `.env.local`, or plist files to git.

## Ops Notifications

The router posts to Slack on key lifecycle events (startup, shutdown, frontend down/recovered, model changes, prompt changes). Set `SLACK_NOTIFY_CHANNEL` in `.env` to enable. If unset, events are logged locally only.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| Cloudflare error 1033 | `cloudflared` not running. Check logs and restart. |
| 502 Bad Gateway | Go router crashed. Check `polymr.err.log`, rebuild, restart. |
| Frontend 500 | Stale build. Run `cd web && npm run build`, restart frontend. |
| Slack not responding | Check `SLACK_BOT_TOKEN` and `SLACK_SIGNING_SECRET`. Look for HMAC errors in logs. |
| 429 Too Many Requests | Rate limit: 100 req/min API, 10 req/min auth. |
| Schedules lost | Check `~/.config/winston/schedules.json` exists and is readable. |
| Scheduled runs not tagging you | Set `SLACK_OWNER_ID` in `.env` and reinstall services. |
| `missing_scope` errors in logs | Bot may lack `channels:join` scope but is already in the channel — this is tolerated. |
