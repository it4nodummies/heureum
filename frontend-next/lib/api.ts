// In Docker: NEXT_PUBLIC_API_URL is "" → fetch uses relative URLs → nginx proxies
// Local dev:  set NEXT_PUBLIC_API_URL=http://localhost:8080 in .env.local
const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "";

// ── Types ────────────────────────────────────────────────────────────────────

export type ProjectTypeKey = "software" | "business";

export interface JiraUserRef {
  accountId: string;
  displayName: string;
  emailAddress?: string;
  avatarUrls: Record<string, string>;
}

export interface ProjectCategoryRef {
  self?: string;
  id: string;
  name: string;
  description: string;
}

export interface Project {
  self: string;
  id: string; // numeric string (seq_id), e.g. "10000"
  key: string;
  name: string;
  description?: string;
  projectTypeKey: ProjectTypeKey;
  style: string;
  simplified: boolean;
  isPrivate: boolean;
  archived: boolean;
  assigneeType?: string;
  url?: string;
  avatarUrls: Record<string, string>;
  lead?: JiraUserRef;
  projectCategory?: ProjectCategoryRef;
}

export interface ADFNode {
  type: string;
  version?: number;
  text?: string;
  content?: ADFNode[];
  attrs?: Record<string, unknown>;
  marks?: { type: string; attrs?: Record<string, unknown> }[];
}

export interface IssueTypeRef {
  id: string;
  name: string;
  iconUrl?: string;
  subtask: boolean;
}

export interface StatusRef {
  id: string;
  name: string;
  statusCategory: { id: number; key: string; colorName: string; name: string };
}

export interface PriorityRef {
  id: string;
  name: string;
  iconUrl?: string;
  statusColor?: string;
}

export interface IssueFields {
  summary: string;
  description: ADFNode | null;
  issuetype: IssueTypeRef | null;
  status: StatusRef | null;
  priority: PriorityRef | null;
  assignee: JiraUserRef | null;
  reporter: JiraUserRef | null;
  labels: string[];
  created: string;
  updated: string;
  duedate?: string;
  parent?: { id: string; key: string };
  project?: { id: string; key: string; name: string };
}

export interface Issue {
  self: string;
  id: string;
  key: string;
  fields: IssueFields;
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

// ── Core fetch helper ────────────────────────────────────────────────────────

function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("token");
}

interface ApiFetchOptions extends RequestInit {
  // When true, a 401 response is NOT treated as "session expired" (no
  // localStorage wipe / redirect to /login). Use this for the login and
  // register requests themselves, whose own 401 means "wrong credentials",
  // not "your session died" — the caller needs the parsed error to reach it.
  skipAuthRedirect?: boolean;
}

async function apiFetch<T>(
  path: string,
  options: ApiFetchOptions = {}
): Promise<T> {
  const { skipAuthRedirect, ...requestInit } = options;
  const token = getToken();
  const headers: HeadersInit = {
    "Content-Type": "application/json",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...requestInit.headers,
  };

  const res = await fetch(`${BASE_URL}${path}`, { ...requestInit, headers });

  if (res.status === 401 && !skipAuthRedirect) {
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
      skipAuthRedirect: true,
    }),

  register: (email: string, username: string, password: string) =>
    apiFetch<User>("/rest/api/3/auth/register", {
      method: "POST",
      body: JSON.stringify({ email, username, password }),
      skipAuthRedirect: true,
    }),

  me: () => apiFetch<User>("/rest/api/3/users/me"),
};

// ── Projects ─────────────────────────────────────────────────────────────────

export const projects = {
  search: (params: { query?: string; startAt?: number; maxResults?: number } = {}) => {
    const qs = buildQuery({
      query: params.query,
      startAt: params.startAt,
      maxResults: params.maxResults,
    });
    return apiFetch<PagedResponse<Project>>(`/rest/api/3/project/search${qs}`);
  },

  get: (idOrKey: string) => apiFetch<Project>(`/rest/api/3/project/${idOrKey}`),

  create: (payload: {
    key: string;
    name: string;
    description?: string;
    projectTypeKey: ProjectTypeKey;
    projectTemplateKey: string;
    assigneeType?: string;
  }) =>
    apiFetch<{ self: string; id: number; key: string }>("/rest/api/3/project", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  update: (
    idOrKey: string,
    payload: { name?: string; description?: string; assigneeType?: string; url?: string }
  ) =>
    apiFetch<Project>(`/rest/api/3/project/${idOrKey}`, {
      method: "PUT",
      body: JSON.stringify(payload),
    }),

  archive: (idOrKey: string) =>
    apiFetch<void>(`/rest/api/3/project/${idOrKey}/archive`, { method: "POST" }),

  restore: (idOrKey: string) =>
    apiFetch<Project>(`/rest/api/3/project/${idOrKey}/restore`, { method: "POST" }),

  types: () => apiFetch<{ key: string; formattedKey: string }[]>("/rest/api/3/project/type"),

  categories: () => apiFetch<ProjectCategoryRef[]>("/rest/api/3/projectCategory"),
};

// ── Issues ───────────────────────────────────────────────────────────────────

export const issues = {
  get: (idOrKey: string) => apiFetch<Issue>(`/rest/api/3/issue/${idOrKey}`),

  create: (payload: {
    projectKey: string;
    summary: string;
    issueTypeName?: string;
    priorityId?: string;
    description?: ADFNode;
    labels?: string[];
  }) =>
    apiFetch<{ id: string; key: string; self: string }>("/rest/api/3/issue", {
      method: "POST",
      body: JSON.stringify({
        fields: {
          project: { key: payload.projectKey },
          summary: payload.summary,
          issuetype: { name: payload.issueTypeName ?? "Task" },
          ...(payload.priorityId ? { priority: { id: payload.priorityId } } : {}),
          ...(payload.description ? { description: payload.description } : {}),
          ...(payload.labels ? { labels: payload.labels } : {}),
        },
      }),
    }),

  update: (idOrKey: string, fields: Record<string, unknown>) =>
    apiFetch<void>(`/rest/api/3/issue/${idOrKey}`, {
      method: "PUT",
      body: JSON.stringify({ fields }),
    }),

  del: (idOrKey: string) => apiFetch<void>(`/rest/api/3/issue/${idOrKey}`, { method: "DELETE" }),
};

// ── Metadata (priorities, issue types, statuses) ────────────────────────────

export const meta = {
  priorities: () => apiFetch<PriorityRef[]>("/rest/api/3/priority"),
  issueTypes: () => apiFetch<IssueTypeRef[]>("/rest/api/3/issuetype"),
  statuses: () => apiFetch<StatusRef[]>("/rest/api/3/status"),
};
