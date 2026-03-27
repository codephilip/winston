import Link from "next/link";

const orchestrator = {
  name: "Winston",
  slug: "winston",
  description:
    "Personal assistant and orchestrator. Manages all agents, tools, and system resources. Ask him anything.",
  icon: "W",
  color: "bg-amber-500",
};

const agents = [
  {
    name: "Marketing",
    slug: "marketing",
    description:
      "Competitive intelligence, SEO audits, content generation, and campaign analysis",
    icon: "M",
    color: "bg-blue-500",
  },
  {
    name: "Pentester",
    slug: "pentester",
    description:
      "Offensive security — recon, vulnerability scanning, exploitation, and reporting via Kali VM",
    icon: "P",
    color: "bg-red-500",
  },
  {
    name: "YouTube",
    slug: "youtube",
    description:
      "Full video production pipeline — trend research, viral scripts, thumbnails, and SEO",
    icon: "Y",
    color: "bg-purple-500",
  },
  {
    name: "Designer",
    slug: "designer",
    description:
      "Elite frontend design — inspiration research, mockup generation, Figma builds, and production-ready UI",
    icon: "D",
    color: "bg-emerald-500",
  },
];

export default function Home() {
  return (
    <div className="min-h-screen bg-zinc-950 text-white">
      <header className="border-b border-zinc-800 px-6 py-4">
        <div className="mx-auto flex max-w-6xl items-center justify-between">
          <h1 className="text-2xl font-bold tracking-tight">Winston</h1>
          <nav className="flex gap-4">
            <Link
              href="/voice"
              className="rounded-lg bg-zinc-800 px-4 py-2 text-sm hover:bg-zinc-700"
            >
              Voice Chat
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

      <main className="mx-auto max-w-6xl px-6 py-12">
        {/* Orchestrator */}
        <section className="mb-12">
          <h2 className="mb-2 text-3xl font-bold">Orchestrator</h2>
          <p className="mb-6 text-zinc-400">
            Your personal assistant with full access to every agent, skill, and
            tool on this machine.
          </p>
          <Link
            href={`/agents/${orchestrator.slug}`}
            className="group flex items-start gap-6 rounded-xl border border-zinc-800 bg-zinc-900 p-6 transition-all hover:border-amber-500/50 hover:bg-zinc-800"
          >
            <div
              className={`flex h-14 w-14 shrink-0 items-center justify-center rounded-lg ${orchestrator.color} text-xl font-bold`}
            >
              {orchestrator.icon}
            </div>
            <div>
              <h3 className="mb-1 text-xl font-semibold group-hover:text-amber-400">
                {orchestrator.name}
              </h3>
              <p className="text-sm text-zinc-400">
                {orchestrator.description}
              </p>
            </div>
          </Link>
        </section>

        {/* Agents */}
        <section>
          <h2 className="mb-2 text-3xl font-bold">Agents</h2>
          <p className="mb-6 text-zinc-400">
            Specialist agents for specific workflows. Chat directly or let
            Winston delegate.
          </p>
          <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
            {agents.map((agent) => (
              <Link
                key={agent.slug}
                href={`/agents/${agent.slug}`}
                className="group rounded-xl border border-zinc-800 bg-zinc-900 p-6 transition-all hover:border-zinc-600 hover:bg-zinc-800"
              >
                <div
                  className={`mb-4 flex h-12 w-12 items-center justify-center rounded-lg ${agent.color} text-lg font-bold`}
                >
                  {agent.icon}
                </div>
                <h3 className="mb-2 text-xl font-semibold group-hover:text-white">
                  {agent.name}
                </h3>
                <p className="text-sm text-zinc-400">{agent.description}</p>
              </Link>
            ))}
          </div>
        </section>
      </main>
    </div>
  );
}
