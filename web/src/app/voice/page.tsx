"use client";

import { useState, useRef, useEffect } from "react";
import Link from "next/link";

interface Message {
  role: "user" | "assistant";
  content: string;
}

export default function VoiceChat() {
  const [isRecording, setIsRecording] = useState(false);
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState("Tap to talk");
  const [selectedAgent, setSelectedAgent] = useState("winston");
  const [messages, setMessages] = useState<Message[]>([]);
  const mediaRecorder = useRef<MediaRecorder | null>(null);
  const audioChunks = useRef<Blob[]>([]);
  const messagesEnd = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEnd.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

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

  return (
    <div className="flex h-[100dvh] flex-col bg-zinc-950 text-white">
      <header className="flex-shrink-0 border-b border-zinc-800 px-4 py-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Link href="/" className="text-zinc-400 hover:text-white">
              &larr;
            </Link>
            <h1 className="text-lg font-bold">Voice</h1>
          </div>
          <select
            value={selectedAgent}
            onChange={(e) => setSelectedAgent(e.target.value)}
            className="rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-1.5 text-sm text-white"
          >
            <optgroup label="Orchestrator">
              <option value="winston">Winston</option>
            </optgroup>
            <optgroup label="Agents">
              <option value="marketing">Marketing</option>
              <option value="pentester">Pentester</option>
              <option value="youtube">YouTube</option>
              <option value="designer">Designer</option>
            </optgroup>
          </select>
        </div>
      </header>

      <div className="flex-1 overflow-y-auto px-4 py-4">
        {messages.length === 0 && (
          <div className="flex h-full flex-col items-center justify-center gap-2 text-zinc-600">
            <p className="text-lg">Talk to /{selectedAgent}</p>
            <p className="text-sm">Tap the button below to start</p>
          </div>
        )}
        <div className="space-y-3">
          {messages.map((msg, i) => (
            <div
              key={i}
              className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}
            >
              <div
                className={`max-w-[85%] rounded-2xl px-4 py-2.5 ${
                  msg.role === "user"
                    ? "bg-blue-600 text-white"
                    : "bg-zinc-800 text-zinc-100"
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

      <div className="flex-shrink-0 border-t border-zinc-800 px-4 pb-8 pt-4">
        <div className="flex flex-col items-center gap-3">
          <p className="text-sm text-zinc-400">{status}</p>
          <button
            onClick={toggleRecording}
            disabled={loading}
            className={`flex h-20 w-20 items-center justify-center rounded-full transition-all active:scale-95 ${
              isRecording
                ? "animate-pulse bg-red-600 shadow-lg shadow-red-600/30"
                : loading
                  ? "bg-zinc-700"
                  : "bg-blue-600 shadow-lg shadow-blue-600/30"
            } disabled:opacity-50`}
          >
            {loading ? (
              <svg className="h-8 w-8 animate-spin text-white" viewBox="0 0 24 24" fill="none">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
              </svg>
            ) : isRecording ? (
              <svg className="h-8 w-8 text-white" viewBox="0 0 24 24" fill="currentColor">
                <rect x="6" y="6" width="12" height="12" rx="2" />
              </svg>
            ) : (
              <svg className="h-8 w-8 text-white" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z" />
                <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z" />
              </svg>
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
