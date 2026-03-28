# Winston v1 Architecture

A self-hosted multi-agent system that runs Claude CLI agents on your machine and exposes them via Slack, a web dashboard, and REST APIs. Agents inherit your full local environment (shell, SSH keys, files, tools) — this is the feature, not a bug.

---

## How Everything Fits Together

```
                        Internet
                           |
                    Cloudflare Edge
                  (DDoS, TLS, Access)
                           |
                  cloudflared daemon          <-- outbound-only tunnel
                  (runs on your Mac)              (no open ports)
                           |
               Go Router (localhost:8080)
              /        |            \
     /slack/*      /api/*        /* (frontend)
        |             |               |
  Slack HMAC     Basic Auth     reverse proxy
   verified       required            |
        |             |        Next.js (localhost:3000)
        \             |              /
         \            |             /
          Agent Manager (in-memory)
                  |
           Claude CLI subprocess
          (claude --print --model ...)
                  |
         Your full environment
      (files, shell, SSH, APIs, etc.)
```

---

## Services

Three processes run as macOS LaunchAgents (or Linux systemd units), all with auto-restart on crash:

| Service | Port | What it does |
|---------|------|-------------|
| **Go Router** (`bin/polymr`) | 8080 | HTTP server — routes requests, runs agents, talks to Slack API |
| **Next.js Frontend** (`web/`) | 3000 | Web dashboard — agent chat, voice, schedules. Only accessible via the Go router's reverse proxy |
| **Cloudflare Tunnel** (`cloudflared`) | none | Connects your machine to Cloudflare's edge. Outbound-only — no ports are opened on your machine |

### How the tunnel works

Your machine is **not directly reachable from the internet**. Instead:

1. `cloudflared` opens a persistent **outbound** connection to Cloudflare
2. DNS for `personal.polymr.io` and `personal-api.polymr.io` points to Cloudflare
3. When someone hits those domains, Cloudflare forwards the request down the tunnel to `localhost:8080`
4. The Go router processes it and sends the response back up the tunnel

Think of it as your machine calling Cloudflare and saying "send me any requests for these domains." No one on the internet can connect to your machine directly — they only talk to Cloudflare.

---

## Request Routing

The Go router splits traffic by hostname:

| Hostname | Where it goes | Auth required |
|----------|--------------|---------------|
| `personal.polymr.io` | Next.js frontend (proxied to `:3000`) | Basic Auth (except static assets under `/_next/`) |
| `personal-api.polymr.io` | API + Slack endpoints | Basic Auth for `/api/*`, HMAC for `/slack/*` |
| `localhost` | API + Slack endpoints (same as above) | Same |

### Middleware stack (applied in order)

1. **Logger** — access log to stderr
2. **Panic Recovery** — catches Go panics, returns 500
3. **Security Headers** — XSS, CSP, clickjacking, MIME sniffing protection
4. **Rate Limiting** — 30 req/min per IP (API), 15 req/min per IP (auth endpoints)
5. **CORS** — restricted to `https://personal.polymr.io`
6. **Auth** — Basic Auth or Slack HMAC depending on route
7. **Audit Logging** — JSON log of all authenticated requests

### API routes

```
GET  /health                         (public, no auth)
POST /slack/commands                  (Slack HMAC verified)
POST /slack/events                    (Slack HMAC verified)
POST /slack/interactions              (Slack HMAC verified)
GET  /api/agents                      (Basic Auth + audit)
POST /api/agents/{agent}/run          (Basic Auth + audit)
GET  /api/agents/{agent}/sessions/*   (Basic Auth + audit)
POST /api/agents/{agent}/sessions/*/send  (Basic Auth + audit)
GET  /api/schedules                   (Basic Auth + audit)
POST /api/schedules                   (Basic Auth + audit)
DELETE /api/schedules/{id}            (Basic Auth + audit)
POST /api/voice/transcribe            (Basic Auth + audit)
POST /api/voice/synthesize            (Basic Auth + audit)
GET  /api/kali/status                 (Basic Auth + audit)
```

---

## Slack Integration

Slack communicates via **HTTP webhooks** (not Socket Mode). Slack POSTs to your Cloudflare domain, which tunnels the request to your Go router.

