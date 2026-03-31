# Agent Guide

How to add new agents, update existing ones, and make them available in Slack.

## How Agents Work

An agent is a Markdown file in `~/.claude/agents/` with YAML frontmatter and a system prompt. On startup, the Go router reads every `.md` file in that directory and registers it. That single file becomes:

- A **Slack slash command** (`/myagent do something`)
- An **@mention handler** (`@Winston myagent do something`)
- **HTTP API endpoints** (`POST /api/agents/myagent/run`, `GET /api/agents/myagent`)
- A **web dashboard card** with tool icons, model badge, and system prompt viewer
- An **individual chat page** (`/agents/myagent`)

The router does not care which channel you're in — agents work the same in every channel. More on channel behavior [below](#how-agents-work-in-slack-channels).

---

## Adding a New Agent

### Step 1: Create the agent file

Create `~/.claude/agents/<name>.md`:

```markdown
---
name: researcher
description: Deep research agent for any topic
model: sonnet
timeout: 600
max_turns: 50
---

You are a research agent. When given a topic, use web search to find
the most recent and authoritative sources. Synthesize your findings
into a clear, well-structured report with citations.

Always verify claims across multiple sources before including them.
```

### Frontmatter fields

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `name` | Yes | — | Agent ID. This becomes the slash command name (`/researcher`) and the API route. Use lowercase, no spaces. Prefix with `workspace-` for grouping (see [Workspaces](#workspaces)). |
| `description` | No | — | One-line description. Shown on the dashboard and when a user @mentions Winston without specifying an agent. |
| `model` | No | `sonnet` | Claude model: `opus` (Opus 4.6), `sonnet` (Sonnet 4.6), or `haiku` (Haiku 4.5). Can be changed at runtime via the web UI or API. |
| `timeout` | No | `600` | Max execution time in seconds before the agent is killed. |
| `max_turns` | No | `25` | Max conversation turns per agent run. |

### Step 2: Restart the router

Agents are loaded once at startup. After creating or editing an agent file, restart:

```bash
make build && make run
```

You'll see this in the logs if it loaded correctly:

```
[agents] loaded researcher (model=sonnet)
[agents] 6 agent(s) ready
```

If there's a problem with your file (missing frontmatter, no `name` field), you'll see:

```
[agents] skipping researcher.md: missing 'name' in frontmatter
```

### Step 3: Create the Slack slash command

Go to **[api.slack.com/apps](https://api.slack.com/apps)** -> your Winston app -> **Features -> Slash Commands** -> **Create New Command**:

| Field | Value |
|-------|-------|
| **Command** | `/researcher` |
| **Request URL** | `https://YOUR_DOMAIN/slack/commands` |
| **Short Description** | Deep research agent |

The request URL is the same for every agent — the router reads the command name from the payload and routes to the right agent.

After saving, Slack may ask you to **reinstall the app** to your workspace. Do that.

### Step 4: Test it

In any Slack channel:

```
/researcher what are the latest developments in quantum computing?
```

The agent will post a "thinking..." message in a thread and stream its response as it works.

---

## Workspaces

Agent names with a hyphen are auto-grouped by workspace prefix on the dashboard:

| Agent file name | Workspace | Short name |
|----------------|-----------|------------|
| `codephil-research.md` | codephil | research |
| `codephil-youtube.md` | codephil | youtube |
| `rivalytics-social.md` | rivalytics | social |
| `winston.md` | *(top-level)* | winston |

The dashboard shows:
- A **workspace dropdown** to filter by workspace (colored initials, checkmarks)
- Agents grouped under their workspace heading
- Pipeline ordering within workspaces (research -> director -> assets -> deliver)

This is purely a UI grouping — in Slack and the API, you always use the full agent name (`/codephil-research`).

---

## Tool Auto-Detection

The router scans each agent's system prompt for keywords and tags detected tools automatically. These appear as icon badges on the dashboard agent cards.

Supported tool detections:

| Keywords in system prompt | Tool icon shown |
|--------------------------|----------------|
| `web search`, `web_search` | Web Search |
| `git`, `github`, `repository` | Git |
| `figma` | Figma |
| `google workspace`, `google calendar`, `gmail` | Google Workspace |
| `slack` | Slack |
| `youtube` | YouTube |
| `image gen`, `nano banana`, `thumbnail` | Image Gen |
| `playwright`, `browser` | Playwright |
| `security`, `pentest`, `nmap`, `exploit` | Security Tools |
| `remotion`, `video` | Remotion |
| `schedule`, `cron` | Scheduling |

Agents with many tools show the first 3 as icon+label badges, with a clickable **+N** overflow that expands to show the rest.

---

## Updating an Existing Agent

### Edit the file directly

```bash
vim ~/.claude/agents/marketing.md
```

After saving, restart the router: `make build && make run`

### Change the model at runtime (no restart needed)

**Web UI:** Open the agent chat page (`/agents/marketing`) and click the model switcher in the header (Haiku 4.5 / Sonnet 4.6 / Opus 4.6). This updates the `.md` file on disk and triggers an automatic service restart.

**API:**
```bash
curl -X PUT -u admin:yourpass http://localhost:8080/api/agents/marketing/model \
  -H 'Content-Type: application/json' \
  -d '{"model": "opus"}'
```

A notification is posted to Slack when the model changes.

### Edit the system prompt at runtime

**Web UI:** On the dashboard, click any agent card, then click the edit toggle in the system prompt viewer. Edit the markdown and click Save.

**API:**
```bash
curl -X PUT -u admin:yourpass http://localhost:8080/api/agents/marketing/prompt \
  -H 'Content-Type: application/json' \
  -d '{"system_prompt": "You are a revised marketing agent..."}'
```

Both update the `.md` file on disk, notify Slack, and trigger a restart.

---

## How Agents Work in Slack Channels

### Slash commands (`/agent prompt`)

Slash commands work in **any channel** — public, private, or DM. When you type `/marketing analyze our competitors`, Slack sends it to the router regardless of which channel you're in.

The agent's response appears as a **thread reply** to the command message. This keeps the channel clean — the main channel just shows the command, and the full response is in the thread.

### @mentions (`@Winston agent prompt`)

Mentioning `@Winston` works in any channel the bot has been invited to. The format is:

```
@Winston marketing analyze our homepage SEO
```

The router parses the first word after the mention as the agent name. Supported formats:

- `@Winston marketing do X`
- `@Winston /marketing do X`
- `@Winston marketing: do X`

If you @mention Winston without specifying an agent, it replies with a list of available agents.

### Thread replies (continuing a conversation)

Once an agent responds in a thread, you can **reply in that thread** to continue the conversation — no need to @mention or use a slash command. Just type your follow-up message in the thread.

The router keeps a session for each thread (keyed by the thread timestamp). When you reply, it resumes the same Claude session with full conversation history.

**Sessions persist across restarts** — they're saved to `~/.config/winston/sessions.json`. A session stub is stored before the agent run starts, so follow-ups work even if the initial run fails or is interrupted.

### Channel-specific tips

| Channel type | Slash commands | @mentions | Thread replies |
|-------------|---------------|-----------|---------------|
| **Public channel** | Always work | Work if bot is in the channel | Work while session exists |
| **Private channel** | Always work | Must invite the bot first (`/invite @Winston`) | Work while session exists |
| **DM with the bot** | Always work | Always work | Work while session exists |
| **Group DM** | Always work | Must include the bot in the group | Work while session exists |

Slash commands are the most reliable — they work everywhere without needing to invite the bot.

---

## Scheduled Agent Runs

Agents can be scheduled to run automatically on a cron pattern:

```bash
# Via the /schedules web page (cron builder UI)
# Or via the API:
curl -X POST -u admin:yourpass http://localhost:8080/api/schedules \
  -H 'Content-Type: application/json' \
  -d '{
    "agent_id": "marketing",
    "prompt": "Generate weekly competitor report",
    "cron": "0 9 * * 1",
    "slack_channel": "marketing-reports",
    "timezone": "America/Denver"
  }'
```

When a schedule fires:
1. Posts a trigger message to the Slack channel
2. Runs the agent and posts results as a **thread reply**
3. Tags the owner (`@PG`) so they get a notification
4. Stores a session so the owner can reply in the thread to continue

Schedules persist across restarts (`~/.config/winston/schedules.json`).

---

## Writing Good System Prompts

The system prompt is what makes your agent useful. Some guidelines:

**Be specific about the agent's role.** "You are a marketing specialist" is better than "You are a helpful assistant."

**Tell it what tools to use.** Claude Code agents have access to web search, file reading, bash commands, and more. If your agent should use specific tools, say so — and the dashboard will auto-detect and show the tool icons:

```markdown
When asked to analyze a website, use web search and browsing tools
to gather real data. Never make up statistics.
```

**Set boundaries.** If the agent should stay in its lane:

```markdown
You only handle marketing tasks. If asked about something unrelated,
politely redirect the user to use a different agent.
```

**Include output format preferences:**

```markdown
Always structure reports with:
- Executive summary (2-3 sentences)
- Key findings (bulleted list)
- Recommendations (numbered, actionable)
```

### Example: Full agent file

```markdown
---
name: reviewer
description: Code review agent that checks PRs for bugs and style
model: opus
timeout: 900
max_turns: 50
---

You are a senior code reviewer. When given a PR number or diff, analyze it for:

1. **Bugs** — logic errors, edge cases, null safety, race conditions
2. **Security** — injection, auth issues, secrets in code
3. **Style** — naming, structure, consistency with the codebase
4. **Performance** — unnecessary allocations, N+1 queries, missing indexes

Use `gh pr diff` to read the actual diff. Use `gh pr view` for context.

Be direct. If it looks good, say so in one line. If there are issues,
list them with file:line references.

Do NOT nitpick formatting or suggest trivial renaming unless it
meaningfully improves readability.
```

---

## Removing an Agent

1. Delete the file: `rm ~/.claude/agents/researcher.md`
2. Restart the router: `make build && make run`
3. Optionally remove the slash command from the Slack app settings (it will return "Unknown agent: /researcher" if someone uses it, but it won't break anything)

---

## Quick Reference

```bash
# List current agent files
ls ~/.claude/agents/*.md

# Check what the router loaded (look at startup logs)
make run 2>&1 | grep '\[agents\]'

# Test an agent via the HTTP API (no Slack needed)
curl -X POST -u admin:yourpass http://localhost:8080/api/agents/researcher/run \
  -H 'Content-Type: application/json' \
  -d '{"prompt": "summarize the latest AI news"}'

# List all registered agents via API
curl -u admin:yourpass http://localhost:8080/api/agents

# Get agent detail (including system prompt)
curl -u admin:yourpass http://localhost:8080/api/agents/researcher

# Change an agent's model
curl -X PUT -u admin:yourpass http://localhost:8080/api/agents/researcher/model \
  -H 'Content-Type: application/json' \
  -d '{"model": "opus"}'
```

---

## Troubleshooting

| Problem | Cause | Fix |
|---------|-------|-----|
| Agent not showing up after restart | Frontmatter parse error | Check logs for `[agents] skipping`. Ensure you have `---` on first line, `name:` field, and closing `---`. |
| Slash command returns "Unknown agent" | Agent name doesn't match command | The `name:` in frontmatter must exactly match the slash command (e.g., `name: researcher` for `/researcher`). |
| "thinking..." message but no response | Agent timed out | Increase `timeout:` in frontmatter, or simplify the prompt. Check router logs for timeout errors. |
| @mention doesn't respond | Bot not in channel | Invite the bot: `/invite @Winston`. Or use slash commands instead (work everywhere). |
| Thread replies don't work | Session not found | Sessions persist across restarts now. If still failing, check that the original slash command completed successfully. |
| Slash command not available in Slack | Not created in Slack app settings | Go to api.slack.com/apps -> Slash Commands -> Create New Command. |
| Model change doesn't take effect | Restart failed | Check logs after model change. The service should auto-restart. Try `./scripts/restart.sh` manually. |
| Agent not in workspace group | Name missing hyphen | Prefix with `workspace-` (e.g., `codephil-research`) for grouping. |
