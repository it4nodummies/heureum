"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";

interface NavItem {
  label: string;
  href?: string;
  icon: React.ReactNode;
  badge?: string;
  external?: boolean;
  comingSoon?: boolean;
  children?: { label: string; href: string; icon: React.ReactNode }[];
}

function JiraLogo() {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" className="w-5 h-5 text-white">
      <path d="M11.571 11.513H0a5.218 5.218 0 0 0 5.232 5.215h2.13v2.057A5.215 5.215 0 0 0 12.575 24V12.518a1.005 1.005 0 0 0-1.004-1.005z" />
      <path
        d="M5.943 6.285H17.51a5.218 5.218 0 0 1-5.232 5.215H10.15V13.557A5.215 5.215 0 0 1 4.938 8.342V7.29a1.005 1.005 0 0 1 1.005-1.005z"
        opacity=".65"
      />
      <path
        d="M.312.057H11.88a5.218 5.218 0 0 1-5.232 5.215H4.518v2.057A5.215 5.215 0 0 1-.694 2.114V1.062A1.005 1.005 0 0 1 .312.057z"
        opacity=".3"
      />
    </svg>
  );
}

const NAV_ITEMS: NavItem[] = [
  {
    label: "For you",
    href: "/app",
    icon: (
      <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
        <path
          fillRule="evenodd"
          d="M10 9a3 3 0 100-6 3 3 0 000 6zm-7 9a7 7 0 1114 0H3z"
          clipRule="evenodd"
        />
      </svg>
    ),
  },
  {
    label: "Recent",
    icon: (
      <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
        <path
          fillRule="evenodd"
          d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z"
          clipRule="evenodd"
        />
      </svg>
    ),
  },
  {
    label: "Starred",
    icon: (
      <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
        <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z" />
      </svg>
    ),
  },
  {
    label: "Apps",
    comingSoon: true,
    icon: (
      <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
        <path d="M5 3a2 2 0 00-2 2v2a2 2 0 002 2h2a2 2 0 002-2V5a2 2 0 00-2-2H5zM5 11a2 2 0 00-2 2v2a2 2 0 002 2h2a2 2 0 002-2v-2a2 2 0 00-2-2H5zM11 5a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V5zM11 13a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" />
      </svg>
    ),
  },
  {
    label: "Plans",
    comingSoon: true,
    icon: (
      <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
        <path
          fillRule="evenodd"
          d="M3 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1z"
          clipRule="evenodd"
        />
      </svg>
    ),
  },
  {
    label: "Projects",
    href: "/app/projects",
    icon: (
      <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
        <path d="M7 3a1 1 0 000 2h6a1 1 0 100-2H7zM4 7a1 1 0 011-1h10a1 1 0 110 2H5a1 1 0 01-1-1zM2 11a2 2 0 012-2h12a2 2 0 012 2v4a2 2 0 01-2 2H4a2 2 0 01-2-2v-4z" />
      </svg>
    ),
  },
  {
    label: "Filters",
    href: "/app/filters",
    icon: (
      <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
        <path
          fillRule="evenodd"
          d="M3 3a1 1 0 011-1h12a1 1 0 011 1v3a1 1 0 01-.293.707L12 11.414V15a1 1 0 01-.293.707l-2 2A1 1 0 018 17v-5.586L3.293 6.707A1 1 0 013 6V3z"
          clipRule="evenodd"
        />
      </svg>
    ),
  },
  {
    label: "Dashboards",
    href: "/app/dashboards",
    icon: (
      <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
        <path d="M2 11a1 1 0 011-1h2a1 1 0 011 1v5a1 1 0 01-1 1H3a1 1 0 01-1-1v-5zM8 7a1 1 0 011-1h2a1 1 0 011 1v9a1 1 0 01-1 1H9a1 1 0 01-1-1V7zM14 4a1 1 0 011-1h2a1 1 0 011 1v12a1 1 0 01-1 1h-2a1 1 0 01-1-1V4z" />
      </svg>
    ),
  },
];

const BOTTOM_ITEMS = [
  {
    label: "Goals",
    comingSoon: true,
    icon: (
      <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
        <path
          fillRule="evenodd"
          d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z"
          clipRule="evenodd"
        />
      </svg>
    ),
  },
  {
    label: "Teams",
    comingSoon: true,
    icon: (
      <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
        <path d="M13 6a3 3 0 11-6 0 3 3 0 016 0zM18 8a2 2 0 11-4 0 2 2 0 014 0zM14 15a4 4 0 00-8 0v3h8v-3zM6 8a2 2 0 11-4 0 2 2 0 014 0zM16 18v-3a5.972 5.972 0 00-.75-2.906A3.005 3.005 0 0119 15v3h-3zM4.75 12.094A5.973 5.973 0 004 15v3H1v-3a3 3 0 013.75-2.906z" />
      </svg>
    ),
  },
];