### Three webhook endpoints

| Endpoint | Slack feature | What triggers it |
|----------|--------------|-----------------|
| `/slack/commands` | Slash commands | User types `/marketing analyze competitors` |
| `/slack/events` | Event subscriptions | User @mentions the bot or replies in a thread |
| `/slack/interactions` | Interactive components | User clicks a button in a bot message |

### Slash command flow

```
User types:  /marketing analyze competitors
                    |
Slack POSTs to /slack/commands
                    |
Router verifies HMAC-SHA256 signature
                    |
Handler responds immediately (echoes the command to channel)
                    |
        [async goroutine starts]
                    |
Finds the command message in Slack, posts "_thinking..._" in thread
                    |
Spawns: claude --print --output-format stream-json --model sonnet \
        --system-prompt <marketing agent prompt> "analyze competitors"
                    |
Every ~2 seconds, edits the Slack message with latest output
                    |
Final result posted to thread. Session saved (keyed by thread timestamp).
```

### Thread replies (conversation continuity)

When a user replies in a thread that has an active session:

1. Slack POSTs to `/slack/events` with `thread_ts`
2. Handler looks up session by thread timestamp
3. Spawns `claude --resume <session_id>` to continue the conversation
4. Same streaming update cycle as above

If no session exists for that thread, the reply is ignored (with a cleanup of any placeholder messages).

### Security

- **HMAC-SHA256 verification** — every Slack request is signed. The handler verifies the signature using the `SLACK_SIGNING_SECRET` before processing.
- **Timestamp validation** — requests older than 5 minutes are rejected (replay protection)
- **Bot loop prevention** — messages from bots (including itself) are ignored

---

## Agent System

### How agents are defined

Each agent is a Markdown file in `~/.claude/agents/` with YAML frontmatter:

```markdown
---
name: marketing
description: Marketing intelligence and content generation
model: sonnet
timeout: 600
---

You are a marketing agent. You have access to...
(system prompt body)
```

The Go router loads all agent files at startup. Each agent becomes:
- A Slack slash command (`/marketing`)
- An API endpoint (`/api/agents/marketing/run`)
- A card on the web dashboard

### How agents execute

Agents run as **Claude CLI subprocesses** on your machine:

```bash
claude --print \
  --output-format stream-json \
  --dangerously-skip-permissions \
  --model sonnet \
  --system-prompt "You are a marketing agent..." \
  "analyze competitors"
```

Key details:
- **Working directory:** `$HOME` (so Claude picks up `~/.claude/` config, agents, skills)
- **Full environment access:** the subprocess inherits your PATH, SSH keys, API tokens, everything
- **`--dangerously-skip-permissions`:** required for headless (non-interactive) operation. This means the agent can read/write files, run commands, etc. without asking for confirmation
- **Model override:** a prompt starting with `opus:` or `sonnet:` overrides the agent's default model
- **Timeout:** configurable per-agent (default 10 minutes)

### Sessions

Sessions are stored **in-memory** in the Go process, keyed by Slack thread timestamp:

```
Thread TS "1774709768.109529" -> Session {
    ClaudeSessionID: "e93f1c15-54f8-42dc-8b2d-04cda9a47b0c"
    AgentID:         "marketing"
    SlackChannel:    "C0APB6KTUPL"
    LastUsed:        2026-03-28T09:30:12Z
}
```

- New slash command -> new session
- Thread reply -> resume session via `claude --resume`
- Router restart -> all sessions lost (by design — keeps things simple)

### Input sanitization

All user input (from Slack or API) is sanitized before reaching Claude:
- **Max length:** 4,000 characters
- **Prompt injection detection:** 13 regex patterns are stripped (e.g., "ignore previous instructions", "jailbreak", "DAN mode")

---

## Frontend (Web Dashboard)

Next.js 16 + React 19 + Tailwind CSS 4. Runs on `localhost:3000`, only accessible through the Go router's reverse proxy (never directly from the internet).

### Pages

