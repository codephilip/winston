"use client";

import { useState, useEffect } from "react";
import Link from "next/link";

const PROMPT_DEFAULTS: Record<string, string> = {
  marketing: "Check in with a daily marketing briefing",
  pentester: "Run a weekly security scan summary",
  youtube:
    "Run weekly video prep — find trends, suggest 3 topics, and wait for confirmation in Slack",
};

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

interface Schedule {
  id: string;
  agent_id: string;
  cron: string;
  prompt: string;
  slack_channel?: string;
  status: string;
}

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

export default function Schedules() {
  const [schedules, setSchedules] = useState<Schedule[]>([]);
  const [showForm, setShowForm] = useState(false);

  // Schedule builder state
  const [hour, setHour] = useState(9);
  const [minute, setMinute] = useState(0);
  const [ampm, setAmpm] = useState<"AM" | "PM">("AM");
  const [repeat, setRepeat] = useState("weekdays");
  const [selectedDays, setSelectedDays] = useState<number[]>([1]);
  const [form, setForm] = useState({
    agent_id: "marketing",
    prompt: PROMPT_DEFAULTS["marketing"],
    slack_channel: "#polymr-personal",
  });

  const [timezone, setTimezone] = useState("");

  useEffect(() => {
    setTimezone(Intl.DateTimeFormat().resolvedOptions().timeZone);
    fetchSchedules();
  }, []);

  function toggleDay(day: number) {
    setSelectedDays((prev) =>
      prev.includes(day) ? prev.filter((d) => d !== day) : [...prev, day]
    );
  }

  async function fetchSchedules() {
    try {
      const res = await fetch("/api/schedules");
      const data = await res.json();
      setSchedules(data || []);
    } catch {
      /* router not running */
    }
  }

  async function createSchedule(e: React.FormEvent) {
    e.preventDefault();
    const h24 = ampm === "PM" ? (hour === 12 ? 12 : hour + 12) : hour === 12 ? 0 : hour;
    const cron = buildCron(h24, minute, repeat, selectedDays);
    try {
      await fetch("/api/schedules", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...form, cron }),
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
            {/* Agent */}
            <div className="mb-5">
              <label className="mb-1.5 block text-sm font-medium text-zinc-300">
                Agent
              </label>
              <div className="grid grid-cols-3 gap-2">
                {["marketing", "pentester", "youtube"].map((id) => (
                  <button
                    key={id}
                    type="button"
                    onClick={() =>
                      setForm({
                        ...form,
                        agent_id: id,
                        prompt: PROMPT_DEFAULTS[id] || "",
                      })
                    }
                    className={`rounded-lg border px-3 py-2.5 text-sm font-medium capitalize transition-all ${
                      form.agent_id === id
                        ? "border-blue-500 bg-blue-500/20 text-blue-400"
                        : "border-zinc-700 bg-zinc-800 text-zinc-400 hover:border-zinc-600"
                    }`}
                  >
                    {id}
                  </button>
                ))}
              </div>
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
                value={form.prompt}
                onChange={(e) => setForm({ ...form, prompt: e.target.value })}
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
                value={form.slack_channel}
                onChange={(e) =>
                  setForm({ ...form, slack_channel: e.target.value })
                }
                placeholder="#channel-name"
                className="w-full rounded-lg border border-zinc-700 bg-zinc-800 px-3 py-2.5 text-sm"
              />
            </div>

            {/* Preview + Actions */}
            <div className="flex items-center justify-between rounded-lg bg-zinc-800/50 px-4 py-3">
              <div>
              <p className="text-sm text-zinc-400">
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
              {timezone && (
                <p className="mt-0.5 text-xs text-zinc-600">{timezone}</p>
              )}
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

        <div className="space-y-3">
          {schedules.length === 0 && (
            <p className="py-12 text-center text-zinc-600">
              No scheduled agents yet. Create one to get started.
            </p>
          )}
          {schedules.map((sched) => (
            <div
              key={sched.id}
              className="flex items-center justify-between rounded-xl border border-zinc-800 bg-zinc-900 p-4"
            >
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="font-medium capitalize">
                    {sched.agent_id}
                  </span>
                  <span className="rounded bg-green-900 px-2 py-0.5 text-xs text-green-300">
                    {sched.status}
                  </span>
                </div>
                <p className="mt-1 truncate text-sm text-zinc-400">
                  {sched.prompt}
                </p>
                <p className="mt-1 text-xs text-zinc-500">
                  {cronToHuman(sched.cron)}
                  {sched.slack_channel && ` | ${sched.slack_channel}`}
                </p>
              </div>
              <button
                onClick={() => deleteSchedule(sched.id)}
                className="ml-3 shrink-0 rounded-lg bg-zinc-800 px-3 py-1.5 text-sm text-red-400 hover:bg-zinc-700"
              >
                Delete
              </button>
            </div>
          ))}
        </div>
      </main>
    </div>
  );
}
