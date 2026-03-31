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
          Agent Manager (in-memory + disk persistence)
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
4. **Rate Limiting** — 100 req/min per IP (API), 10 req/min per IP (auth endpoints)
5. **CORS** — restricted to `https://personal.polymr.io`
6. **Auth** — Basic Auth or Slack HMAC depending on route
7. **Audit Logging** — JSON log of all authenticated requests

### API routes

```
GET  /health                                  (public, no auth)
POST /slack/commands                           (Slack HMAC verified)
POST /slack/events                             (Slack HMAC verified)
POST /slack/interactions                       (Slack HMAC verified)
GET  /api/agents                               (Basic Auth + audit)
GET  /api/agents/{agent}                       (Basic Auth + audit)
POST /api/agents/{agent}/run                   (Basic Auth + audit)
PUT  /api/agents/{agent}/model                 (Basic Auth + audit)
PUT  /api/agents/{agent}/prompt                (Basic Auth + audit)
GET  /api/agents/{agent}/sessions/{session}    (Basic Auth + audit)
POST /api/agents/{agent}/sessions/{session}/message  (Basic Auth + audit)
GET  /api/schedules                            (Basic Auth + audit)
POST /api/schedules                            (Basic Auth + audit)
POST /api/schedules/sync-calendar              (Basic Auth + audit)
PUT  /api/schedules/{id}                       (Basic Auth + audit)
DELETE /api/schedules/{id}                     (Basic Auth + audit)
POST /api/voice/transcribe                     (Basic Auth + audit)
POST /api/voice/synthesize                     (Basic Auth + audit)
GET  /api/kali/status                          (Basic Auth + audit)
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
2. Handler checks `subtype == ""` (ignores bot edits, `message_changed` events, etc.)
3. Looks up session by thread timestamp
4. Spawns `claude --resume <session_id>` to continue the conversation
5. Same streaming update cycle as above

If no session exists for that thread, the bot posts a helpful notice ("No active session in this thread. Start a new conversation with a slash command.").

### Security

- **HMAC-SHA256 verification** — every Slack request is signed. The handler verifies the signature using the `SLACK_SIGNING_SECRET` before processing.
- **Timestamp validation** — requests older than 5 minutes are rejected (replay protection)
- **Bot loop prevention** — messages from bots (including itself) are ignored
- **Subtype filtering** — only plain user messages (`subtype == ""`) trigger agent responses, preventing loops from `message_changed` and other system events

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
max_turns: 50
---

You are a marketing agent. You have access to...
(system prompt body)
```

The Go router loads all agent files at startup. Each agent becomes:
- A Slack slash command (`/marketing`)
- API endpoints (`/api/agents/marketing/run`, `/api/agents/marketing`)
- A card on the web dashboard
- An individual chat page (`/agents/marketing`)

### Frontmatter fields

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `name` | Yes | — | Agent ID. Becomes the slash command, API route, and dashboard card. Lowercase, no spaces. Prefix with `workspace-` for grouping (e.g., `codephil-research`). |
| `description` | No | — | One-line description shown on dashboard and in Slack. |
| `model` | No | `sonnet` | Claude model: `opus` (Opus 4.6), `sonnet` (Sonnet 4.6), or `haiku` (Haiku 4.5). |
| `timeout` | No | `600` | Max execution time in seconds. |
| `max_turns` | No | `25` | Max conversation turns per run. |

### Workspace grouping

Agent names with a hyphen are auto-grouped by workspace prefix on the dashboard:

- `codephil-research` -> workspace `codephil`, short name `research`
- `codephil-youtube` -> workspace `codephil`, short name `youtube`
- `rivalytics-social` -> workspace `rivalytics`, short name `social`
- `winston` -> no workspace (top-level)

The dashboard shows a workspace dropdown to filter agents by workspace, and arranges agents within each workspace in a logical pipeline order (research -> director -> assets -> deliver).

### Tool auto-detection

The router scans each agent's system prompt for keywords and automatically tags detected tools:

| Keyword match | Tool tag |
|---------------|----------|
| `web search`, `web_search` | Web Search |
| `web fetch`, `web_fetch` | Web Fetch |
| `git`, `github`, `repository` | Git |
| `figma` | Figma |
| `google workspace`, `google calendar`, `gmail` | Google Workspace |
| `slack` | Slack |
| `youtube` | YouTube Data |
| `image gen`, `nano banana`, `thumbnail` | Image Gen |
| `playwright`, `browser` | Playwright |
| `security`, `pentest`, `nmap`, `exploit` | Security Tools |
| `remotion`, `video` | Remotion |
| `manim`, `animation` | Manim |
| `trend`, `analytics` | Trend Analysis |
| `sub-agent`, `spawn agent`, `delegate` | Sub-Agents |
| `schedule`, `cron` | Scheduling |