| Page | URL | What it does |
|------|-----|-------------|
| **Dashboard** | `/` | Grid of agent cards. Click to open chat. |
| **Agent Chat** | `/agents/[slug]` | Chat interface for a specific agent. Sends prompts to `/api/agents/{slug}/run`. |
| **Voice Chat** | `/voice` | Hold-to-talk voice interface. Audio -> ElevenLabs STT -> agent -> ElevenLabs TTS -> audio playback. |
| **Schedules** | `/schedules` | Cron schedule builder. Create/delete scheduled agent runs with optional Slack output. |

### Auth

The frontend prompts for username/password on first visit, stores credentials in the browser, and sends them as `Authorization: Basic` headers on every API request.

---

## Scheduled Agent Runs

Agents can run on a cron schedule via the `/schedules` page or API:

```json
{
  "agent_id": "marketing",
  "prompt": "Generate weekly competitor report",
  "cron": "0 9 * * 1",
  "slack_channel": "marketing-reports"
}
```

The Go router runs a cron scheduler (`robfig/cron`) in-process. When a schedule fires:
1. Spawns the agent with the configured prompt
2. Captures the result
3. Posts to the configured Slack channel (if set)

Schedules are stored in-memory and lost on restart (same as sessions).

---

## Security Architecture

### Network layer

```
Internet -> Cloudflare Edge -> Cloudflare Tunnel -> localhost:8080
```

- **Zero open ports** on your machine
- **Outbound-only tunnel** — your machine calls out to Cloudflare, never the reverse
- **DDoS protection** at Cloudflare's edge
- **TLS termination** at Cloudflare (HTTPS everywhere)

### Authentication layers

| Layer | Protects | How it works |
|-------|----------|-------------|
| **Cloudflare Access** (optional) | `personal.polymr.io` | Email OTP verification before the request even reaches your machine |
| **Basic Auth** | `/api/*` and frontend | Username/password with constant-time comparison (SHA256 + `subtle.ConstantTimeCompare`) |
| **Slack HMAC** | `/slack/*` | HMAC-SHA256 signature verification + 5-minute timestamp window |

### Rate limiting

| Endpoint group | Limit | Purpose |
|----------------|-------|---------|
| `/api/*` | 30 req/min per IP | General abuse prevention |
| Auth endpoints | 15 req/min per IP | Brute-force protection |

IP is extracted from `Cf-Connecting-Ip` (Cloudflare header), falling back to `X-Forwarded-For`.

### Audit logging

All authenticated requests and failed auth attempts are logged as JSON to `~/Library/Logs/polymr-audit.log`:

```json
{"timestamp":"2026-03-28T09:30:12Z","ip":"203.0.113.42","user":"pg","method":"POST","path":"/api/agents/marketing/run","status":200,"user_agent":"Mozilla/5.0..."}
```

### Security headers

Applied to all responses: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `X-XSS-Protection`, `Referrer-Policy: strict-origin-when-cross-origin`, `Content-Security-Policy`.

### What's NOT sandboxed

Claude CLI runs as **your user** with **full environment access**. This is intentional — agents need to read files, run commands, SSH into servers, etc. The trust boundary is:
- You trust the people who can trigger agents (authenticated Slack users, dashboard users)
- You trust that input sanitization catches obvious prompt injection
- You accept that a sufficiently clever prompt could still make an agent do something unexpected

---

## Ops Notifications

The router posts to Slack (`SLACK_NOTIFY_CHANNEL`) on key events:

| Event | Message |
|-------|---------|
| **Router started** | Router started on `hostname` |
| **Router shutting down** | Router shutting down — SIGTERM / SIGINT / error |
| **Frontend unreachable** | Frontend (localhost:3000) is unreachable (debounced, max once per 5 min) |
| **Frontend recovered** | Frontend recovered (sent once when it comes back) |

If `SLACK_NOTIFY_CHANNEL` is not set, notifications are logged locally only.

---

## External Services

| Service | Used for | Auth method |
|---------|---------|-------------|
| **Claude CLI** | Agent execution | Logged-in CLI session |
| **Slack API** | Send/edit/delete messages, join channels | Bot token (`xoxb-...`) |
| **ElevenLabs** | Voice chat (text-to-speech, speech-to-text) | API key |
| **Cloudflare** | Tunnel + DNS + DDoS + optional Access | Tunnel credentials |
| **Kali VM** (optional) | Pentester agent SSH access | SSH key |
| **YouTube Data API** (optional) | YouTube agent research | API key |
| **Nano Banana** (optional) | Thumbnail image generation | API key |
| **Google Workspace** (optional) | Marketing agent (email, docs, calendar) | OAuth client credentials |

