# Winston

Your AI agents, running on your machine, accessible from anywhere via Slack.

Winston is a self-hosted multi-agent system. You define AI agents as simple Markdown files, and Winston makes them available as Slack slash commands. Type `/winston help me plan my week` from your phone, and Claude runs on your home Mac with full access to your tools, files, and dev environment.

## Why Self-Host Your Agents?

Every cloud AI platform runs in a sandbox. It can't use your CLI tools, read your local files, SSH into your servers, or run the scripts you've spent years building. You're always copying context in and results out.

Winston inverts this. Your agents run *on your machine* as real Claude CLI processes. They inherit your entire environment — your shell, your PATH, your SSH keys, your databases, your browser, everything. Slack is just the remote control.

**What you need:** A computer that stays on (Mac, Linux box, home server), a [Claude Max subscription](https://claude.ai) (flat-rate, no per-token billing), and a free Slack workspace.

**What you get:** A team of AI agents you can talk to from anywhere — your phone, your laptop, a browser — that can do anything you could do if you were sitting at that computer.

```
Your Phone
  → Slack: "/marketing analyze competitor Acme Corp"
    → Internet (HTTPS)
      → Cloudflare Tunnel (encrypted, no open ports)
        → Your Mac (Go router on :8080)
          → claude --print --model sonnet "analyze competitor Acme Corp"
            → Claude runs with your full environment
              → Response posted back to Slack thread
```

---

## Table of Contents

1. [Prerequisites](#1-prerequisites)
2. [Clone and Install](#2-clone-and-install)
3. [Create Your Agents](#3-create-your-agents)
4. [Configure Environment Variables](#4-configure-environment-variables)
5. [Build and Run Locally](#5-build-and-run-locally)
6. [Create Your Slack App](#6-create-your-slack-app)
7. [Expose Your Machine to the Internet](#7-expose-your-machine-to-the-internet)
8. [Connect Slack to Your Machine](#8-connect-slack-to-your-machine)
9. [Test Everything](#9-test-everything)
10. [Run as a Persistent Service](#10-run-as-a-persistent-service-optional)
11. [Using Winston](#using-winston)
12. [Adding New Agents](#adding-new-agents)
13. [Security Guide](#security)
14. [Troubleshooting](#troubleshooting)

---

## 1. Prerequisites

Install these before starting. If you already have them, skip ahead.

### Required

| Tool | How to Install | Why |
|------|---------------|-----|
| **Go 1.22+** | `brew install go` or [go.dev/dl](https://go.dev/dl/) | Builds the router |
| **Node.js 20+** | `brew install node` or [nodejs.org](https://nodejs.org/) | Builds the web dashboard |
| **Claude CLI** | `npm install -g @anthropic-ai/claude-code` | The AI runtime — your agents are Claude processes |
| **Claude Max subscription** | [claude.ai](https://claude.ai) | Gives your agents model access with no per-token costs |

### Authenticate the Claude CLI

After installing, log in once so the CLI can use your subscription:

```bash
claude
# Follow the prompts to authenticate via browser
# Once done, verify:
claude --version
```

This stores your credentials locally. Every agent Winston spawns will use this authentication.

### Optional

| Tool | Why |
|------|-----|
| **cloudflared** | Expose your machine to the internet without opening ports (needed for Slack outside your LAN) |
| **A domain name** | For production Cloudflare Tunnel setup (not needed for local testing) |
| **ElevenLabs API key** | Voice chat — talk to agents, hear responses spoken |
| **Kali Linux VM** | For the pentester agent to SSH into |

---

## 2. Clone and Install

```bash
git clone https://github.com/YOUR_USER/winston.git
cd winston
make deps
```

This runs `go mod tidy` (Go dependencies) and `cd web && npm install` (frontend dependencies).

---

## 3. Create Your Agents

Agents are Markdown files in `~/.claude/agents/`. The Go router reads this directory on startup and registers each file as an available agent. The filename doesn't matter — the `name` in the frontmatter does.

```bash
mkdir -p ~/.claude/agents
```

Create at least one agent to start with. Here's a general-purpose one:

**`~/.claude/agents/winston.md`**

```markdown
---
name: winston
description: Personal assistant and orchestrator
model: sonnet
---

You are Winston, a personal AI assistant. You have full access to this machine's
file system, CLI tools, and development environment.

Help the user with whatever they ask. Be concise and practical.
```

You can create as many agents as you want. Each one becomes a slash command in Slack:

**`~/.claude/agents/marketing.md`**

```markdown
---
name: marketing
description: Marketing intelligence and content generation
model: sonnet
---

You are a marketing specialist. You handle competitive intelligence, SEO audits,
content generation, campaign analysis, and social media strategy.

When asked to analyze a website, use web search and browsing tools to gather real data.
```

**`~/.claude/agents/pentester.md`**

```markdown
---
name: pentester
description: Authorized security testing
model: opus
---

You are a security testing agent. You perform authorized penetration testing
using tools available on this machine and the Kali VM.

Only test targets the user explicitly authorizes. Document findings clearly.
```

### Agent frontmatter reference

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `name` | Yes | — | The agent ID. This becomes the Slack slash command (`/name`) |
| `description` | No | — | Human-readable description |
| `model` | No | `sonnet` | Claude model: `opus`, `sonnet`, or `haiku` |

The body of the Markdown file (below the `---`) is the agent's system prompt. Write it like you're giving instructions to a new team member.

---

## 4. Configure Environment Variables

```bash
cp .env.example .env
```

Open `.env` and fill in the values. You won't have Slack tokens yet — that's fine, we'll come back to it in Step 6.

```env
# Server
PORT=8080

# Slack (fill in after Step 6)
SLACK_BOT_TOKEN=xoxb-...
SLACK_SIGNING_SECRET=...

# Basic Auth — protects your web dashboard and API
# Pick a username and a strong password
POLYMR_USER=admin
POLYMR_PASS=pick-a-strong-password-here

# --- Everything below is optional ---

# ElevenLabs (for voice chat)
ELEVENLABS_API_KEY=
ELEVENLABS_VOICE_ID=

# Kali VM (for pentester agent)
KALI_VM_HOST=
KALI_VM_USER=
KALI_VM_SSH_KEY=

# YouTube Data API
YOUTUBE_API_KEY=

# Nano Banana (for AI image generation)
NANO_BANANA_API_KEY=
```

Also create the frontend env file:

```bash
cat > web/.env.local << 'EOF'
NEXT_PUBLIC_API_URL=http://localhost:8080
POLYMR_USER=admin
POLYMR_PASS=pick-a-strong-password-here
EOF
```

Use the same `POLYMR_USER` and `POLYMR_PASS` in both files.

---

## 5. Build and Run Locally

```bash
# Build the Go router
make build

# Build the Next.js frontend
cd web && npm run build && cd ..

# Start both (two terminal tabs)
# Tab 1:
make run

# Tab 2:
cd web && npm start
```

Verify it's working:

```bash
curl http://localhost:8080/health
# {"status":"ok"}

# Test with auth (use your POLYMR_USER and POLYMR_PASS)
curl -u admin:pick-a-strong-password-here http://localhost:8080/api/agents
# [{"name":"winston","description":"Personal assistant and orchestrator","model":"sonnet"}, ...]
```

If you see your agents listed, the router is working. You can also open `http://localhost:3000` in a browser to see the dashboard (it will ask for your username/password).

**At this point Winston works locally.** The next steps connect it to Slack so you can use it from anywhere.

---

## 6. Create Your Slack App

Go to **[api.slack.com/apps](https://api.slack.com/apps)** and click **Create New App → From scratch**.

- **App Name:** `Winston` (or whatever you want)
- **Workspace:** Pick your Slack workspace

### 6a. Get your Signing Secret

Go to **Settings → Basic Information**. Under **App Credentials**, copy the **Signing Secret**.

Paste it into your `.env`:

```env
SLACK_SIGNING_SECRET=paste-signing-secret-here
```

### 6b. Set the Bot Display Name

Go to **Features → App Home**. Under **Your App's Presence in Slack**:

- **Display Name:** `Winston`
- **Default Username:** `winston`

This is what shows as the sender name when your bot posts messages.

### 6c. Add OAuth Scopes

Go to **Features → OAuth & Permissions**. Scroll to **Scopes → Bot Token Scopes** and add:

| Scope | What it allows |
|-------|---------------|
| `chat:write` | Post messages to channels |
| `chat:write.customize` | Show "Winston" as the sender name (overrides any cached app name) |
| `commands` | Receive slash commands |
| `app_mentions:read` | Respond when someone @mentions the bot |

### 6d. Install to Workspace

Still on the **OAuth & Permissions** page, click **Install to Workspace** and authorize.

Copy the **Bot User OAuth Token** (starts with `xoxb-`). Paste it into your `.env`:

```env
SLACK_BOT_TOKEN=xoxb-paste-your-token-here
```

### 6e. Create Slash Commands

Go to **Features → Slash Commands**. Create one command for each agent:

Click **Create New Command** for each:

| Command | Request URL | Short Description |
|---------|-------------|-------------------|
| `/winston` | `https://YOUR_DOMAIN/slack/commands` | Talk to Winston |
| `/marketing` | `https://YOUR_DOMAIN/slack/commands` | Marketing agent |
| `/pentester` | `https://YOUR_DOMAIN/slack/commands` | Security testing |
| `/youtube` | `https://YOUR_DOMAIN/slack/commands` | YouTube production |

**All commands point to the same URL.** The router reads the command name and routes to the right agent.

> You don't have `YOUR_DOMAIN` yet — that's Step 7. You can save the commands now with a placeholder URL and update them after setting up the tunnel.

### 6f. Enable Events

Go to **Features → Event Subscriptions**. Toggle **Enable Events** on.

- **Request URL:** `https://YOUR_DOMAIN/slack/events`

Under **Subscribe to bot events**, add:

| Event | Why |
|-------|-----|
| `app_mention` | Respond when someone types @Winston in a channel |
| `message.channels` | Continue conversations in threads (follow-up messages) |

### 6g. Enable Interactivity

Go to **Features → Interactivity & Shortcuts**. Toggle on.

- **Request URL:** `https://YOUR_DOMAIN/slack/interactions`

This enables interactive buttons (e.g., when the YouTube agent offers topic choices).

---

## 7. Expose Your Machine to the Internet

Slack needs to reach your machine over HTTPS. You have three options:

### Option A: Cloudflare Tunnel (Recommended for production)

This is the most secure option. No ports are opened on your machine — Cloudflare creates an outbound-only encrypted tunnel.

**You need:** A domain name with DNS on Cloudflare (free plan works).

```bash
# Install
brew install cloudflare/cloudflare/cloudflared

# Authenticate (opens browser)
cloudflared tunnel login

# Create a tunnel
cloudflared tunnel create winston
```

Note the tunnel ID printed. Create `~/.cloudflared/config.yml`:

```yaml
tunnel: YOUR_TUNNEL_ID
credentials-file: /Users/YOUR_USER/.cloudflared/YOUR_TUNNEL_ID.json

ingress:
  - hostname: winston-api.yourdomain.com
    service: http://localhost:8080
  - service: http_status:404
```

Add DNS and run:

```bash
# Point your subdomain to the tunnel
cloudflared tunnel route dns winston winston-api.yourdomain.com

# Start the tunnel
cloudflared tunnel run winston
```

Your machine is now reachable at `https://winston-api.yourdomain.com`.

#### Securing the dashboard with Cloudflare Access (strongly recommended)

If you want to access the web dashboard over the internet, add a second hostname for it and protect it with Cloudflare Access:

1. Go to **Cloudflare Dashboard → Zero Trust → Access → Applications**
2. Add an application for `winston.yourdomain.com`
3. Set policy: **Allow** → **Emails** → your email address
4. This adds email OTP verification before anyone can see the dashboard

This is on top of the Basic Auth the router already requires.

### Option B: ngrok (Quick testing, no domain needed)

Good for testing. Free tier works but gives you a random URL that changes on restart.

```bash
brew install ngrok
ngrok http 8080
```

ngrok prints a URL like `https://abc123.ngrok-free.app`. Use this as `YOUR_DOMAIN` when configuring Slack commands, events, and interactions.

**Downside:** The URL changes every time you restart ngrok (unless you pay for a fixed domain), so you'll need to update your Slack app settings each time.

### Option C: Local network only (no internet access)

If your phone and computer are on the same network, you can skip tunneling entirely:

1. Find your machine's local IP: `ifconfig | grep "inet " | grep -v 127.0.0.1`
2. Use `http://192.168.x.x:8080` as your base URL

**Downside:** Slack slash commands won't work (Slack's servers can't reach your LAN). But you can still use the web dashboard and API from devices on the same network.

---

## 8. Connect Slack to Your Machine

Now that you have a public URL, go back to your Slack app settings and update the placeholder URLs:

1. **Slash Commands** — edit each command's Request URL:
   `https://YOUR_DOMAIN/slack/commands`

2. **Event Subscriptions** — update the Request URL:
   `https://YOUR_DOMAIN/slack/events`
   (Slack will send a verification challenge — make sure your router is running)

3. **Interactivity** — update the Request URL:
   `https://YOUR_DOMAIN/slack/interactions`

**Restart the router** so it picks up the Slack tokens from `.env`:

```bash
make build && make run
```

---

## 9. Test Everything

Open Slack and type in any channel where the app is installed:

```
/winston hello
```

You should see:
1. A single message from **Winston** saying "_thinking..._"
2. After a few seconds, a reply in the thread with the actual response

Try replying in the thread — Winston remembers the conversation context.

If something goes wrong, see [Troubleshooting](#troubleshooting).

---

## 10. Run as a Persistent Service (Optional)

You probably don't want to keep terminal windows open forever. Here's how to run everything in the background.

### macOS (launchd)

Create two plist files. Replace `/path/to/winston` with your actual clone path.

<details>
<summary><strong>~/Library/LaunchAgents/com.winston.router.plist</strong></summary>

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.winston.router</string>
    <key>ProgramArguments</key>
    <array>
        <string>/path/to/winston/bin/polymr</string>
    </array>
    <key>WorkingDirectory</key>
    <string>/path/to/winston</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PORT</key>
        <string>8080</string>
        <key>SLACK_BOT_TOKEN</key>
        <string>xoxb-your-token</string>
        <key>SLACK_SIGNING_SECRET</key>
        <string>your-signing-secret</string>
        <key>POLYMR_USER</key>
        <string>admin</string>
        <key>POLYMR_PASS</key>
        <string>your-password</string>
        <!-- Add other env vars as needed -->
    </dict>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/winston-router.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/winston-router.err.log</string>
</dict>
</plist>
```

</details>

<details>
<summary><strong>~/Library/LaunchAgents/com.winston.frontend.plist</strong></summary>

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.winston.frontend</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/npm</string>
        <string>start</string>
    </array>
    <key>WorkingDirectory</key>
    <string>/path/to/winston/web</string>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/winston-frontend.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/winston-frontend.err.log</string>
</dict>
</plist>
```

</details>

Load them:

```bash
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.winston.router.plist
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.winston.frontend.plist
```

To restart after a rebuild:

```bash
launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/com.winston.router.plist
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.winston.router.plist
```

### Linux (systemd)

Create `/etc/systemd/system/winston-router.service`:

```ini
[Unit]
Description=Winston Router
After=network.target

[Service]
Type=simple
User=your-user
WorkingDirectory=/path/to/winston
ExecStart=/path/to/winston/bin/polymr
EnvironmentFile=/path/to/winston/.env
Restart=always

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now winston-router
```

Create a similar unit for the frontend (`npm start` in the `web/` directory).

---

## Using Winston

### Slack Commands

Every agent you define becomes a slash command:

```
/winston what files changed in my project this week?
/marketing write 5 tweet variations for our product launch
/pentester run a port scan on 10.0.0.1 (authorized)
/youtube what topics are trending in tech right now?
```

### Threaded Conversations

When you use a slash command, Winston creates a thread. **Reply in the thread** to continue the conversation — the agent remembers everything from the thread.

```
You:      /marketing analyze competitor Acme Corp
Winston:  Here's what I found about Acme Corp... [analysis]
You:      (in thread) Now compare them to our pricing
Winston:  Based on my earlier analysis... [comparison using prior context]
```

### Model Override

Any agent can be temporarily switched to a different model by prefixing the prompt:

```
/winston opus: write a detailed architecture proposal for our new API
/marketing haiku: quick — give me 3 tagline ideas
```

### @Mentions

In any channel where the bot is present, mention it with an agent name:

```
@Winston /marketing what's our SEO score for example.com?
```

### Web Dashboard

Open your frontend URL in a browser. The dashboard shows:

- **Agent cards** — click any agent to start a chat
- **Voice chat** — speak to agents (requires ElevenLabs API key)
- **Schedules** — set up cron jobs (e.g., "run a marketing report every Monday at 9am and post it to #reports")

### REST API

All endpoints require Basic Auth (`-u user:pass`).

```bash
# List agents
curl -u admin:pass http://localhost:8080/api/agents

# Run an agent
curl -X POST -u admin:pass http://localhost:8080/api/agents/winston/run \
  -H 'Content-Type: application/json' \
  -d '{"prompt": "what is in my home directory?"}'

# Create a scheduled run
curl -X POST -u admin:pass http://localhost:8080/api/schedules \
  -H 'Content-Type: application/json' \
  -d '{
    "agent_id": "marketing",
    "cron": "0 9 * * 1",
    "prompt": "weekly SEO report",
    "slack_channel": "#marketing"
  }'

# List / delete schedules
curl -u admin:pass http://localhost:8080/api/schedules
curl -X DELETE -u admin:pass http://localhost:8080/api/schedules/sched_1
```

---

## Why Local Scheduling Beats Claude's Native `/schedule`

Claude Code has a built-in `/schedule` command that creates cloud-hosted cron jobs running on Anthropic's infrastructure. Winston deliberately does **not** use it for its core scheduling, and that's a feature.

| | Winston (local cron) | Claude `/schedule` (cloud) |
|---|---|---|
| **Tool access** | Full — SSH, browser, file system, APIs, your entire Mac | None — isolated cloud sandbox with no local tools |
| **Environment** | Your `.env`, credentials, MCP servers, all skills | Clean room — no secrets, no local context |
| **What it can do** | `ssh kali`, read files, call internal APIs, run scripts | Text generation only |
| **Slack posting** | Direct via your bot token | Requires Slack connector setup |
| **Persistence** | Lives with the process; survives restarts via launchd | Managed by Anthropic |
| **Debugging** | `tail -f /tmp/winston-router.log` | Black box |

The `/schedule` command is great for pure text tasks — summarise this URL every morning, draft a newsletter, remind me about X. But the moment your scheduled job needs to *do* something — check a server, run a scan, pull analytics from an API, push a file — it's useless. It has no access to your machine, your credentials, or your tools.

Winston's local cron runs agents the same way a Slack command does: a full Claude CLI subprocess with your environment, your MCP servers, your SSH keys, your everything. A scheduled pentester scan can actually SSH into Kali. A scheduled marketing report can pull live data from your APIs. A scheduled YouTube agent can query the YouTube Data API and upload a draft.

That's the whole point — **scheduled agents that can act, not just think**.

---

## Adding New Agents

It takes about 2 minutes:

1. **Create the agent file:**

```bash
cat > ~/.claude/agents/researcher.md << 'EOF'
---
name: researcher
description: Deep research on any topic
model: opus
---

You are a research agent. When given a topic, conduct thorough research using
web search, synthesize findings, and present a clear summary with sources.
EOF
```

2. **Add a Slack slash command:**
   Go to your Slack app → **Slash Commands** → **Create New Command**:
   - Command: `/researcher`
   - Request URL: `https://YOUR_DOMAIN/slack/commands`
   - Description: "Deep research agent"

3. **Restart the router** (it loads agents on startup):

```bash
make build && make run
# Or if using launchd:
make build && bash scripts/restart.sh
```

4. **Test it:**

```
/researcher what are the latest developments in quantum computing?
```

That's it. The new agent works in Slack, the API, and the dashboard immediately.

---

## Security

Running AI agents on your personal machine with internet access requires careful security. Here's what Winston does and what you should do.

### What Winston Protects

| Layer | How It Works |
|-------|-------------|
| **No open ports** | Cloudflare Tunnel creates an outbound-only connection. Your machine doesn't listen on any public port. |
| **Slack request verification** | Every webhook from Slack is verified using HMAC-SHA256 with your signing secret. Requests older than 5 minutes are rejected (replay protection). |
| **Basic Auth on API/dashboard** | Every request to `/api/*` and the web dashboard requires a username and password. |
| **Rate limiting** | API: 15 requests/min per IP. Auth endpoints: 5 requests/min per IP. Prevents brute-force and abuse. |
| **Audit logging** | Every authenticated request is logged as JSON with timestamp, IP, user, method, path, and status code. Logged to `~/Library/Logs/polymr-audit.log` (configurable via `AUDIT_LOG_PATH` env var). |
| **Input sanitization** | All user input is truncated to 4,000 characters. Known prompt injection patterns are detected and stripped. |
| **Security headers** | CSP, X-Frame-Options DENY, X-Content-Type-Options nosniff, referrer policy. |

### What You Should Do

| Action | Why |
|--------|-----|
| **Use a strong password** for `POLYMR_USER`/`POLYMR_PASS` | This is your only auth layer on the API. Make it long and random. |
| **Set up Cloudflare Access** on your frontend domain | Adds email-based MFA *before* anyone even sees a login form. Free tier. |
| **Don't commit `.env`** | The `.gitignore` already excludes it. Never push secrets to git. |
| **Restrict agent permissions thoughtfully** | Your agents run with your user permissions. A careless system prompt could let Claude delete files or send emails. Be specific about what each agent should and shouldn't do. |
| **Use `--dangerously-skip-permissions` carefully** | Winston uses this flag so agents can act autonomously. This is the tradeoff: convenience vs. safety. For maximum safety, remove this flag and approve each tool use manually (but this breaks the async Slack flow). |
| **Keep Claude CLI updated** | `npm update -g @anthropic-ai/claude-code` — security fixes ship regularly. |
| **Review audit logs periodically** | `cat ~/Library/Logs/polymr-audit.log | jq .` — look for unexpected IPs or failed auth. |

### Security Model Summary

```
Internet
  → Cloudflare (DDoS protection, TLS termination)
    → Cloudflare Access (email OTP gate — optional but recommended)
      → Cloudflare Tunnel (encrypted, outbound-only, no open ports)
        → Go Router (rate limiting → Slack HMAC verification OR Basic Auth)
          → Input sanitization (length + injection pattern stripping)
            → Claude CLI (runs as your user, inherits your environment)
```

For the full threat model and hardening checklist, see [docs/SECURITY.md](docs/SECURITY.md).

---

## Project Structure

```
.
├── cmd/polymr/main.go            # Entry point — HTTP server
├── internal/
│   ├── agents/manager.go         # Agent registry, sessions, scheduling, Claude CLI exec
│   ├── router/
│   │   ├── router.go             # HTTP routing, host splitting, Next.js proxy
│   │   ├── auth.go               # Basic Auth middleware
│   │   ├── audit.go              # JSON audit log middleware
│   │   └── ratelimit.go          # Token bucket rate limiter
│   ├── sanitize/sanitize.go      # Input validation + prompt injection defense
│   ├── slack/
│   │   ├── client.go             # Slack API wrapper (PostMessage, PostThreadReply)
│   │   ├── handler.go            # Slash commands, events, interactive components
│   │   └── verify.go             # HMAC signature verification middleware
│   ├── scheduler/scheduler.go    # Cron-based scheduled agent runs
│   └── voice/elevenlabs.go       # ElevenLabs TTS/STT integration
├── web/                          # Next.js frontend
│   └── src/app/
│       ├── page.tsx              # Dashboard — agent cards
│       ├── agents/[slug]/page.tsx # Per-agent chat UI
│       ├── voice/page.tsx        # Voice chat
│       └── schedules/page.tsx    # Schedule manager
├── scripts/restart.sh            # Rebuild + restart launchd services
├── docs/
│   ├── DEPLOYMENT.md             # Ops reference
│   └── SECURITY.md               # Threat model
├── .env.example                  # Environment variable template
├── Makefile                      # build, run, dev, frontend, deps, clean
└── README.md                     # You are here
```

---

## Troubleshooting

### "command not found: claude"

The Claude CLI isn't in your PATH. Install it:

```bash
npm install -g @anthropic-ai/claude-code
```

Then verify: `which claude` should print a path. If you installed it but it's not found, your Node global bin directory might not be in PATH. Add it:

```bash
export PATH="$PATH:$(npm config get prefix)/bin"
```

### Slack shows "dispatch_failed" when you type a command

Slack can't reach your machine. Check:

1. Is your tunnel/ngrok running? `curl https://YOUR_DOMAIN/health` should return `{"status":"ok"}`
2. Is the Go router running? `curl http://localhost:8080/health`
3. Does the slash command URL match? Go to Slack App → Slash Commands — the URL should be `https://YOUR_DOMAIN/slack/commands`

### Slack shows the old bot name

Two places to fix:

1. **Slack App → App Home → Bot User Display Name** — change to "Winston"
2. **Slack App → OAuth & Permissions** — make sure `chat:write.customize` scope is added. If you just added it, click **Reinstall to Workspace**.

### Agent returns an empty or error response

```bash
# Test the claude CLI directly:
claude --print --model sonnet "hello"
```

If this fails, your Claude CLI isn't authenticated or your subscription has expired.

### No agents loaded (API returns empty list)

```bash
ls ~/.claude/agents/
```

You should see `.md` files. Each must have valid YAML frontmatter with at least a `name:` field:

```markdown
---
name: winston
---
Your prompt here.
```

### 502 Bad Gateway

The Go router crashed. Check logs:

```bash
# If using launchd:
cat /tmp/winston-router.err.log

# If running in terminal, check the terminal output
```

Rebuild and restart: `make build && make run`

### Voice chat doesn't work

The voice endpoints return HTTP 503 if `ELEVENLABS_API_KEY` isn't set. Add your key to `.env` and restart.

### Rate limited

If you're getting 429 errors, you're hitting the rate limiter (15 req/min on API, 5 req/min on auth). Wait a minute or adjust the limits in `internal/router/ratelimit.go`.

---

## License

MIT
