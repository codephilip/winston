"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import Link from "next/link";

/* ── types ── */

interface AgentInfo {
  name: string;
  description: string;
  model?: string;
  workspace?: string;
  short_name?: string;
  tools?: string[];
}

interface HealthStatus {
  status: string;
  uptime: string;
  agents: number;
  active_sessions: number;
  active_schedules: number;
}

/* ── constants ── */

const TOOL_COLORS: Record<string, string> = {
  "Web Search": "text-sky-400 bg-sky-950",
  "Web Fetch": "text-sky-400 bg-sky-950",
  Git: "text-orange-400 bg-orange-950",
  Figma: "text-purple-400 bg-purple-950",
  "Google Workspace": "text-blue-400 bg-blue-950",
  Slack: "text-green-400 bg-green-950",
  "YouTube Data": "text-red-400 bg-red-950",
  "Image Gen": "text-pink-400 bg-pink-950",
  Playwright: "text-emerald-400 bg-emerald-950",
  "Security Tools": "text-red-400 bg-red-950",
  Remotion: "text-indigo-400 bg-indigo-950",
  Manim: "text-amber-400 bg-amber-950",
  "Trend Analysis": "text-cyan-400 bg-cyan-950",
  "Sub-Agents": "text-violet-400 bg-violet-950",
  Scheduling: "text-yellow-400 bg-yellow-950",
};

const MODEL_COLORS: Record<string, string> = {
  opus: "text-amber-400",
  sonnet: "text-blue-400",
  haiku: "text-green-400",
};

/* ── helpers ── */

function getAgentDisplayName(a: AgentInfo) {
  return a.short_name || a.name;
}

function buildHierarchy(
  agents: AgentInfo[],
  workspace: string | null
): AgentInfo[][] {
  if (workspace === null) {
    // Personal: orchestrator first, then standalone agents as flat list
    const orch = agents.find((a) => a.name === "winston");
    const standalone = agents.filter(
      (a) => a.name !== "winston" && !a.workspace
    );
    const rows: AgentInfo[][] = [];
    if (orch) rows.push([orch]);
    if (standalone.length) rows.push(standalone);
    return rows;
  }
  // Workspace: order pipeline, return as single row per stage
  const wsAgents = agents.filter((a) => a.workspace === workspace);
  const order = ["research", "director", "assets", "deliver"];
  wsAgents.sort((a, b) => {
    const ai = order.indexOf(a.short_name || "");
    const bi = order.indexOf(b.short_name || "");
    if (ai >= 0 && bi >= 0) return ai - bi;
    if (ai >= 0) return -1;
    if (bi >= 0) return 1;
    return (a.short_name || a.name).localeCompare(b.short_name || b.name);
  });
  return wsAgents.map((a) => [a]);
}

/* ── components ── */

function ToolBadge({ tool }: { tool: string }) {
  const color = TOOL_COLORS[tool] || "text-zinc-400 bg-zinc-900";
  return (
    <span className={`rounded px-1.5 py-0.5 text-[10px] font-medium leading-none ${color}`}>
      {tool}
    </span>
  );
}

