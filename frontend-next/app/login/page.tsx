"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { auth } from "@/lib/api";
import { saveToken, saveUser } from "@/lib/auth";

export default function LoginPage() {
  const router = useRouter();
  const [tab, setTab] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      if (tab === "login") {
        const { token } = await auth.login(email, password);
        saveToken(token);
        const user = await auth.me();
        saveUser(user);
      } else {
        await auth.register(email, username, password);
        const { token } = await auth.login(email, password);
        saveToken(token);
        const user = await auth.me();
        saveUser(user);
      }
      router.push("/jira/projects");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "An error occurred");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-[#f0f4ff] to-[#e8edf8]">
      <div className="w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center gap-2.5 mb-3">
            <div className="w-9 h-9 rounded-lg bg-[#0052cc] flex items-center justify-center">
              <svg viewBox="0 0 24 24" fill="white" className="w-5 h-5">
                <path d="M11.571 11.513H0a5.218 5.218 0 0 0 5.232 5.215h2.13v2.057A5.215 5.215 0 0 0 12.575 24V12.518a1.005 1.005 0 0 0-1.004-1.005z" />
                <path
                  d="M5.943 6.285H17.51a5.218 5.218 0 0 1-5.232 5.215H10.15V13.557A5.215 5.215 0 0 1 4.938 8.342V7.29a1.005 1.005 0 0 1 1.005-1.005z"
                  opacity=".5"
                />
                <path
                  d="M.312.057H11.88a5.218 5.218 0 0 1-5.232 5.215H4.518v2.057A5.215 5.215 0 0 1-.694 2.114V1.062A1.005 1.005 0 0 1 .312.057z"
                  opacity=".25"
                />
              </svg>
            </div>
            <span className="text-xl font-bold text-[#1a1f36] tracking-tight">
              Open Jira
            </span>
          </div>
          <p className="text-sm text-[#64748b]">Project management, modernized</p>
        </div>

        {/* Card */}
        <div className="bg-white rounded-2xl shadow-lg shadow-slate-200/80 border border-slate-100 overflow-hidden">
          {/* Tabs */}
          <div className="flex border-b border-slate-100">
            <button
              onClick={() => setTab("login")}
              className={`flex-1 py-4 text-sm font-medium transition-colors ${
                tab === "login"
                  ? "text-[#0052cc] border-b-2 border-[#0052cc] -mb-px"
                  : "text-[#64748b] hover:text-[#1a1f36]"
              }`}
            >
              Sign in
            </button>
            <button
              onClick={() => setTab("register")}
              className={`flex-1 py-4 text-sm font-medium transition-colors ${
                tab === "register"
                  ? "text-[#0052cc] border-b-2 border-[#0052cc] -mb-px"
                  : "text-[#64748b] hover:text-[#1a1f36]"
              }`}
            >
              Create account
            </button>
          </div>

          <form onSubmit={handleSubmit} className="p-8 space-y-4">
            {error && (
              <div className="bg-red-50 border border-red-100 text-red-600 text-sm rounded-lg px-4 py-3">
                {error}
              </div>
            )}

            <div>
              <label className="block text-xs font-semibold text-[#64748b] uppercase tracking-wider mb-1.5">
                Email
              </label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                placeholder="you@example.com"
                className="w-full px-3.5 py-2.5 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc] transition-all placeholder:text-slate-300"
              />
            </div>

            {tab === "register" && (
              <div>
                <label className="block text-xs font-semibold text-[#64748b] uppercase tracking-wider mb-1.5">
                  Username
                </label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                  placeholder="johndoe"
                  className="w-full px-3.5 py-2.5 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc] transition-all placeholder:text-slate-300"
                />
              </div>
            )}

            <div>
              <label className="block text-xs font-semibold text-[#64748b] uppercase tracking-wider mb-1.5">
                Password
              </label>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                placeholder="••••••••"
                className="w-full px-3.5 py-2.5 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-[#0052cc]/20 focus:border-[#0052cc] transition-all placeholder:text-slate-300"
              />
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full mt-2 py-2.5 px-4 bg-[#0052cc] hover:bg-[#0065ff] disabled:opacity-60 text-white text-sm font-semibold rounded-lg transition-colors focus:outline-none focus:ring-2 focus:ring-[#0052cc]/30 focus:ring-offset-2"
            >
              {loading
                ? "Please wait…"
                : tab === "login"
                ? "Sign in"
                : "Create account"}
            </button>
          </form>
        </div>
      </div>
    </div>
  );
}
