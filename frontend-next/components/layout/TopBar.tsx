"use client";

import { useState, useEffect, useRef } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { getStoredUser, clearToken } from "@/lib/auth";
import type { User } from "@/lib/api";
import CreateProjectModal from "@/components/projects/CreateProjectModal";
import { CreateIssueModal } from "@/components/issues/CreateIssueModal";
import { GlobalSearch } from "@/components/search/GlobalSearch";
import { NotificationBell } from "@/components/notifications/NotificationBell";
import { ThemeToggle } from "@/components/layout/ThemeProvider";

export default function TopBar() {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [userMenuOpen, setUserMenuOpen] = useState(false);
  const [createMenuOpen, setCreateMenuOpen] = useState(false);
  const [createProjectOpen, setCreateProjectOpen] = useState(false);
  const [createIssueOpen, setCreateIssueOpen] = useState(false);
  const userMenuRef = useRef<HTMLDivElement>(null);
  const createMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setUser(getStoredUser());
  }, []);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (userMenuRef.current && !userMenuRef.current.contains(e.target as Node)) {
        setUserMenuOpen(false);
      }
      if (createMenuRef.current && !createMenuRef.current.contains(e.target as Node)) {
        setCreateMenuOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  function handleLogout() {
    clearToken();
    router.push("/login");
  }

  function getInitials(u: User) {
    if (u.display_name) {
      return u.display_name
        .split(" ")
        .map((w) => w[0])
        .join("")
        .toUpperCase()
        .slice(0, 2);
    }
    return u.username.slice(0, 2).toUpperCase();
  }

  return (
    <>
      <header className="h-[52px] bg-white dark:bg-[#161b22] border-b border-slate-100 dark:border-[#2a3142] flex items-center px-4 gap-4 shrink-0 z-30">
        {/* Search */}
        <div className="flex-1 max-w-lg">
          <GlobalSearch />
        </div>

        {/* Right actions */}
        <div className="flex items-center gap-2">
          {/* Create button */}
          <div className="relative" ref={createMenuRef}>
            <button
              onClick={() => setCreateMenuOpen((v) => !v)}
              className="flex items-center gap-1.5 px-3.5 py-1.5 bg-[#0052cc] hover:bg-[#0065ff] text-white text-sm font-semibold rounded-lg transition-colors"
            >
              <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
                <path
                  fillRule="evenodd"
                  d="M10 5a1 1 0 011 1v3h3a1 1 0 110 2h-3v3a1 1 0 11-2 0v-3H6a1 1 0 110-2h3V6a1 1 0 011-1z"
                  clipRule="evenodd"
                />
              </svg>
              Create
            </button>

            {createMenuOpen && (
              <div className="absolute left-0 top-10 w-44 bg-white dark:bg-[#1a1f2e] rounded-xl shadow-lg shadow-slate-200/80 dark:shadow-black/40 border border-slate-100 dark:border-[#2a3142] py-1.5 z-50">
                <button
                  onClick={() => {
                    setCreateMenuOpen(false);
                    setCreateIssueOpen(true);
                  }}
                  className="flex items-center gap-2.5 w-full px-4 py-2 text-sm text-[#42526e] dark:text-[#c3c9d3] hover:bg-slate-50 dark:hover:bg-[#232a3a] transition-colors"
                >
                  Issue
                </button>
                <button
                  onClick={() => {
                    setCreateMenuOpen(false);
                    setCreateProjectOpen(true);
                  }}
                  className="flex items-center gap-2.5 w-full px-4 py-2 text-sm text-[#42526e] dark:text-[#c3c9d3] hover:bg-slate-50 dark:hover:bg-[#232a3a] transition-colors"
                >
                  Project
                </button>
              </div>
            )}
          </div>

          {/* Notification bell */}
          <NotificationBell />

          {/* Help */}
          <button className="p-1.5 rounded-lg text-slate-400 hover:text-slate-600 hover:bg-slate-100 transition-colors">
            <svg viewBox="0 0 20 20" fill="currentColor" className="w-5 h-5">
              <path
                fillRule="evenodd"
                d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-8-3a1 1 0 00-.867.5 1 1 0 11-1.731-1A3 3 0 0113 8a3.001 3.001 0 01-2 2.83V11a1 1 0 11-2 0v-1a1 1 0 011-1 1 1 0 100-2zm0 8a1 1 0 100-2 1 1 0 000 2z"
                clipRule="evenodd"
              />
            </svg>
          </button>

          {/* Settings */}
          <button className="p-1.5 rounded-lg text-slate-400 hover:text-slate-600 hover:bg-slate-100 transition-colors">
            <svg viewBox="0 0 20 20" fill="currentColor" className="w-5 h-5">
              <path
                fillRule="evenodd"
                d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z"
                clipRule="evenodd"
              />
            </svg>
          </button>

          {/* User avatar */}
          <div className="relative" ref={userMenuRef}>
            <button
              onClick={() => setUserMenuOpen((v) => !v)}
              className="w-8 h-8 rounded-full bg-gradient-to-br from-[#0052cc] to-[#0065ff] flex items-center justify-center text-white text-xs font-bold hover:ring-2 hover:ring-[#0052cc]/30 transition-all"
            >
              {user ? (
                user.avatar_url ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img
                    src={user.avatar_url}
                    alt={user.display_name}
                    className="w-full h-full rounded-full object-cover"
                  />
                ) : (
                  getInitials(user)
                )
              ) : (
                "?"
              )}
            </button>

            {userMenuOpen && (
              <div className="absolute right-0 top-10 w-56 bg-white dark:bg-[#1a1f2e] rounded-xl shadow-lg shadow-slate-200/80 dark:shadow-black/40 border border-slate-100 dark:border-[#2a3142] py-1.5 z-50">
                {user && (
                  <div className="px-4 py-2.5 border-b border-slate-100 dark:border-[#2a3142] mb-1">
                    <p className="text-sm font-semibold text-[#1a1f36] dark:text-[#e6e8eb] truncate">
                      {user.display_name || user.username}
                    </p>
                    <p className="text-xs text-slate-400 truncate">{user.email}</p>
                  </div>
                )}
                <div className="px-4 py-1.5">
                  <ThemeToggle className="flex w-full items-center gap-2.5 rounded-lg px-0 py-1 text-sm text-[#42526e] dark:text-[#c3c9d3]" />
                </div>
                <button className="flex items-center gap-2.5 w-full px-4 py-2 text-sm text-[#42526e] dark:text-[#c3c9d3] hover:bg-slate-50 dark:hover:bg-[#232a3a] transition-colors">
                  <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4 text-slate-400">
                    <path fillRule="evenodd" d="M10 9a3 3 0 100-6 3 3 0 000 6zm-7 9a7 7 0 1114 0H3z" clipRule="evenodd" />
                  </svg>
                  Profile
                </button>
                <button className="flex items-center gap-2.5 w-full px-4 py-2 text-sm text-[#42526e] dark:text-[#c3c9d3] hover:bg-slate-50 dark:hover:bg-[#232a3a] transition-colors">
                  <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4 text-slate-400">
                    <path fillRule="evenodd" d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z" clipRule="evenodd" />
                  </svg>
                  Settings
                </button>
                <div className="border-t border-slate-100 dark:border-[#2a3142] mt-1 pt-1">
                  <button
                    onClick={handleLogout}
                    className="flex items-center gap-2.5 w-full px-4 py-2 text-sm text-red-500 hover:bg-red-50 dark:hover:bg-red-950/40 transition-colors"
                  >
                    <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
                      <path fillRule="evenodd" d="M3 3a1 1 0 00-1 1v12a1 1 0 102 0V4a1 1 0 00-1-1zm10.293 9.293a1 1 0 001.414 1.414l3-3a1 1 0 000-1.414l-3-3a1 1 0 10-1.414 1.414L14.586 9H7a1 1 0 100 2h7.586l-1.293 1.293z" clipRule="evenodd" />
                    </svg>
                    Sign out
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      </header>

      {createProjectOpen && (
        <CreateProjectModal
          onClose={() => setCreateProjectOpen(false)}
          onCreated={() => {
            setCreateProjectOpen(false);
            router.refresh();
          }}
        />
      )}

      {createIssueOpen && (
        <CreateIssueModal
          onClose={() => setCreateIssueOpen(false)}
          onCreated={(key) => {
            setCreateIssueOpen(false);
            router.push(`/app/browse/${key}`);
          }}
        />
      )}
    </>
  );
}
