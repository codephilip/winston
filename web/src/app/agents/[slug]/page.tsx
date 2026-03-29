"use client";

import { useState, useRef, useEffect } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";

interface Message {
  role: "user" | "assistant";
  content: string;
  timestamp: Date;
}

const MODELS = ["haiku", "sonnet", "opus"] as const;
type Model = (typeof MODELS)[number];

export default function AgentChat() {
  const { slug } = useParams<{ slug: string }>();
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [model, setModel] = useState<Model | null>(null);
  const [modelLoading, setModelLoading] = useState(false);
  const messagesEnd = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEnd.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  useEffect(() => {
    async function fetchModel() {
      try {
        const res = await fetch("/api/agents");
        const agents = await res.json();
        const agent = agents.find((a: { name: string }) => a.name === slug);
        if (agent?.model) setModel(agent.model);
      } catch {
        // router not running
      }
    }
    fetchModel();
  }, [slug]);

  async function changeModel(newModel: Model) {
    if (newModel === model || modelLoading) return;
    setModelLoading(true);
    try {
      const res = await fetch(`/api/agents/${slug}/model`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ model: newModel }),
      });
      if (res.ok) {
        const data = await res.json();
        setModel(data.new_model);
        if (data.restart) {
          setMessages((prev) => [
            ...prev,
            {
              role: "assistant",
              content: `Model changed to ${data.new_model}. Services are restarting — this may take a moment.`,
              timestamp: new Date(),
            },
          ]);
        }
      }
    } catch {
      // connection error
    } finally {
      setModelLoading(false);
    }
  }

  async function sendMessage(e: React.FormEvent) {
    e.preventDefault();
    if (!input.trim() || loading) return;

    const userMsg: Message = {
      role: "user",
      content: input,
      timestamp: new Date(),
    };
    setMessages((prev) => [...prev, userMsg]);
    setInput("");
    setLoading(true);

    try {
      const res = await fetch(`/api/agents/${slug}/run`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ prompt: input }),
      });
      const data = await res.json();
      setMessages((prev) => [
        ...prev,
        {
          role: "assistant",
          content: data.result || data.error || "No response",
          timestamp: new Date(),
        },
      ]);
    } catch {
      setMessages((prev) => [
        ...prev,
        {
          role: "assistant",
          content: "Failed to reach agent. Is the router running?",
          timestamp: new Date(),
        },
      ]);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex min-h-screen flex-col bg-zinc-950 text-white">
      <header className="border-b border-zinc-800 px-6 py-4">
        <div className="mx-auto flex max-w-4xl items-center justify-between">
          <div className="flex items-center gap-4">
            <Link href="/" className="text-zinc-400 hover:text-white">
              &larr;
            </Link>
            <h1 className="text-xl font-bold capitalize">/{slug}</h1>
          </div>
          {model && (
            <div className="flex items-center gap-1 rounded-lg border border-zinc-800 bg-zinc-900 p-1">
              {MODELS.map((m) => (
                <button
                  key={m}
                  onClick={() => changeModel(m)}
                  disabled={modelLoading}
                  className={`rounded-md px-3 py-1.5 text-xs font-medium transition-all ${
                    model === m
                      ? "bg-zinc-700 text-white"
                      : "text-zinc-500 hover:text-zinc-300"
                  } ${modelLoading ? "opacity-50" : ""}`}
                >
                  {m}
                </button>
              ))}
            </div>
          )}
        </div>
      </header>

      <main className="mx-auto flex w-full max-w-4xl flex-1 flex-col px-6 py-6">
        <div className="flex-1 space-y-4 overflow-y-auto">
          {messages.length === 0 && (
            <div className="flex h-full items-center justify-center text-zinc-600">
              <p>Send a message to start a conversation with /{slug}</p>
            </div>
          )}
          {messages.map((msg, i) => (
            <div
              key={i}
              className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}
            >
              <div
                className={`max-w-[80%] rounded-xl px-4 py-3 ${
                  msg.role === "user"
                    ? "bg-blue-600 text-white"
                    : "bg-zinc-800 text-zinc-100"
                }`}
              >
                <pre className="whitespace-pre-wrap font-sans text-sm">
                  {msg.content}
                </pre>
              </div>
            </div>
          ))}
          {loading && (
            <div className="flex justify-start">
              <div className="rounded-xl bg-zinc-800 px-4 py-3 text-zinc-400">
                Thinking...
              </div>
            </div>
          )}
          <div ref={messagesEnd} />
        </div>

        <form onSubmit={sendMessage} className="mt-4 flex gap-3">
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder={`Message /${slug}...`}
            className="flex-1 rounded-xl border border-zinc-700 bg-zinc-900 px-4 py-3 text-white placeholder-zinc-500 focus:border-zinc-500 focus:outline-none"
          />
          <button
            type="submit"
            disabled={loading || !input.trim()}
            className="rounded-xl bg-blue-600 px-6 py-3 font-medium hover:bg-blue-500 disabled:opacity-50"
          >
            Send
          </button>
        </form>
      </main>
    </div>
  );
}
