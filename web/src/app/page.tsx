"use client";

import { useState, useEffect } from "react";
import Link from "next/link";

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
  frontend: string;
  agents: number;
  active_sessions: number;
  active_schedules: number;
}

interface Workspace {
  name: string;
  agents: AgentInfo[];
}

const AGENT_STYLES: Record<string, { icon: string; color: string }> = {
  winston: { icon: "W", color: "bg-amber-500" },
  marketing: { icon: "M", color: "bg-blue-500" },
  pentester: { icon: "P", color: "bg-red-500" },
  youtube: { icon: "Y", color: "bg-purple-500" },
  designer: { icon: "D", color: "bg-emerald-500" },
  research: { icon: "R", color: "bg-cyan-500" },
  director: { icon: "Dr", color: "bg-orange-500" },
  assets: { icon: "A", color: "bg-pink-500" },
  deliver: { icon: "De", color: "bg-teal-500" },
  social: { icon: "S", color: "bg-violet-500" },
};

function getStyle(agent: AgentInfo) {
  return (
    AGENT_STYLES[agent.short_name || agent.name] ||
    AGENT_STYLES[agent.name] || {
      icon: (agent.short_name || agent.name).charAt(0).toUpperCase(),
      color: "bg-zinc-600",
    }
  );
}

const TOOL_COLORS: Record<string, string> = {
  "Web Search": "bg-sky-900/50 text-sky-300",
  "Web Fetch": "bg-sky-900/50 text-sky-300",
  Git: "bg-orange-900/50 text-orange-300",
  Figma: "bg-purple-900/50 text-purple-300",
  "Google Workspace": "bg-blue-900/50 text-blue-300",
  Slack: "bg-green-900/50 text-green-300",
  "YouTube Data": "bg-red-900/50 text-red-300",
  "Image Gen": "bg-pink-900/50 text-pink-300",
  Playwright: "bg-emerald-900/50 text-emerald-300",
  "Security Tools": "bg-red-900/50 text-red-300",
  Remotion: "bg-indigo-900/50 text-indigo-300",
  Manim: "bg-amber-900/50 text-amber-300",
  "Trend Analysis": "bg-cyan-900/50 text-cyan-300",
  "Sub-Agents": "bg-violet-900/50 text-violet-300",
  Scheduling: "bg-yellow-900/50 text-yellow-300",
};

function ToolBadge({ tool }: { tool: string }) {
  const color = TOOL_COLORS[tool] || "bg-zinc-800 text-zinc-400";
  return (
    <span className={`inline-block rounded-full px-2 py-0.5 text-[10px] font-medium ${color}`}>
      {tool}
    </span>
  );
}

function AgentCard({ agent, compact }: { agent: AgentInfo; compact?: boolean }) {
  const style = getStyle(agent);
  const displayName = agent.short_name || agent.name;

  return (
    <Link
      href={`/agents/${agent.name}`}
      className={`group relative rounded-xl border border-zinc-800 bg-zinc-900 transition-all hover:border-zinc-600 hover:bg-zinc-800/80 ${compact ? "p-4" : "p-5"}`}
    >
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div
            className={`flex ${compact ? "h-9 w-9 text-sm" : "h-10 w-10 text-base"} items-center justify-center rounded-lg ${style.color} font-bold`}
          >
            {style.icon}
          </div>
          <div>
            <h3 className={`font-semibold capitalize group-hover:text-white ${compact ? "text-sm" : "text-base"}`}>
              {displayName}
            </h3>
          </div>
        </div>
        {agent.model && (
          <span className="rounded bg-zinc-800 px-2 py-0.5 text-[10px] text-zinc-500 group-hover:text-zinc-400">
            {agent.model}
          </span>
        )}
      </div>
      {!compact && agent.description && (
        <p className="mb-3 text-xs leading-relaxed text-zinc-500 line-clamp-2">
          {agent.description}
        </p>
      )}
      {agent.tools && agent.tools.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {agent.tools.map((tool) => (
            <ToolBadge key={tool} tool={tool} />
          ))}
        </div>
      )}
    </Link>
  );
}

function PipelineConnector() {
  return (
    <div className="flex items-center justify-center py-1">
      <div className="flex flex-col items-center">
        <div className="h-4 w-px bg-gradient-to-b from-zinc-700 to-zinc-600" />
        <svg className="h-3 w-3 text-zinc-600" viewBox="0 0 12 12" fill="currentColor">
          <path d="M6 9L1 4h10L6 9z" />
        </svg>
      </div>
    </div>
  );
}

function WorkspaceSection({ workspace }: { workspace: Workspace }) {
  const isPipeline = workspace.agents.length > 1;

  return (
    <div className="rounded-2xl border border-zinc-800/50 bg-zinc-900/30 p-6">
      <div className="mb-4 flex items-center gap-3">
        <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-zinc-700 to-zinc-800 text-xs font-bold uppercase text-zinc-300">
          {workspace.name.charAt(0)}
        </div>
        <div>
          <h3 className="text-lg font-bold capitalize">{workspace.name}</h3>
          <p className="text-xs text-zinc-500">
            {workspace.agents.length} agent{workspace.agents.length > 1 ? "s" : ""}
            {isPipeline ? " · pipeline" : ""}
          </p>
        </div>
      </div>

      {isPipeline ? (
        <div className="space-y-0">
          {workspace.agents.map((agent, i) => (
            <div key={agent.name}>
              <AgentCard agent={agent} compact />
              {i < workspace.agents.length - 1 && <PipelineConnector />}
            </div>
          ))}
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {workspace.agents.map((agent) => (
            <AgentCard key={agent.name} agent={agent} />
          ))}
        </div>
      )}
    </div>
  );
}