function WorkspaceDropdown({
  workspaces,
  active,
  onChange,
}: {
  workspaces: string[];
  active: string | null;
  onChange: (ws: string | null) => void;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function close(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, []);

  const label = active === null ? "Personal" : active;

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-2 rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-1.5 text-sm font-medium capitalize transition-colors hover:border-zinc-700"
      >
        <span className="h-2 w-2 rounded-full bg-gradient-to-br from-blue-400 to-violet-500" />
        {label}
        <svg
          className={`h-3.5 w-3.5 text-zinc-500 transition-transform ${open ? "rotate-180" : ""}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
        </svg>
      </button>
      {open && (
        <div className="absolute left-0 top-full z-20 mt-1 min-w-[180px] overflow-hidden rounded-xl border border-zinc-800 bg-zinc-900 shadow-2xl shadow-black/40">
          <button
            onClick={() => { onChange(null); setOpen(false); }}
            className={`flex w-full items-center gap-2.5 px-3.5 py-2.5 text-left text-sm transition-colors hover:bg-zinc-800 ${active === null ? "bg-zinc-800/60 text-white" : "text-zinc-400"}`}
          >
            <span className="h-2 w-2 rounded-full bg-amber-500" />
            Personal
          </button>
          {workspaces.map((ws) => (
            <button
              key={ws}
              onClick={() => { onChange(ws); setOpen(false); }}
              className={`flex w-full items-center gap-2.5 px-3.5 py-2.5 text-left text-sm capitalize transition-colors hover:bg-zinc-800 ${active === ws ? "bg-zinc-800/60 text-white" : "text-zinc-400"}`}
            >
              <span className="h-2 w-2 rounded-full bg-violet-500" />
              {ws}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function AgentRow({
  agent,
  isExpanded,
  onToggle,
  depth,
}: {
  agent: AgentInfo;
  isExpanded: boolean;
  onToggle: () => void;
  depth: number;
}) {
  const name = getAgentDisplayName(agent);

  return (
    <div>
      <div
        className="flex items-center gap-3"
        style={{ paddingLeft: depth * 24 }}
      >
        {/* depth indicator */}
        {depth > 0 && (
          <div className="flex h-5 w-5 items-center justify-center">
            <svg className="h-3 w-3 text-zinc-700" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5">
              <path d="M2 0v6h10" />
            </svg>
          </div>
        )}

        {/* main card */}
        <button
          onClick={onToggle}
          className={`flex min-w-0 flex-1 items-center gap-3 rounded-xl border px-4 py-3 text-left transition-all ${
            isExpanded
              ? "border-zinc-700 bg-zinc-800/70"
              : "border-transparent bg-zinc-900/60 hover:bg-zinc-900"
          }`}
        >
          {/* model badge */}
          <span className={`shrink-0 text-[11px] font-semibold ${MODEL_COLORS[agent.model || "sonnet"] || "text-zinc-400"}`}>
            {agent.model}
          </span>

          {/* name + desc */}
          <div className="min-w-0 flex-1">
            <span className="font-semibold capitalize">{name}</span>
            <p className="truncate text-xs text-zinc-500">{agent.description}</p>
          </div>

          {/* tools (desktop) */}
          <div className="hidden gap-1 lg:flex">
            {(agent.tools || []).slice(0, 3).map((t) => (
              <ToolBadge key={t} tool={t} />
            ))}
            {(agent.tools || []).length > 3 && (
              <span className="rounded bg-zinc-800 px-1.5 py-0.5 text-[10px] text-zinc-600">
                +{(agent.tools || []).length - 3}
              </span>
            )}
          </div>
        </button>

        {/* chat link */}
        <Link
          href={`/agents/${agent.name}`}
          className="shrink-0 rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-xs text-zinc-500 transition-all hover:border-zinc-600 hover:text-white"
        >
          Chat
        </Link>
      </div>

      {/* expanded detail */}
      {isExpanded && (
        <div style={{ paddingLeft: depth * 24 + (depth > 0 ? 32 : 0) }}>
          <AgentDetail agent={agent} />
        </div>
      )}
    </div>
  );
}

function AgentDetail({ agent }: { agent: AgentInfo }) {
  const [prompt, setPrompt] = useState<string>("");
  const [original, setOriginal] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    fetch(`/api/agents/${agent.name}`)
      .then((r) => r.json())
      .then((data) => {
        const p = data.system_prompt || "";
        setPrompt(p);
        setOriginal(p);
      })
      .catch(() => setPrompt("(failed to load)"))
      .finally(() => setLoading(false));
  }, [agent.name]);

  const dirty = prompt !== original;

  async function save() {
    setSaving(true);
    setSaved(false);
    try {
      const res = await fetch(`/api/agents/${agent.name}/prompt`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ system_prompt: prompt }),
      });
      if (res.ok) {
        setOriginal(prompt);
        setSaved(true);
        setTimeout(() => setSaved(false), 4000);
      }
    } catch {
      // connection error
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="mt-2 mb-2 rounded-xl border border-zinc-800 bg-zinc-900/80 p-4">
      {/* tools */}
      {agent.tools && agent.tools.length > 0 && (
        <div className="mb-3 flex flex-wrap gap-1.5">
          {agent.tools.map((t) => (
            <ToolBadge key={t} tool={t} />
          ))}
        </div>
      )}

      {/* editor */}
      <div className="rounded-lg border border-zinc-800 bg-zinc-950">
        <div className="flex items-center justify-between border-b border-zinc-800 px-3 py-2">
          <span className="text-[10px] font-medium uppercase tracking-wider text-zinc-600">
            {agent.name}.md
          </span>
          <div className="flex items-center gap-2">
            {saved && (
              <span className="text-[11px] text-green-400">
                Saved — restarting…
              </span>
            )}
            {dirty && !saved && (
              <button
                onClick={save}
                disabled={saving}
                className="rounded-md bg-blue-600 px-3 py-1 text-[11px] font-medium text-white transition-colors hover:bg-blue-500 disabled:opacity-50"
              >
                {saving ? "Saving…" : "Save & Restart"}
              </button>
            )}
          </div>
        </div>
        {loading ? (
          <p className="px-3 py-4 text-xs text-zinc-600">Loading…</p>
        ) : (
          <textarea
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            spellCheck={false}
            className="block w-full resize-y bg-transparent px-3 py-3 font-mono text-xs leading-relaxed text-zinc-300 placeholder-zinc-700 focus:outline-none"
            rows={Math.min(Math.max(prompt.split("\n").length + 2, 8), 24)}
          />
        )}
      </div>
    </div>
  );
}

/* ── main page ── */

export default function Home() {
  const [agents, setAgents] = useState<AgentInfo[]>([]);
  const [health, setHealth] = useState<HealthStatus | null>(null);
  const [serviceUp, setServiceUp] = useState<boolean | null>(null);
  const [activeWorkspace, setActiveWorkspace] = useState<string | null>(null);
  const [expandedAgent, setExpandedAgent] = useState<string | null>(null);

  useEffect(() => {
    async function fetchData() {
      try {
        const [healthRes, agentsRes] = await Promise.all([
          fetch("/api/health"),
          fetch("/api/agents"),
        ]);
        if (healthRes.ok) {
          setHealth(await healthRes.json());
          setServiceUp(true);
        } else {
          setServiceUp(false);
        }
        if (agentsRes.ok) setAgents(await agentsRes.json());
      } catch {
        setServiceUp(false);
      }
    }
    fetchData();
  }, []);

  const toggleAgent = useCallback(
    (name: string) => setExpandedAgent((p) => (p === name ? null : name)),
    []
  );

  const workspaceNames = [
    ...new Set(agents.filter((a) => a.workspace).map((a) => a.workspace!)),
  ].sort();

  const tiers = buildHierarchy(agents, activeWorkspace);

  return (
    <div className="min-h-screen bg-zinc-950 text-white">
      {/* header */}
      <header className="sticky top-0 z-10 border-b border-zinc-800 bg-zinc-950/95 px-6 py-3 backdrop-blur-sm">
        <div className="mx-auto flex max-w-4xl items-center justify-between">
          <div className="flex items-center gap-4">
            <h1 className="text-lg font-bold tracking-tight">Winston</h1>

            <WorkspaceDropdown
              workspaces={workspaceNames}
              active={activeWorkspace}
              onChange={(ws) => { setActiveWorkspace(ws); setExpandedAgent(null); }}
            />

            {serviceUp !== null && (
              <div className="flex items-center gap-1.5">
                <span className={`h-1.5 w-1.5 rounded-full ${serviceUp ? "bg-green-500" : "bg-red-500"}`} />
                {health ? (
                  <span className="text-[11px] text-zinc-600">{health.uptime}</span>
                ) : serviceUp === false ? (
                  <span className="text-[11px] text-red-500">offline</span>
                ) : null}
              </div>
            )}
          </div>
          <nav className="flex gap-2">
            <Link href="/voice" className="rounded-lg bg-zinc-900 px-3 py-1.5 text-xs text-zinc-400 transition-colors hover:bg-zinc-800 hover:text-white">
              Voice
            </Link>
            <Link href="/schedules" className="rounded-lg bg-zinc-900 px-3 py-1.5 text-xs text-zinc-400 transition-colors hover:bg-zinc-800 hover:text-white">
              Schedules
            </Link>
          </nav>
        </div>
      </header>

      {/* body */}
      <main className="mx-auto max-w-4xl px-6 py-6">
        {agents.length === 0 && serviceUp !== false && (
          <p className="py-20 text-center text-sm text-zinc-600">Loading agents…</p>
        )}
        {serviceUp === false && (
          <p className="py-20 text-center text-sm text-red-500/80">
            Service unreachable
          </p>
        )}

        <div className="space-y-1.5">
          {tiers.map((tier, tierIdx) => (
            <div key={tierIdx}>
              {/* tier label for personal workspace */}
              {activeWorkspace === null && tierIdx === 0 && tier.length === 1 && (
                <p className="mb-2 text-[10px] font-medium uppercase tracking-widest text-zinc-600">
                  Orchestrator
                </p>
              )}
              {activeWorkspace === null && tierIdx === 1 && (
                <p className="mb-2 mt-4 text-[10px] font-medium uppercase tracking-widest text-zinc-600">
                  Agents
                </p>
              )}

              {/* pipeline arrow between workspace stages */}
              {activeWorkspace !== null && tierIdx > 0 && (
                <div className="flex justify-center py-0.5">
                  <svg className="h-4 w-4 text-zinc-700" fill="currentColor" viewBox="0 0 16 16">
                    <path d="M8 12l-4-4h8l-4 4z" />
                  </svg>
                </div>
              )}

              <div className="space-y-1.5">
                {tier.map((agent) => (
                  <AgentRow
                    key={agent.name}
                    agent={agent}
                    isExpanded={expandedAgent === agent.name}
                    onToggle={() => toggleAgent(agent.name)}
                    depth={activeWorkspace === null && tierIdx === 1 ? 1 : 0}
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      </main>
    </div>
  );
}