Detected tools appear as icon badges on agent cards in the dashboard, with a clickable overflow for agents with many tools.

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
- **`--dangerously-skip-permissions`:** required for headless (non-interactive) operation
- **Model override:** a prompt starting with `opus:` or `sonnet:` overrides the agent's default model
- **Timeout:** configurable per-agent (default 10 minutes)

### Sessions

Sessions are persisted to `~/.config/winston/sessions.json`, keyed by Slack thread timestamp:

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
- Sessions survive router restarts (loaded from disk on startup)
- A session stub is stored *before* the agent run starts, so thread follow-ups can find the session even if the initial run fails

### Model switching

Agent models can be changed at runtime:
- **Web UI:** model switcher on the agent chat page (Haiku 4.5 / Sonnet 4.6 / Opus 4.6)
- **API:** `PUT /api/agents/{agent}/model` with `{"model": "opus"}`

Changing the model updates the agent's `.md` file on disk, posts a notification to Slack, and triggers a service restart so the router picks up the change.

### System prompt editing

System prompts can be edited at runtime:
- **Web UI:** inline editor with markdown preview on the dashboard
- **API:** `PUT /api/agents/{agent}/prompt` with `{"system_prompt": "..."}`

Saves to the agent's `.md` file on disk, posts a notification to Slack, and triggers a restart.

### Input sanitization

All user input (from Slack or API) is sanitized before reaching Claude:
- **Max length:** 4,000 characters
- **Prompt injection detection:** 13 regex patterns are stripped (e.g., "ignore previous instructions", "jailbreak", "DAN mode")

---

## Scheduled Agent Runs

Agents can run on a cron schedule via the `/schedules` page or API:

```json
{
  "agent_id": "marketing",
  "prompt": "Generate weekly competitor report",
  "cron": "0 9 * * 1",
  "slack_channel": "marketing-reports",
  "timezone": "America/Denver"
}
```

The Go router runs a cron scheduler (`robfig/cron`) in-process. When a schedule fires:

1. Posts a trigger message to the configured Slack channel using `PostMessageTS` (returns message timestamp for threading)
2. Spawns the agent with `SpawnAgentInThreadStreaming`, posting the result as a **thread reply** to the trigger message
3. Tags the owner (`<@SLACK_OWNER_ID>`) in the response so they get a notification
4. Session is stored so the owner can reply in the thread to continue the conversation

### Persistence

Schedules are persisted to `~/.config/winston/schedules.json` and restored on router startup. Creating, editing, or deleting a schedule saves the file immediately.

### Editing schedules

Schedules can be updated via `PUT /api/schedules/{id}` with partial updates (cron, prompt, timezone, agent_id, slack_channel). The old cron entry is removed and a new one registered with the updated configuration.

### Google Calendar sync

`POST /api/schedules/sync-calendar` spawns the winston agent with Google Workspace MCP tools to create or update Google Calendar events for all active schedules. Events are titled `[Agent] <agent_id>` with RRULE recurrence derived from the cron expression.

---

## Frontend (Web Dashboard)

Next.js 16 + React 19 + Tailwind CSS 4. Runs on `localhost:3000`, only accessible through the Go router's reverse proxy (never directly from the internet).

### Design system

The frontend uses a dark, glass-morphism design language:

- **CSS tokens:** `--surface-0` through `--surface-3` (layered dark backgrounds), `--border`, `--accent` (indigo), `--glow`
- **Glass cards:** `.glass-card` and `.glass-card-hover` — translucent backgrounds with backdrop blur and subtle borders
- **Noise texture:** `.noise-bg` — SVG fractal noise overlay at low opacity
- **Ambient orbs:** Gradient blur circles for subtle depth
- **Custom scrollbar:** Thin, translucent, rounded
- **Skeleton loading:** `.skeleton` — shimmer animation for loading states
- **Gradient text:** `.gradient-text` and `.gradient-text-accent` for headings

### Pages

| Page | URL | What it does |
|------|-----|-------------|
| **Dashboard** | `/` | Workspace-grouped agent cards with tool icons, model badges, system prompt viewer/editor, health status. Workspace dropdown filters agents by prefix. |
| **Agent Chat** | `/agents/[slug]` | Chat interface with model switcher (Haiku 4.5 / Sonnet 4.6 / Opus 4.6), health status dot, service restart awareness. |
| **Voice Chat** | `/voice` | Push-to-talk voice interface with agent selector. Audio -> ElevenLabs STT -> agent -> ElevenLabs TTS -> audio playback. Gradient mic button with pulse ring animation. |
| **Schedules** | `/schedules` | List and calendar views. Create/edit/delete schedules with cron builder, timezone picker, agent selector. Google Calendar sync button. Visual time grid (6am-11pm). |

