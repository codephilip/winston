"use client";

import { useState, useEffect, useRef } from "react";
import Link from "next/link";

/* ── types ── */

interface AgentInfo {
  name: string;
  description: string;
  model?: string;
  workspace?: string;
  short_name?: string;
}

interface Schedule {
  id: string;
  agent_id: string;
  cron: string;
  prompt: string;
  slack_channel?: string;
  timezone?: string;
  status: string;
}

/* ── constants ── */

const DAYS = [
  { label: "Sun", value: 0 },
  { label: "Mon", value: 1 },
  { label: "Tue", value: 2 },
  { label: "Wed", value: 3 },
  { label: "Thu", value: 4 },
  { label: "Fri", value: 5 },
  { label: "Sat", value: 6 },
];

const REPEAT_OPTIONS = [
  { label: "Every day", value: "daily" },
  { label: "Weekdays", value: "weekdays" },
  { label: "Specific days", value: "specific" },
  { label: "Monthly (1st)", value: "monthly" },
];

const COMMON_TIMEZONES = [
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "America/Anchorage",
  "Pacific/Honolulu",
  "Europe/London",
  "Europe/Berlin",
  "Europe/Paris",
  "Asia/Tokyo",
  "Asia/Shanghai",
  "Asia/Kolkata",
  "Australia/Sydney",
  "UTC",
];

/* ── helpers ── */

function buildCron(
  hour: number,
  minute: number,
  repeat: string,
  selectedDays: number[]
): string {
  const dayPart =
    repeat === "daily"
      ? "*"
      : repeat === "weekdays"
        ? "1-5"
        : repeat === "monthly"
          ? "*"
          : selectedDays.sort().join(",") || "*";
  const dayOfMonth = repeat === "monthly" ? "1" : "*";
  return `${minute} ${hour} ${dayOfMonth} * ${dayPart}`;
}

function cronToHuman(cron: string): string {
  const parts = cron.split(" ");
  if (parts.length !== 5) return cron;
  const [min, hr, dom, , dow] = parts;
  const h = parseInt(hr);
  const m = parseInt(min);
  const ampm = h >= 12 ? "PM" : "AM";
  const h12 = h === 0 ? 12 : h > 12 ? h - 12 : h;
  const time = `${h12}:${m.toString().padStart(2, "0")} ${ampm}`;

  if (dom === "1") return `1st of every month at ${time}`;
  if (dow === "*") return `Every day at ${time}`;
  if (dow === "1-5") return `Weekdays at ${time}`;

  const dayNames = dow.split(",").map((d) => {
    const day = DAYS.find((x) => x.value === parseInt(d));
    return day?.label || d;
  });
  return `${dayNames.join(", ")} at ${time}`;
}

function groupAgentsByWorkspace(agents: AgentInfo[]): {
  workspaces: { name: string; agents: AgentInfo[] }[];
  standalone: AgentInfo[];
} {
  const wsMap = new Map<string, AgentInfo[]>();
  const standalone: AgentInfo[] = [];

  for (const agent of agents) {
    if (agent.workspace) {
      const list = wsMap.get(agent.workspace) || [];
      list.push(agent);
      wsMap.set(agent.workspace, list);
    } else {
      standalone.push(agent);
    }
  }

  const workspaces = Array.from(wsMap.entries())
    .map(([name, agents]) => ({ name, agents }))
    .sort((a, b) => a.name.localeCompare(b.name));

  return { workspaces, standalone };
}

function shortTz(tz: string): string {
  const parts = tz.split("/");
  return parts[parts.length - 1].replace(/_/g, " ");
}

/* ── workspace dropdown for filtering ── */

