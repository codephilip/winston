"use client";

import { useState, useRef, useEffect } from "react";
import Link from "next/link";

interface AgentInfo {
  name: string;
  description: string;
  workspace?: string;
  short_name?: string;
}

interface Message {
  role: "user" | "assistant";
  content: string;
}

export default function VoiceChat() {
  const [isRecording, setIsRecording] = useState(false);
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState("Tap to talk");
  const [agents, setAgents] = useState<AgentInfo[]>([]);
  const [selectedAgent, setSelectedAgent] = useState("winston");
  const [messages, setMessages] = useState<Message[]>([]);
  const mediaRecorder = useRef<MediaRecorder | null>(null);
  const audioChunks = useRef<Blob[]>([]);
  const messagesEnd = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEnd.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  useEffect(() => {
    fetch("/api/agents")
      .then((r) => r.json())
      .then((data) => setAgents(data || []))
      .catch(() => {});
  }, []);

  async function toggleRecording() {
    if (isRecording) {
      mediaRecorder.current?.stop();
      setIsRecording(false);
    } else {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({
          audio: true,
        });
        mediaRecorder.current = new MediaRecorder(stream, {
          mimeType: "audio/webm;codecs=opus",
        });
        audioChunks.current = [];

        mediaRecorder.current.ondataavailable = (e) => {
          if (e.data.size > 0) audioChunks.current.push(e.data);
        };

        mediaRecorder.current.onstop = async () => {
          const audioBlob = new Blob(audioChunks.current, {
            type: "audio/webm",
          });
          stream.getTracks().forEach((t) => t.stop());
          await processAudio(audioBlob);
        };

        mediaRecorder.current.start();
        setIsRecording(true);
        setStatus("Listening...");
      } catch {
        setStatus("Microphone access denied");
      }
    }
  }

  async function processAudio(audioBlob: Blob) {
    setLoading(true);

    setStatus("Transcribing...");
    try {
      const formData = new FormData();
      formData.append("audio", audioBlob, "recording.webm");

      const sttRes = await fetch(`/api/voice/transcribe`, {
        method: "POST",
        body: formData,
      });

      if (!sttRes.ok) {
        const err = await sttRes.json();
        setStatus(err.error || "Transcription failed");
        setLoading(false);
        return;
      }

      const { text } = await sttRes.json();
      if (!text) {
        setStatus("Couldn't hear you. Try again.");
        setLoading(false);
        return;
      }

      setMessages((prev) => [...prev, { role: "user", content: text }]);

      setStatus(`Asking /${selectedAgent}...`);
      const agentRes = await fetch(
        `/api/agents/${selectedAgent}/run`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ prompt: text }),
        }
      );

      const { result } = await agentRes.json();
      setMessages((prev) => [...prev, { role: "assistant", content: result }]);

      setStatus("Speaking...");
      const ttsRes = await fetch(`/api/voice/synthesize`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ text: result }),
      });

      if (ttsRes.ok) {
        const audioData = await ttsRes.blob();
        const audioUrl = URL.createObjectURL(audioData);
        const audio = new Audio(audioUrl);
        audio.onended = () => setStatus("Tap to talk");
        audio.play();
      } else {
        setStatus("Tap to talk");
      }
    } catch {
      setStatus("Connection failed. Is the router running?");
    } finally {
      setLoading(false);
    }
  }

  const currentAgent = agents.find((a) => a.name === selectedAgent);

  return (
    <div className="noise-bg relative flex h-[100dvh] flex-col overflow-hidden bg-[var(--surface-0)] text-white">
      {/* Ambient gradient orbs */}
      <div className="pointer-events-none fixed inset-0 z-0 overflow-hidden">
        <div className="absolute -left-32 top-1/4 h-[500px] w-[500px] rounded-full bg-indigo-600/[0.04] blur-[120px]" />
        <div className="absolute -right-32 bottom-1/4 h-[400px] w-[400px] rounded-full bg-violet-600/[0.04] blur-[120px]" />
      </div>

      {/* Header */}
      <header className="relative z-10 flex-shrink-0 border-b border-[var(--border)] bg-[var(--surface-0)]/80 px-5 py-3 backdrop-blur-2xl">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <Link href="/" className="rounded-lg p-1.5 text-zinc-500 transition-colors hover:bg-white/5 hover:text-white">
              <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
              </svg>
            </Link>
            <div className="flex items-center gap-2.5">
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-indigo-500 to-violet-600">
                <svg className="h-4 w-4 text-white" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z" />
                  <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z" />
                </svg>
              </div>
              <h1 className="text-lg font-semibold tracking-tight">Voice</h1>
            </div>
          </div>

          {/* Agent selector */}
          <div className="flex items-center gap-2">
            {currentAgent && (
              <span className="hidden text-xs text-zinc-500 sm:inline">
                {currentAgent.description?.slice(0, 40)}...
              </span>
            )}
            <select
              value={selectedAgent}
              onChange={(e) => setSelectedAgent(e.target.value)}
              className="rounded-lg border border-[var(--border)] bg-[var(--surface-2)] px-3 py-1.5 text-sm text-white transition-colors hover:border-zinc-600"
            >
              {agents.map((a) => (
                <option key={a.name} value={a.name}>
                  {a.workspace ? `${a.workspace} / ${a.short_name}` : a.name}
                </option>
              ))}
            </select>
          </div>
        </div>
      </header>

      {/* Messages */}
      <div className="relative z-10 flex-1 overflow-y-auto px-5 py-6">
        {messages.length === 0 && (
          <div className="flex h-full flex-col items-center justify-center gap-4">
            <div className="flex h-20 w-20 items-center justify-center rounded-full bg-gradient-to-br from-indigo-500/10 to-violet-600/10 ring-1 ring-inset ring-white/5">
              <svg className="h-10 w-10 text-indigo-400/60" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z" />
                <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z" />
              </svg>
            </div>
            <div className="text-center">
              <p className="text-lg font-medium text-zinc-300">Talk to /{selectedAgent}</p>
              <p className="mt-1 text-sm text-zinc-600">Tap the button below to start a conversation</p>
            </div>
          </div>
        )}
        <div className="mx-auto max-w-2xl space-y-4">
          {messages.map((msg, i) => (
            <div
              key={i}
              className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}
            >
              <div
                className={`max-w-[85%] rounded-2xl px-4 py-3 ${
                  msg.role === "user"
                    ? "bg-gradient-to-br from-indigo-600 to-indigo-700 text-white shadow-lg shadow-indigo-600/10"
                    : "glass-card text-zinc-200"
                }`}
              >
                <pre className="whitespace-pre-wrap font-sans text-sm leading-relaxed">
                  {msg.content}
                </pre>
              </div>
            </div>
          ))}
          <div ref={messagesEnd} />
        </div>
      </div>

      {/* Recording controls */}
      <div className="relative z-10 flex-shrink-0 border-t border-[var(--border)] bg-[var(--surface-0)]/80 px-5 pb-8 pt-5 backdrop-blur-2xl">
        <div className="flex flex-col items-center gap-4">
          <p className={`text-sm transition-colors ${isRecording ? "text-red-400" : loading ? "text-indigo-400" : "text-zinc-500"}`}>
            {status}
          </p>
          <button
            onClick={toggleRecording}
            disabled={loading}
            className={`group relative flex h-20 w-20 items-center justify-center rounded-full transition-all duration-300 active:scale-95 disabled:opacity-50 ${
              isRecording
                ? "bg-red-600 shadow-xl shadow-red-600/25"
                : loading
                  ? "bg-[var(--surface-3)]"
                  : "bg-gradient-to-br from-indigo-600 to-violet-600 shadow-xl shadow-indigo-600/20 hover:shadow-indigo-600/30"
            }`}
          >
            {/* Pulse ring when recording */}
            {isRecording && (
              <span className="absolute inset-0 animate-ping rounded-full bg-red-600/30" />
            )}
            {loading ? (
              <svg className="h-8 w-8 animate-spin text-white" viewBox="0 0 24 24" fill="none">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
              </svg>
            ) : isRecording ? (
              <svg className="relative h-8 w-8 text-white" viewBox="0 0 24 24" fill="currentColor">
                <rect x="6" y="6" width="12" height="12" rx="2" />
              </svg>
            ) : (
              <svg className="relative h-8 w-8 text-white" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z" />
                <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z" />
              </svg>
            )}
          </button>
          <p className="text-[11px] text-zinc-700">
            {selectedAgent && `/${selectedAgent}`}
          </p>
        </div>
      </div>
    </div>
  );
}