---

## Configuration

### Environment variables (`.env`)

**Required:**
```bash
PORT=8080
POLYMR_USER=youruser
POLYMR_PASS=yourpassword
SLACK_BOT_TOKEN=xoxb-...
SLACK_SIGNING_SECRET=...
```

**Optional:**
```bash
SLACK_NOTIFY_CHANNEL=C0123456789   # Ops notifications channel
ELEVENLABS_API_KEY=...
ELEVENLABS_VOICE_ID=...
KALI_VM_HOST=...
KALI_VM_USER=...
KALI_VM_SSH_KEY=~/.ssh/kali_vm
YOUTUBE_API_KEY=...
NANO_BANANA_API_KEY=...
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
AUDIT_LOG_PATH=...
```

### Cloudflare Tunnel (`~/.cloudflared/config.yml`)

```yaml
tunnel: <uuid>
credentials-file: ~/.cloudflared/<uuid>.json

ingress:
  - hostname: personal-api.polymr.io
    service: http://localhost:8080
  - hostname: personal.polymr.io
    service: http://localhost:8080
  - service: http_status:404
```

---

## Project Structure

```
winston/
├── cmd/polymr/main.go              # Entry point — starts HTTP server
├── internal/
│   ├── agents/manager.go           # Agent loading, Claude CLI execution, sessions, schedules
│   ├── notify/notify.go            # Ops notifications (startup, shutdown, frontend down)
│   ├── router/
│   │   ├── router.go               # Host-based routing, middleware stack
│   │   ├── auth.go                 # Basic Auth middleware
│   │   ├── ratelimit.go            # Token bucket rate limiter
│   │   └── audit.go                # JSON audit logging
│   ├── slack/
│   │   ├── handler.go              # Slash commands, events, interactions
│   │   ├── client.go               # Slack API wrapper
│   │   └── verify.go               # HMAC-SHA256 request verification
│   ├── sanitize/sanitize.go        # Input length + prompt injection filtering
│   ├── scheduler/scheduler.go      # Cron-based scheduled execution
│   ├── voice/elevenlabs.go         # ElevenLabs TTS/STT
│   └── kali/ssh.go                 # Kali VM SSH connectivity
├── web/                            # Next.js frontend
│   └── src/app/
│       ├── page.tsx                # Dashboard (agent cards)
│       ├── agents/[slug]/page.tsx  # Agent chat UI
│       ├── voice/page.tsx          # Voice chat
│       └── schedules/page.tsx      # Schedule manager
├── scripts/
│   ├── install-services.sh         # Generate + load launchd/systemd services
│   ├── uninstall-services.sh       # Remove services
│   └── restart.sh                  # Rebuild + restart everything
├── docs/                           # You are here
├── .env                            # Environment config (git-ignored)
├── Makefile                        # build, run, test, install-services, etc.
└── go.mod                          # Go dependencies
```

---

## Operations Quick Reference

```bash
# Check service status
launchctl list | grep winston

# View logs
tail -f ~/Library/Logs/winston-router.err.log     # Go router
tail -f ~/Library/Logs/winston-frontend.err.log    # Next.js
tail -f ~/Library/Logs/polymr-audit.log            # Audit trail

# Rebuild and restart everything
./scripts/restart.sh

# Install/reinstall services (after changing .env)
make install-services

# Run tests
make test
```

---

## Known Limitations (v1)

- **Sessions are in-memory** — lost on router restart. No persistent conversation history.
- **Schedules are in-memory** — lost on restart. Must be recreated.
- **Single machine** — no clustering, failover, or replication.
- **No agent sandboxing** — agents run with your full user permissions.
- **Slack message size** — responses are truncated to 3,000 characters (Slack's limit).
- **No queue** — concurrent agent requests each spawn a Claude CLI process. Heavy load = heavy CPU/memory.