function WorkspaceFilter({
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
      if (ref.current && !ref.current.contains(e.target as Node))
        setOpen(false);
    }
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, []);

  if (workspaces.length === 0) return null;

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-2 rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-1.5 text-xs font-medium capitalize transition-colors hover:border-zinc-700"
      >
        <span className="h-2 w-2 rounded-full bg-gradient-to-br from-blue-400 to-violet-500" />
        {active === null ? "All agents" : active}
        <svg
          className={`h-3 w-3 text-zinc-500 transition-transform ${open ? "rotate-180" : ""}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
        </svg>
      </button>
      {open && (
        <div className="absolute left-0 top-full z-20 mt-1 min-w-[180px] overflow-hidden rounded-xl border border-zinc-800 bg-zinc-900 shadow-2xl shadow-black/50">
          <div className="p-1">
            <button
              onClick={() => {
                onChange(null);
                setOpen(false);
              }}
              className={`flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm transition-colors hover:bg-zinc-800 ${active === null ? "bg-zinc-800 text-white" : "text-zinc-400"}`}
            >
              All agents
            </button>
            <button
              onClick={() => {
                onChange("personal");
                setOpen(false);
              }}
              className={`flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm transition-colors hover:bg-zinc-800 ${active === "personal" ? "bg-zinc-800 text-white" : "text-zinc-400"}`}
            >
              <span className="h-2 w-2 rounded-full bg-amber-500" />
              Personal
            </button>
            {workspaces.map((ws) => (
              <button
                key={ws}
                onClick={() => {
                  onChange(ws);
                  setOpen(false);
                }}
                className={`flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm capitalize transition-colors hover:bg-zinc-800 ${active === ws ? "bg-zinc-800 text-white" : "text-zinc-400"}`}
              >
                <span className="h-2 w-2 rounded-full bg-violet-500" />
                {ws}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

/* ── main page ── */

export default function Schedules() {
  const [agents, setAgents] = useState<AgentInfo[]>([]);
  const [schedules, setSchedules] = useState<Schedule[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [wsFilter, setWsFilter] = useState<string | null>(null);

  // Schedule builder state
  const [hour, setHour] = useState(9);
  const [minute, setMinute] = useState(0);
  const [ampm, setAmpm] = useState<"AM" | "PM">("AM");
  const [repeat, setRepeat] = useState("weekdays");
  const [selectedDays, setSelectedDays] = useState<number[]>([1]);
  const [selectedAgent, setSelectedAgent] = useState("");
  const [prompt, setPrompt] = useState("");
  const [slackChannel, setSlackChannel] = useState("#polymr-personal");
  const [timezone, setTimezone] = useState("");

  useEffect(() => {
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    setTimezone(tz);
    fetchAgents();
    fetchSchedules();
  }, []);

  async function fetchAgents() {
    try {
      const res = await fetch("/api/agents");
      const data = await res.json();
      setAgents(data || []);
      // Auto-select first agent if none selected
      if (data?.length && !selectedAgent) {
        setSelectedAgent(data[0].name);
      }
    } catch {
      /* api not running */
    }
  }

  async function fetchSchedules() {
    try {
      const res = await fetch("/api/schedules");
      const data = await res.json();
      setSchedules(data || []);
    } catch {
      /* api not running */
    }
  }

  function toggleDay(day: number) {
    setSelectedDays((prev) =>
      prev.includes(day) ? prev.filter((d) => d !== day) : [...prev, day]
    );
  }

  async function createSchedule(e: React.FormEvent) {
    e.preventDefault();
    const h24 =
      ampm === "PM" ? (hour === 12 ? 12 : hour + 12) : hour === 12 ? 0 : hour;
    const cron = buildCron(h24, minute, repeat, selectedDays);
    try {
      await fetch("/api/schedules", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          agent_id: selectedAgent,
          cron,
          prompt,
          slack_channel: slackChannel,
          timezone,
        }),
      });
      setShowForm(false);
      fetchSchedules();
    } catch {
      alert("Failed to create schedule");
    }
  }

  async function deleteSchedule(id: string) {
    try {
      await fetch(`/api/schedules/${id}`, { method: "DELETE" });
      fetchSchedules();
    } catch {
      alert("Failed to delete schedule");
    }
  }

  // Group agents for the selector
  const { workspaces, standalone } = groupAgentsByWorkspace(agents);
  const workspaceNames = workspaces.map((w) => w.name);

  // Filter schedules by workspace
  const filteredSchedules = schedules.filter((s) => {
    if (wsFilter === null) return true;
    if (wsFilter === "personal") {
      const agent = agents.find((a) => a.name === s.agent_id);
      return !agent?.workspace;
    }
    const agent = agents.find((a) => a.name === s.agent_id);
    return agent?.workspace === wsFilter;
  });

  // Filter agents for selector based on workspace filter
  const selectableAgents =
    wsFilter === null
      ? agents
      : wsFilter === "personal"
        ? standalone
        : agents.filter((a) => a.workspace === wsFilter);

  return (
    <div className="flex min-h-screen flex-col bg-zinc-950 text-white">
      <header className="border-b border-zinc-800 px-6 py-4">
        <div className="mx-auto flex max-w-4xl items-center gap-4">
          <Link href="/" className="text-zinc-400 hover:text-white">
            &larr;
          </Link>
          <h1 className="text-xl font-bold">Scheduled Agents</h1>
        </div>
      </header>

      <main className="mx-auto w-full max-w-4xl px-6 py-8">
        <div className="mb-6 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div>
              <p className="text-zinc-400">
                Schedule recurring agent runs. Results post to Slack.
              </p>
              {timezone && (
                <p className="mt-1 text-xs text-zinc-600">
                  All times in {timezone}
                </p>
              )}
            </div>
            <WorkspaceFilter
              workspaces={workspaceNames}
              active={wsFilter}
              onChange={setWsFilter}
            />
          </div>
          <button
            onClick={() => setShowForm(!showForm)}
            className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium hover:bg-blue-500"
          >
            New Schedule
          </button>
        </div>

        {showForm && (
          <form
            onSubmit={createSchedule}
            className="mb-8 rounded-xl border border-zinc-800 bg-zinc-900 p-6"
          >
            {/* Agent selector — grouped by workspace */}
            <div className="mb-5">
              <label className="mb-1.5 block text-sm font-medium text-zinc-300">
                Agent
              </label>

              {/* Workspace tabs if multiple workspaces exist */}
              {workspaces.length > 0 && (
                <div className="mb-3 space-y-3">
                  {standalone.length > 0 && (
                    <div>
                      <p className="mb-1.5 text-[11px] font-medium uppercase tracking-widest text-zinc-600">
                        Personal
                      </p>
                      <div className="flex flex-wrap gap-2">
                        {standalone.map((agent) => (
                          <button
                            key={agent.name}
                            type="button"
                            onClick={() => {
                              setSelectedAgent(agent.name);
                              setPrompt("");
                            }}
                            className={`rounded-lg border px-3 py-2 text-sm font-medium capitalize transition-all ${
                              selectedAgent === agent.name
                                ? "border-blue-500 bg-blue-500/20 text-blue-400"
                                : "border-zinc-700 bg-zinc-800 text-zinc-400 hover:border-zinc-600"
                            }`}
                          >
                            {agent.short_name || agent.name}
                          </button>
                        ))}
                      </div>
                    </div>
                  )}

                  {workspaces.map((ws) => (
                    <div key={ws.name}>
                      <p className="mb-1.5 text-[11px] font-medium uppercase tracking-widest text-zinc-600">
                        <span className="mr-1.5 inline-block h-2 w-2 rounded-full bg-violet-500" />
                        {ws.name}
                      </p>
                      <div className="flex flex-wrap gap-2">
                        {ws.agents.map((agent) => (
                          <button
                            key={agent.name}
                            type="button"
                            onClick={() => {
                              setSelectedAgent(agent.name);
                              setPrompt("");
                            }}
                            className={`rounded-lg border px-3 py-2 text-sm font-medium capitalize transition-all ${
                              selectedAgent === agent.name
                                ? "border-violet-500 bg-violet-500/20 text-violet-400"
                                : "border-zinc-700 bg-zinc-800 text-zinc-400 hover:border-zinc-600"
                            }`}
                          >
                            {agent.short_name || agent.name}
                          </button>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              )}

              {/* Fallback: flat list if no workspaces */}
              {workspaces.length === 0 && agents.length > 0 && (
                <div className="flex flex-wrap gap-2">
                  {agents.map((agent) => (
                    <button
                      key={agent.name}
                      type="button"
                      onClick={() => {
                        setSelectedAgent(agent.name);
                        setPrompt("");
                      }}
                      className={`rounded-lg border px-3 py-2 text-sm font-medium capitalize transition-all ${
                        selectedAgent === agent.name
                          ? "border-blue-500 bg-blue-500/20 text-blue-400"
                          : "border-zinc-700 bg-zinc-800 text-zinc-400 hover:border-zinc-600"
                      }`}
                    >
                      {agent.short_name || agent.name}
                    </button>
                  ))}
                </div>
              )}

              {/* Selected agent description */}
              {selectedAgent && (
                <p className="mt-2 text-xs text-zinc-500">
                  {agents.find((a) => a.name === selectedAgent)?.description}
                </p>
              )}
            </div>

            {/* Time Picker */}
            <div className="mb-5">
              <label className="mb-1.5 block text-sm font-medium text-zinc-300">
                Time
              </label>
              <div className="flex items-center gap-2">
                <select
                  value={hour}
                  onChange={(e) => setHour(parseInt(e.target.value))}
                  className="rounded-lg border border-zinc-700 bg-zinc-800 px-3 py-2.5 text-center text-lg tabular-nums"
                >
                  {Array.from({ length: 12 }, (_, i) => i + 1).map((h) => (
                    <option key={h} value={h}>
                      {h}
                    </option>
                  ))}
                </select>
                <span className="text-xl text-zinc-500">:</span>
                <select
                  value={minute}
                  onChange={(e) => setMinute(parseInt(e.target.value))}
                  className="rounded-lg border border-zinc-700 bg-zinc-800 px-3 py-2.5 text-center text-lg tabular-nums"
                >
                  {Array.from({ length: 60 }, (_, i) => i).map((m) => (
                    <option key={m} value={m}>
                      {m.toString().padStart(2, "0")}
                    </option>
                  ))}
                </select>
                <div className="flex overflow-hidden rounded-lg border border-zinc-700">
                  {(["AM", "PM"] as const).map((p) => (
                    <button
                      key={p}
                      type="button"
                      onClick={() => setAmpm(p)}
                      className={`px-3 py-2.5 text-sm font-medium transition-all ${
                        ampm === p
                          ? "bg-blue-600 text-white"
                          : "bg-zinc-800 text-zinc-400 hover:bg-zinc-700"
                      }`}
                    >
                      {p}
                    </button>
                  ))}
                </div>
              </div>
            </div>

            {/* Timezone */}
            <div className="mb-5">
              <label className="mb-1.5 block text-sm font-medium text-zinc-300">
                Timezone
              </label>
              <select
                value={timezone}
                onChange={(e) => setTimezone(e.target.value)}
                className="w-full rounded-lg border border-zinc-700 bg-zinc-800 px-3 py-2.5 text-sm"
              >
                {/* Current timezone first if not in common list */}
                {timezone &&
                  !COMMON_TIMEZONES.includes(timezone) && (
                    <option value={timezone}>{timezone} (local)</option>
                  )}
                {COMMON_TIMEZONES.map((tz) => (
                  <option key={tz} value={tz}>
                    {tz === timezone ? `${tz} (local)` : tz}
                  </option>
                ))}
              </select>
            </div>

            {/* Repeat */}
            <div className="mb-5">
              <label className="mb-1.5 block text-sm font-medium text-zinc-300">
                Repeat
              </label>
              <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                {REPEAT_OPTIONS.map((opt) => (
                  <button
                    key={opt.value}
                    type="button"
                    onClick={() => setRepeat(opt.value)}
                    className={`rounded-lg border px-3 py-2 text-sm transition-all ${
                      repeat === opt.value
                        ? "border-blue-500 bg-blue-500/20 text-blue-400"
                        : "border-zinc-700 bg-zinc-800 text-zinc-400 hover:border-zinc-600"
                    }`}
                  >
                    {opt.label}
                  </button>
                ))}
              </div>
            </div>

            {/* Day Picker (specific days) */}
            {repeat === "specific" && (
              <div className="mb-5">
                <label className="mb-1.5 block text-sm font-medium text-zinc-300">
                  Days
                </label>
                <div className="flex gap-1.5">
                  {DAYS.map((day) => (
                    <button
                      key={day.value}
                      type="button"
                      onClick={() => toggleDay(day.value)}
                      className={`flex h-10 w-10 items-center justify-center rounded-full text-xs font-medium transition-all ${
                        selectedDays.includes(day.value)
                          ? "bg-blue-600 text-white"
                          : "bg-zinc-800 text-zinc-500 hover:bg-zinc-700"
                      }`}
                    >
                      {day.label}
                    </button>
                  ))}
                </div>
              </div>
            )}

            {/* Prompt */}
            <div className="mb-5">
              <label className="mb-1.5 block text-sm font-medium text-zinc-300">
                Prompt
              </label>
              <textarea
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                placeholder="What should the agent do on each run?"
                rows={3}
                className="w-full rounded-lg border border-zinc-700 bg-zinc-800 px-3 py-2.5 text-sm"
              />
            </div>

            {/* Slack Channel */}
            <div className="mb-5">
              <label className="mb-1.5 block text-sm font-medium text-zinc-300">
                Slack Channel
              </label>
              <input
                type="text"
                value={slackChannel}
                onChange={(e) => setSlackChannel(e.target.value)}
                placeholder="#channel-name"
                className="w-full rounded-lg border border-zinc-700 bg-zinc-800 px-3 py-2.5 text-sm"
              />
            </div>

            {/* Preview + Actions */}
            <div className="flex items-center justify-between rounded-lg bg-zinc-800/50 px-4 py-3">
              <div>
                <p className="text-sm text-zinc-400">
                  <span className="font-medium capitalize text-zinc-300">
                    {agents.find((a) => a.name === selectedAgent)?.short_name ||
                      selectedAgent}
                  </span>
                  {" — "}
                  {cronToHuman(
                    buildCron(
                      ampm === "PM"
                        ? hour === 12
                          ? 12
                          : hour + 12
                        : hour === 12
                          ? 0
                          : hour,
                      minute,
                      repeat,
                      selectedDays
                    )
                  )}
                </p>
                <p className="mt-0.5 text-xs text-zinc-600">
                  {shortTz(timezone)}
                  {slackChannel && ` · ${slackChannel}`}
                </p>
              </div>
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => setShowForm(false)}
                  className="rounded-lg bg-zinc-700 px-4 py-2 text-sm hover:bg-zinc-600"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium hover:bg-blue-500"
                >
                  Create
                </button>
              </div>
            </div>
          </form>
        )}

        {/* Schedule list */}
        <div className="space-y-3">
          {filteredSchedules.length === 0 && (
            <p className="py-12 text-center text-zinc-600">
              {schedules.length === 0
                ? "No scheduled agents yet. Create one to get started."
                : "No schedules in this workspace."}
            </p>
          )}
          {filteredSchedules.map((sched) => {
            const agent = agents.find((a) => a.name === sched.agent_id);
            return (
              <div
                key={sched.id}
                className="flex items-center justify-between rounded-xl border border-zinc-800 bg-zinc-900 p-4"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    {agent?.workspace && (
                      <span className="rounded bg-violet-900/50 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-violet-400">
                        {agent.workspace}
                      </span>
                    )}
                    <span className="font-medium capitalize">
                      {agent?.short_name || sched.agent_id}
                    </span>
                    <span
                      className={`rounded px-2 py-0.5 text-xs ${
                        sched.status === "active"
                          ? "bg-green-900 text-green-300"
                          : "bg-zinc-800 text-zinc-500"
                      }`}
                    >
                      {sched.status}
                    </span>
                  </div>
                  <p className="mt-1 truncate text-sm text-zinc-400">
                    {sched.prompt}
                  </p>
                  <p className="mt-1 text-xs text-zinc-500">
                    {cronToHuman(sched.cron)}
                    {sched.timezone && ` · ${shortTz(sched.timezone)}`}
                    {sched.slack_channel && ` · ${sched.slack_channel}`}
                  </p>
                </div>
                <button
                  onClick={() => deleteSchedule(sched.id)}
                  className="ml-3 shrink-0 rounded-lg bg-zinc-800 px-3 py-1.5 text-sm text-red-400 hover:bg-zinc-700"
                >
                  Delete
                </button>
              </div>
            );
          })}
        </div>
      </main>
    </div>
  );
}