export default function Home() {
  const [agents, setAgents] = useState<AgentInfo[]>([]);
  const [health, setHealth] = useState<HealthStatus | null>(null);
  const [serviceUp, setServiceUp] = useState<boolean | null>(null);

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

  // Separate orchestrator, standalone agents, and workspace groups
  const orchestrator = agents.find((a) => a.name === "winston");
  const standaloneAgents = agents.filter(
    (a) => a.name !== "winston" && !a.workspace
  );
  const workspaceMap = new Map<string, AgentInfo[]>();
  for (const agent of agents) {
    if (agent.workspace) {
      const list = workspaceMap.get(agent.workspace) || [];
      list.push(agent);
      workspaceMap.set(agent.workspace, list);
    }
  }
  const workspaces: Workspace[] = Array.from(workspaceMap.entries()).map(
    ([name, ws]) => ({ name, agents: ws })
  );

  return (
    <div className="min-h-screen bg-zinc-950 text-white">
      <header className="border-b border-zinc-800 px-6 py-4">
        <div className="mx-auto flex max-w-6xl items-center justify-between">
          <div className="flex items-center gap-4">
            <h1 className="text-2xl font-bold tracking-tight">Winston</h1>
            {serviceUp !== null && (
              <div className="flex items-center gap-2">
                <span
                  className={`inline-block h-2 w-2 rounded-full ${serviceUp ? "bg-green-500" : "bg-red-500"}`}
                />
                {health && (
                  <span className="text-xs text-zinc-500">
                    {health.agents} agents &middot; {health.uptime} uptime
                    {health.active_sessions > 0 &&
                      ` · ${health.active_sessions} sessions`}
                    {health.active_schedules > 0 &&
                      ` · ${health.active_schedules} schedules`}
                  </span>
                )}
                {serviceUp === false && (
                  <span className="text-xs text-red-400">
                    Service unreachable
                  </span>
                )}
              </div>
            )}
          </div>
          <nav className="flex gap-3">
            <Link
              href="/voice"
              className="rounded-lg bg-zinc-800 px-4 py-2 text-sm hover:bg-zinc-700"
            >
              Voice
            </Link>
            <Link
              href="/schedules"
              className="rounded-lg bg-zinc-800 px-4 py-2 text-sm hover:bg-zinc-700"
            >
              Schedules
            </Link>
          </nav>
        </div>
      </header>

      <main className="mx-auto max-w-6xl px-6 py-10">
        {/* Orchestrator */}
        {orchestrator && (
          <section className="mb-10">
            <h2 className="mb-1 text-xs font-medium uppercase tracking-widest text-zinc-500">
              Orchestrator
            </h2>
            <div className="mt-3">
              <Link
                href={`/agents/${orchestrator.name}`}
                className="group flex items-start gap-5 rounded-xl border border-zinc-800 bg-zinc-900 p-6 transition-all hover:border-amber-500/40 hover:bg-zinc-800/80"
              >
                <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-amber-500 to-amber-600 text-xl font-bold shadow-lg shadow-amber-500/20">
                  W
                </div>
                <div className="flex-1">
                  <div className="mb-1 flex items-center gap-3">
                    <h3 className="text-xl font-bold group-hover:text-amber-400">
                      Winston
                    </h3>
                    {orchestrator.model && (
                      <span className="rounded bg-zinc-800 px-2 py-0.5 text-xs text-zinc-400">
                        {orchestrator.model}
                      </span>
                    )}
                  </div>
                  <p className="mb-3 text-sm text-zinc-400">
                    {orchestrator.description}
                  </p>
                  {orchestrator.tools && orchestrator.tools.length > 0 && (
                    <div className="flex flex-wrap gap-1.5">
                      {orchestrator.tools.map((tool) => (
                        <ToolBadge key={tool} tool={tool} />
                      ))}
                    </div>
                  )}
                </div>
              </Link>
            </div>
          </section>
        )}

        {/* Workspaces */}
        {workspaces.length > 0 && (
          <section className="mb-10">
            <h2 className="mb-4 text-xs font-medium uppercase tracking-widest text-zinc-500">
              Workspaces
            </h2>
            <div className="grid gap-6 lg:grid-cols-2">
              {workspaces.map((ws) => (
                <WorkspaceSection key={ws.name} workspace={ws} />
              ))}
            </div>
          </section>
        )}

        {/* Standalone Agents */}
        {standaloneAgents.length > 0 && (
          <section>
            <h2 className="mb-4 text-xs font-medium uppercase tracking-widest text-zinc-500">
              Agents
            </h2>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {standaloneAgents.map((agent) => (
                <AgentCard key={agent.name} agent={agent} />
              ))}
            </div>
          </section>
        )}
      </main>
    </div>
  );
}
