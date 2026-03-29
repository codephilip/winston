"use client";

import { useState, useEffect, useCallback } from "react";
import Link from "next/link";

interface AgentInfo {
  name: string;
  description: string;
  model?: string;
  workspace?: string;
  short_name?: string;
  tools?: string[];
  system_prompt?: string;
}

interface HealthStatus {
  status: string;
  uptime: string;
  agents: number;
  active_sessions: number;
  active_schedules: number;
}

/* ── tool badge colours ── */
const TOOL_COLORS: Record<string, string> = {
  "Web Search": "border-sky-800 text-sky-400",
  "Web Fetch": "border-sky-800 text-sky-400",
  Git: "border-orange-800 text-orange-400",
  Figma: "border-purple-800 text-purple-400",
  "Google Workspace": "border-blue-800 text-blue-400",
  Slack: "border-green-800 text-green-400",
  "YouTube Data": "border-red-800 text-red-400",
  "Image Gen": "border-pink-800 text-pink-400",
  Playwright: "border-emerald-800 text-emerald-400",
  "Security Tools": "border-red-800 text-red-400",
  Remotion: "border-indigo-800 text-indigo-400",
  Manim: "border-amber-800 text-amber-400",
  "Trend Analysis": "border-cyan-800 text-cyan-400",
  "Sub-Agents": "border-violet-800 text-violet-400",
  Scheduling: "border-yellow-800 text-yellow-400",
};

/* ── hierarchy node ── */
interface TreeNode {
  agent: AgentInfo;
  children: TreeNode[];
  depth: number;
}

function buildTree(agents: AgentInfo[], workspace: string | null): TreeNode[] {
  if (workspace === null) {
    // Personal workspace: winston at root, standalone agents as children
    const orchestrator = agents.find((a) => a.name === "winston");
    const standalone = agents.filter(
      (a) => a.name !== "winston" && !a.workspace
    );
    if (!orchestrator) return standalone.map((a) => ({ agent: a, children: [], depth: 0 }));
    return [
      {
        agent: orchestrator,
        depth: 0,
        children: standalone.map((a) => ({ agent: a, children: [], depth: 1 })),
      },
    ];
  }
  // Workspace: show pipeline chain
  const wsAgents = agents
    .filter((a) => a.workspace === workspace)
    .sort((a, b) => {
      // Try to order by known pipeline stages
      const order = ["research", "director", "assets", "deliver"];
      const ai = order.indexOf(a.short_name || "");
      const bi = order.indexOf(b.short_name || "");
      if (ai >= 0 && bi >= 0) return ai - bi;
      if (ai >= 0) return -1;
      if (bi >= 0) return 1;
      return (a.short_name || a.name).localeCompare(b.short_name || b.name);
    });
  if (wsAgents.length === 0) return [];
  // Chain: first agent is root, each subsequent is child of previous
  let current: TreeNode = { agent: wsAgents[0], children: [], depth: 0 };
  const root = current;
  for (let i = 1; i < wsAgents.length; i++) {
    const child: TreeNode = { agent: wsAgents[i], children: [], depth: i };
    current.children = [child];
    current = child;
  }
  return [root];
}

/* ── components ── */

function ToolBadge({ tool }: { tool: string }) {
  const color = TOOL_COLORS[tool] || "border-zinc-700 text-zinc-500";
  return (
    <span
      className={`inline-block rounded border px-1.5 py-px text-[10px] leading-tight ${color}`}
    >
      {tool}
    </span>
  );
}