export default function Sidebar() {
  const pathname = usePathname();
  const [collapsed, setCollapsed] = useState(false);

  return (
    <aside
      className={`flex flex-col bg-white border-r border-slate-100 transition-all duration-200 shrink-0 ${
        collapsed ? "w-14" : "w-[240px]"
      }`}
      style={{ height: "calc(100vh - 52px)" }}
    >
      {/* Logo + collapse button */}
      <div className="flex items-center justify-between px-3 py-3 border-b border-slate-100">
        <div
          className={`flex items-center gap-2.5 overflow-hidden ${
            collapsed ? "w-0" : "w-auto"
          }`}
        >
          <div className="w-7 h-7 rounded-md bg-[#0052cc] flex items-center justify-center shrink-0">
            <JiraLogo />
          </div>
          {!collapsed && (
            <span className="font-bold text-sm text-[#1a1f36] whitespace-nowrap">
              Heureum
            </span>
          )}
        </div>
        {collapsed && (
          <div className="w-7 h-7 rounded-md bg-[#0052cc] flex items-center justify-center shrink-0 mx-auto">
            <JiraLogo />
          </div>
        )}
        {!collapsed && (
          <button
            onClick={() => setCollapsed(true)}
            className="p-1.5 rounded-md text-slate-400 hover:text-slate-600 hover:bg-slate-100 transition-colors"
            title="Collapse sidebar"
          >
            <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
              <path
                fillRule="evenodd"
                d="M12.707 5.293a1 1 0 010 1.414L9.414 10l3.293 3.293a1 1 0 01-1.414 1.414l-4-4a1 1 0 010-1.414l4-4a1 1 0 011.414 0z"
                clipRule="evenodd"
              />
            </svg>
          </button>
        )}
        {collapsed && (
          <button
            onClick={() => setCollapsed(false)}
            className="absolute left-11 p-1.5 rounded-md bg-white border border-slate-200 text-slate-400 hover:text-slate-600 hover:bg-slate-50 transition-colors shadow-sm"
            title="Expand sidebar"
          >
            <svg viewBox="0 0 20 20" fill="currentColor" className="w-3.5 h-3.5">
              <path
                fillRule="evenodd"
                d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z"
                clipRule="evenodd"
              />
            </svg>
          </button>
        )}
      </div>

      {/* Nav items */}
      <nav className="flex-1 overflow-y-auto py-2 px-2 space-y-0.5">
        {NAV_ITEMS.map((item) => {
          if (item.comingSoon) {
            return (
              <span
                key={item.label}
                aria-disabled="true"
                title={collapsed ? `${item.label} (Coming soon)` : "Coming soon"}
                className={`flex items-center gap-2.5 w-full px-2.5 py-2 rounded-lg text-sm font-medium text-[#42526e] opacity-40 cursor-not-allowed select-none ${
                  collapsed ? "justify-center" : ""
                }`}
              >
                <span className="shrink-0 text-slate-400">{item.icon}</span>
                {!collapsed && (
                  <span className="truncate flex items-center gap-1.5">
                    {item.label}
                    <span className="px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wide bg-slate-100 text-slate-400">
                      Soon
                    </span>
                  </span>
                )}
              </span>
            );
          }
          const isActive = item.href ? pathname === item.href || pathname.startsWith(item.href + "/") : false;
          const Wrapper = item.href ? Link : "button";
          return (
            <Wrapper
              key={item.label}
              href={item.href ?? "#"}
              className={`flex items-center gap-2.5 w-full px-2.5 py-2 rounded-lg text-sm font-medium transition-colors group ${
                isActive
                  ? "bg-[#e8f0fe] text-[#0052cc]"
                  : "text-[#42526e] hover:bg-slate-100 hover:text-[#1a1f36]"
              } ${collapsed ? "justify-center" : ""}`}
              title={collapsed ? item.label : undefined}
            >
              <span className={`shrink-0 ${isActive ? "text-[#0052cc]" : "text-slate-400 group-hover:text-slate-500"}`}>
                {item.icon}
              </span>
              {!collapsed && <span className="truncate">{item.label}</span>}
            </Wrapper>
          );
        })}
      </nav>

      {/* Bottom items */}
      <div className="border-t border-slate-100 py-2 px-2 space-y-0.5">
        {BOTTOM_ITEMS.map((item) => (
          <span
            key={item.label}
            aria-disabled="true"
            title={collapsed ? `${item.label} (Coming soon)` : "Coming soon"}
            className={`flex items-center gap-2.5 w-full px-2.5 py-2 rounded-lg text-sm font-medium text-[#42526e] opacity-40 cursor-not-allowed select-none ${
              collapsed ? "justify-center" : ""
            }`}
          >
            <span className="shrink-0 text-slate-400">{item.icon}</span>
            {!collapsed && (
              <span className="truncate flex items-center gap-1.5">
                {item.label}
                <span className="px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wide bg-slate-100 text-slate-400">
                  Soon
                </span>
              </span>
            )}
          </span>
        ))}

        {!collapsed && (
          <button className="flex items-center gap-2 w-full px-2.5 py-2 rounded-lg text-xs text-slate-400 hover:text-slate-500 hover:bg-slate-50 transition-colors">
            <svg viewBox="0 0 20 20" fill="currentColor" className="w-3.5 h-3.5">
              <path
                fillRule="evenodd"
                d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z"
                clipRule="evenodd"
              />
            </svg>
            Customize sidebar
          </button>
        )}
      </div>
    </aside>
  );
}
