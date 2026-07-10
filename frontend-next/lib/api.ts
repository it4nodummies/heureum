// In Docker: NEXT_PUBLIC_API_URL is "" → fetch uses relative URLs → nginx proxies
// Local dev:  set NEXT_PUBLIC_API_URL=http://localhost:8080 in .env.local
const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "";

// ── Types ────────────────────────────────────────────────────────────────────

export type ProjectType = "scrum" | "kanban" | "business";

export interface LeadInfo {
  id: string;
  display_name: string;
  avatar_url: string;
  email: string;
}

export interface Project {
  id: string;
  org_id?: string;
  name: string;
  key: string;
  description: string;
  type: ProjectType;
  lead_user_id?: string;
  default_assignee: string;
  icon_url: string;
  is_archived: boolean;
  created_at: string;
  updated_at: string;
  // enriched fields
  lead?: LeadInfo;
  is_starred: boolean;
}

export interface PagedResponse<T> {
  startAt: number;
  maxResults: number;
  total: number;
  isLast: boolean;
  values: T[];
}

export interface User {
  id: string;
  email: string;
  username: string;
  display_name: string;
  avatar_url: string;
  is_admin: boolean;
  is_active: boolean;
}

export interface ListProjectsParams {
  query?: string;
  type?: string; // comma-separated types
  orderBy?: "name" | "key" | "type" | "created_at";
  direction?: "asc" | "desc";
  startAt?: number;
  maxResults?: number;
}

// ── Core fetch helper ────────────────────────────────────────────────────────

function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("token");
}

async function apiFetch<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = getToken();
  const headers: HeadersInit = {
    "Content-Type": "application/json",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...options.headers,
  };

  const res = await fetch(`${BASE_URL}${path}`, { ...options, headers });

  if (res.status === 401) {
    if (typeof window !== "undefined") {
      localStorage.removeItem("token");
      window.location.href = "/login";
    }
    throw new Error("Unauthorized");
  }

  if (!res.ok) {
    const text = await res.text();
    let msg = `HTTP ${res.status}`;
    try {
      const json = JSON.parse(text);
      // Formato Jira v3: { errorMessages: string[], errors: Record<string,string> }
      if (Array.isArray(json.errorMessages) && json.errorMessages.length > 0) {
        msg = json.errorMessages.join(" ");
      } else if (json.errors && Object.keys(json.errors).length > 0) {
        msg = Object.entries(json.errors)
          .map(([field, err]) => `${field}: ${err}`)
          .join("; ");
      } else if (json.error) {
        msg = json.error; // retrocompatibilità con endpoint non ancora migrati
      }
    } catch {
      /* ignore */
    }
    throw new Error(msg);
  }

  if (res.status === 204) return undefined as unknown as T;
  return res.json() as Promise<T>;
}

function buildQuery(params: Record<string, string | number | undefined>): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== "") q.set(k, String(v));
  }
  return q.toString() ? `?${q.toString()}` : "";
}

// ── Auth ─────────────────────────────────────────────────────────────────────

export const auth = {
  login: (email: string, password: string) =>
    apiFetch<{ token: string }>("/rest/api/3/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),

  register: (email: string, username: string, password: string) =>
    apiFetch<User>("/rest/api/3/auth/register", {
      method: "POST",
      body: JSON.stringify({ email, username, password }),
    }),

  me: () => apiFetch<User>("/rest/api/3/users/me"),
};

// ── Projects ─────────────────────────────────────────────────────────────────

export const projects = {
  list: (params: ListProjectsParams = {}): Promise<PagedResponse<Project>> => {
    const qs = buildQuery({
      query: params.query,
      type: params.type,
      orderBy: params.orderBy,
      direction: params.direction,
      startAt: params.startAt,
      maxResults: params.maxResults,
    });
    return apiFetch<PagedResponse<Project>>(`/rest/api/3/project${qs}`);
  },

  get: (key: string) =>
    apiFetch<Project>(`/rest/api/3/project/${key}`),

  create: (payload: {
    name: string;
    key: string;
    description?: string;
    type: ProjectType;
  }) =>
    apiFetch<Project>("/rest/api/3/project", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  update: (key: string, payload: { name?: string; description?: string }) =>
    apiFetch<Project>(`/rest/api/3/project/${key}`, {
      method: "PUT",
      body: JSON.stringify(payload),
    }),

  archive: (key: string) =>
    apiFetch<void>(`/rest/api/3/project/${key}`, { method: "DELETE" }),

  star: (key: string) =>
    apiFetch<void>(`/rest/api/3/project/${key}/star`, { method: "PUT" }),

  unstar: (key: string) =>
    apiFetch<void>(`/rest/api/3/project/${key}/star`, { method: "DELETE" }),
};