function TreeNodeRow({
  node,
  expandedAgent,
  onToggle,
  isLast,
}: {
  node: TreeNode;
  expandedAgent: string | null;
  onToggle: (name: string) => void;
  isLast: boolean;
}) {
  const { agent, children, depth } = node;
  const isExpanded = expandedAgent === agent.name;
  const hasChildren = children.length > 0;
  const displayName = agent.short_name || agent.name;

  return (
    <div>
      {/* node row */}
      <div className="flex items-stretch">
        {/* tree lines */}
        {depth > 0 && (
          <div className="flex" style={{ width: depth * 32 }}>
            {Array.from({ length: depth }).map((_, i) => (
              <div key={i} className="flex w-8 justify-center">
                {i === depth - 1 ? (
                  <div className="relative w-8">
                    <div
                      className={`absolute left-1/2 top-0 w-px bg-zinc-800 ${isLast ? "h-1/2" : "h-full"}`}
                    />
                    <div className="absolute left-1/2 top-1/2 h-px w-4 bg-zinc-800" />
                  </div>
                ) : (
                  <div className="w-px bg-zinc-800" />
                )}
              </div>
            ))}
          </div>
        )}

        {/* agent card */}
        <button
          onClick={() => onToggle(agent.name)}
          className={`group flex flex-1 items-center gap-4 rounded-lg border px-4 py-3 text-left transition-all ${
            isExpanded
              ? "border-zinc-600 bg-zinc-800/80"
              : "border-zinc-800/50 bg-zinc-900/50 hover:border-zinc-700 hover:bg-zinc-800/40"
          }`}
        >
          {/* model dot */}
          <div className="flex flex-col items-center gap-0.5">
            <span
              className={`text-[10px] font-medium ${
                agent.model === "opus"
                  ? "text-amber-400"
                  : agent.model === "haiku"
                    ? "text-green-400"
                    : "text-blue-400"
              }`}
            >
              {agent.model}
            </span>
          </div>

          {/* name + desc */}
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <span className="font-semibold capitalize text-zinc-100">
                {displayName}
              </span>
              {hasChildren && (
                <span className="text-[10px] text-zinc-600">
                  → {children.length} agent{children.length > 1 ? "s" : ""}
                </span>
              )}
            </div>
            <p className="truncate text-xs text-zinc-500">
              {agent.description}
            </p>
          </div>

          {/* tools */}
          <div className="hidden flex-wrap justify-end gap-1 sm:flex">
            {(agent.tools || []).slice(0, 4).map((t) => (
              <ToolBadge key={t} tool={t} />
            ))}
            {(agent.tools || []).length > 4 && (
              <span className="text-[10px] text-zinc-600">
                +{(agent.tools || []).length - 4}
              </span>
            )}
          </div>

          {/* expand indicator */}
          <svg
            className={`h-4 w-4 shrink-0 text-zinc-600 transition-transform ${isExpanded ? "rotate-180" : ""}`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
          </svg>
        </button>

        {/* chat link */}
        <Link
          href={`/agents/${agent.name}`}
          className="ml-2 flex items-center rounded-lg border border-zinc-800/50 px-3 text-xs text-zinc-500 transition-all hover:border-zinc-600 hover:text-zinc-300"
        >
          Chat
        </Link>
      </div>

      {/* expanded detail */}
      {isExpanded && (
        <div className="mt-1 mb-1" style={{ marginLeft: depth * 32 }}>
          <AgentDetail agent={agent} />
        </div>
      )}

      {/* children */}
      {children.map((child, i) => (
        <div key={child.agent.name} className="mt-1">
          <TreeNodeRow
            node={child}
            expandedAgent={expandedAgent}
            onToggle={onToggle}
            isLast={i === children.length - 1}
          />
        </div>
      ))}
    </div>
  );
}

function AgentDetail({ agent }: { agent: AgentInfo }) {
  const [prompt, setPrompt] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setLoading(true);
    fetch(`/api/agents/${agent.name}`)
      .then((r) => r.json())
      .then((data) => setPrompt(data.system_prompt || "(no system prompt)"))
      .catch(() => setPrompt("(failed to load)"))
      .finally(() => setLoading(false));
  }, [agent.name]);

  return (
    <div className="rounded-lg border border-zinc-800 bg-zinc-900/80 p-4">
      {/* tools - full list on mobile */}
      {agent.tools && agent.tools.length > 0 && (
        <div className="mb-3 flex flex-wrap gap-1">
          {agent.tools.map((t) => (
            <ToolBadge key={t} tool={t} />
          ))}
        </div>
      )}

      {/* meta */}
      <div className="mb-3 flex flex-wrap gap-x-6 gap-y-1 text-xs text-zinc-500">
        <span>
          Model: <span className="text-zinc-300">{agent.model}</span>
        </span>
        {agent.workspace && (
          <span>
            Workspace:{" "}
            <span className="text-zinc-300">{agent.workspace}</span>
          </span>
        )}
      </div>

      {/* system prompt */}
      <div className="rounded-md bg-zinc-950 p-3">
        <p className="mb-2 text-[10px] font-medium uppercase tracking-wider text-zinc-600">
          System Prompt
        </p>
        {loading ? (
          <p className="text-xs text-zinc-600">Loading…</p>
        ) : (
          <pre className="max-h-80 overflow-auto whitespace-pre-wrap font-mono text-xs leading-relaxed text-zinc-400">
            {prompt}
          </pre>
        )}
      </div>
    </div>
  );
}

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
        if (agentsRes.ok) {
          setAgents(await agentsRes.json());
        }
      } catch {
        setServiceUp(false);
      }
    }
    fetchData();
  }, []);

  const toggleAgent = useCallback(
    (name: string) => setExpandedAgent((prev) => (prev === name ? null : name)),
    []
  );

  // Derive workspaces
  const workspaceNames = [
    ...new Set(agents.filter((a) => a.workspace).map((a) => a.workspace!)),
  ].sort();

  const tree = buildTree(agents, activeWorkspace);

  return (
    <div className="min-h-screen bg-zinc-950 text-white">
      <header className="sticky top-0 z-10 border-b border-zinc-800 bg-zinc-950/90 px-6 py-3 backdrop-blur-sm">
        <div className="mx-auto flex max-w-5xl items-center justify-between">
          <div className="flex items-center gap-5">
            <h1 className="text-lg font-bold tracking-tight">Winston</h1>

            {/* workspace tabs */}
            <nav className="flex items-center gap-1 rounded-lg border border-zinc-800 bg-zinc-900 p-0.5">
              <button
                onClick={() => { setActiveWorkspace(null); setExpandedAgent(null); }}
                className={`rounded-md px-3 py-1.5 text-xs font-medium transition-all ${
                  activeWorkspace === null
                    ? "bg-zinc-700 text-white"
                    : "text-zinc-500 hover:text-zinc-300"
                }`}
              >
                Personal
              </button>
              {workspaceNames.map((ws) => (
                <button
                  key={ws}
                  onClick={() => { setActiveWorkspace(ws); setExpandedAgent(null); }}
                  className={`rounded-md px-3 py-1.5 text-xs font-medium capitalize transition-all ${
                    activeWorkspace === ws
                      ? "bg-zinc-700 text-white"
                      : "text-zinc-500 hover:text-zinc-300"
                  }`}
                >
                  {ws}
                </button>
              ))}
            </nav>

            {/* health dot */}
            {serviceUp !== null && (
              <div className="flex items-center gap-2">
                <span
                  className={`h-1.5 w-1.5 rounded-full ${serviceUp ? "bg-green-500" : "bg-red-500"}`}
                />
                {health ? (
                  <span className="text-[11px] text-zinc-600">
                    {health.uptime}
                  </span>
                ) : serviceUp === false ? (
                  <span className="text-[11px] text-red-500">offline</span>
                ) : null}
              </div>
            )}
          </div>

          <nav className="flex gap-2">
            <Link
              href="/voice"
              className="rounded-lg bg-zinc-800 px-3 py-1.5 text-xs hover:bg-zinc-700"
            >
              Voice
            </Link>
            <Link
              href="/schedules"
              className="rounded-lg bg-zinc-800 px-3 py-1.5 text-xs hover:bg-zinc-700"
            >
              Schedules
            </Link>
          </nav>
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-6 py-8">
        {agents.length === 0 && serviceUp !== false && (
          <p className="py-20 text-center text-sm text-zinc-600">
            Loading agents…
          </p>
        )}
        {serviceUp === false && (
          <p className="py-20 text-center text-sm text-red-500/80">
            Service unreachable — is the router running?
          </p>
        )}

        <div className="space-y-1">
          {tree.map((node, i) => (
            <TreeNodeRow
              key={node.agent.name}
              node={node}
              expandedAgent={expandedAgent}
              onToggle={toggleAgent}
              isLast={i === tree.length - 1}
            />
          ))}
        </div>
      </main>
    </div>
  );
}