### Dashboard features

- **Workspace navigation:** dropdown with workspace avatars (colored initials), checkmarks for active workspace
- **Agent cards:** name, description, model badge (color-coded), tool icons (inline SVGs for Git, Figma, Slack, YouTube, Google, etc.)
- **Tool overflow:** clickable `+N` badge that expands a popover showing all remaining tools
- **System prompt viewer:** markdown-rendered preview with edit toggle, saves via API
- **Health card:** uptime, agent count, active sessions, active schedules

### Auth

The frontend prompts for username/password on first visit, stores credentials in the browser, and sends them as `Authorization: Basic` headers on every API request.

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
| `/api/*` | 100 req/min per IP | General abuse prevention |
| Auth endpoints | 10 req/min per IP | Brute-force protection |

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
| **Router started** | Router started on `hostname` (includes restart reason if available) |
| **Router shutting down** | Router shutting down — SIGTERM / SIGINT / error |
| **Frontend unreachable** | Frontend (localhost:3000) is unreachable (debounced, max once per 5 min) |
| **Frontend recovered** | Frontend recovered (sent once when it comes back) |
| **Model changed** | Agent `X` model changed from `Y` to `Z` |
| **Prompt changed** | Agent `X` system prompt updated |

If `SLACK_NOTIFY_CHANNEL` is not set, notifications are logged locally only. A breadcrumb file (`/tmp/winston-restart-reason`) is written before restarts and read on next startup to report the reason.

---

## External Services

| Service | Used for | Auth method |
|---------|---------|-------------|
| **Claude CLI** | Agent execution | Logged-in CLI session |
| **Slack API** | Send/edit/delete messages, join channels, threading | Bot token (`xoxb-...`) |
| **ElevenLabs** | Voice chat (text-to-speech, speech-to-text) | API key |
| **Cloudflare** | Tunnel + DNS + DDoS + optional Access | Tunnel credentials |
| **Google Workspace** | Calendar sync, email, docs (via MCP) | OAuth client credentials |
| **Kali VM** (optional) | Pentester agent SSH access | SSH key |
| **YouTube Data API** (optional) | YouTube agent research | API key |
| **Nano Banana** (optional) | Thumbnail image generation | API key |

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
SLACK_OWNER_ID=U0123456789         # User ID to tag in scheduled run results
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

### Persistent data

| File | Contents |
|------|----------|
| `~/.config/winston/sessions.json` | Active agent sessions (keyed by Slack thread TS) |
| `~/.config/winston/schedules.json` | All scheduled agent runs (restored on startup) |

---

## Project Structure

```
winston/
├── cmd/polymr/main.go              # Entry point — starts HTTP server
├── internal/
│   ├── agents/manager.go           # Agent loading, execution, sessions, schedules (~1200 LOC)
│   ├── notify/notify.go            # Ops notifications (startup, shutdown, model/prompt changes)
│   ├── router/
│   │   ├── router.go               # Host-based routing, middleware, calendar sync handler (~630 LOC)
│   │   ├── auth.go                 # Basic Auth middleware
│   │   ├── ratelimit.go            # Token bucket rate limiter
│   │   └── audit.go                # JSON audit logging
│   ├── slack/
│   │   ├── handler.go              # Slash commands, events, interactions, streaming updater
│   │   ├── client.go               # Slack API wrapper (PostMessage, PostMessageTS, PostThreadReply)
│   │   └── verify.go               # HMAC-SHA256 request verification
│   ├── sanitize/sanitize.go        # Input length + prompt injection filtering
│   ├── voice/elevenlabs.go         # ElevenLabs TTS/STT
│   └── kali/ssh.go                 # Kali VM SSH connectivity
├── web/                            # Next.js frontend
│   └── src/app/
│       ├── page.tsx                # Dashboard — workspace nav, agent cards, tool icons (~1400 LOC)
│       ├── agents/[slug]/page.tsx  # Agent chat with model switcher
│       ├── voice/page.tsx          # Voice chat with agent selector
│       ├── schedules/page.tsx      # Schedule manager — list + calendar views
│       └── globals.css             # Design tokens, glass cards, noise texture, markdown styles
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

- **Single machine** — no clustering, failover, or replication.
- **No agent sandboxing** — agents run with your full user permissions.
- **Slack message size** — responses are truncated to 3,000 characters (Slack's limit).
- **No queue** — concurrent agent requests each spawn a Claude CLI process. Heavy load = heavy CPU/memory.
- **No auth on frontend beyond Basic Auth** — no per-user roles or permissions.
