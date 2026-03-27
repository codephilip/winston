"use client";

import { createContext, useContext, useState, useEffect, ReactNode } from "react";

interface AuthContextType {
  credentials: string | null;
  login: (user: string, pass: string) => Promise<boolean>;
  logout: () => void;
  authHeaders: () => HeadersInit;
}

const AuthContext = createContext<AuthContextType | null>(null);

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be inside AuthProvider");
  return ctx;
}

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export function AuthProvider({ children }: { children: ReactNode }) {
  const [credentials, setCredentials] = useState<string | null>(null);

  useEffect(() => {
    const saved = localStorage.getItem("polymr_auth");
    if (saved) setCredentials(saved);
  }, []);

  async function login(user: string, pass: string): Promise<boolean> {
    const encoded = btoa(`${user}:${pass}`);
    try {
      const res = await fetch(`${API_BASE}/api/agents`, {
        headers: { Authorization: `Basic ${encoded}` },
      });
      if (res.ok) {
        setCredentials(encoded);
        localStorage.setItem("polymr_auth", encoded);
        return true;
      }
    } catch {
      // connection error
    }
    return false;
  }

  function logout() {
    setCredentials(null);
    localStorage.removeItem("polymr_auth");
  }

  function authHeaders(): HeadersInit {
    if (!credentials) return {};
    return { Authorization: `Basic ${credentials}` };
  }

  return (
    <AuthContext.Provider value={{ credentials, login, logout, authHeaders }}>
      {children}
    </AuthContext.Provider>
  );
}

export function LoginGate({ children }: { children: ReactNode }) {
  const { credentials, login } = useAuth();
  const [user, setUser] = useState("");
  const [pass, setPass] = useState("");
  const [error, setError] = useState(false);
  const [loading, setLoading] = useState(false);

  if (credentials) return <>{children}</>;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(false);
    const ok = await login(user, pass);
    if (!ok) setError(true);
    setLoading(false);
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-zinc-950 px-4">
      <form
        onSubmit={handleSubmit}
        className="w-full max-w-sm rounded-2xl border border-zinc-800 bg-zinc-900 p-8"
      >
        <h1 className="mb-6 text-center text-2xl font-bold text-white">
          Winston
        </h1>
        {error && (
          <p className="mb-4 text-center text-sm text-red-400">
            Invalid credentials
          </p>
        )}
        <input
          type="text"
          placeholder="Username"
          value={user}
          onChange={(e) => setUser(e.target.value)}
          className="mb-3 w-full rounded-xl border border-zinc-700 bg-zinc-800 px-4 py-3 text-white placeholder-zinc-500 focus:border-zinc-500 focus:outline-none"
          autoComplete="username"
        />
        <input
          type="password"
          placeholder="Password"
          value={pass}
          onChange={(e) => setPass(e.target.value)}
          className="mb-4 w-full rounded-xl border border-zinc-700 bg-zinc-800 px-4 py-3 text-white placeholder-zinc-500 focus:border-zinc-500 focus:outline-none"
          autoComplete="current-password"
        />
        <button
          type="submit"
          disabled={loading || !user || !pass}
          className="w-full rounded-xl bg-blue-600 py-3 font-medium text-white hover:bg-blue-500 disabled:opacity-50"
        >
          {loading ? "Signing in..." : "Sign In"}
        </button>
      </form>
    </div>
  );
}
