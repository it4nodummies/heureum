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

  // PUT /rest/api/3/issue/{key} non gestisce lo status (non è un campo "fields"
  // libero come Jira reale: richiede una transizione validata dal workflow).
  // Endpoint reale: POST /rest/api/3/issue/{key}/transitions { status_id }.
  transition: (idOrKey: string, statusId: string) =>
    apiFetch<void>(`/rest/api/3/issue/${idOrKey}/transitions`, {
      method: "POST",
      body: JSON.stringify({ status_id: statusId }),
    }),

  del: (idOrKey: string) => apiFetch<void>(`/rest/api/3/issue/${idOrKey}`, { method: "DELETE" }),

  availableTransitions: (idOrKey: string) =>
    apiFetch<{ transitions: AvailableTransition[] }>(`/rest/api/3/issue/${idOrKey}/transitions`),
};

// ── Metadata (priorities, issue types, statuses) ────────────────────────────

export const meta = {
  priorities: () => apiFetch<PriorityRef[]>("/rest/api/3/priority"),
  issueTypes: () => apiFetch<IssueTypeRef[]>("/rest/api/3/issuetype"),
  statuses: () => apiFetch<StatusRef[]>("/rest/api/3/status"),
};

// ── Comments ─────────────────────────────────────────────────────────────────

export interface Comment {
  self: string;
  id: string;
  author: JiraUserRef | null;
  body: ADFNode | null;
  created: string;
  updated: string;
}
export interface PageOfComments { startAt: number; maxResults: number; total: number; comments: Comment[]; }

export const comments = {
  list: (issueKey: string) => apiFetch<PageOfComments>(`/rest/api/3/issue/${issueKey}/comment`),
  add: (issueKey: string, body: ADFNode) =>
    apiFetch<Comment>(`/rest/api/3/issue/${issueKey}/comment`, { method: "POST", body: JSON.stringify({ body }) }),
  del: (issueKey: string, id: string) =>
    apiFetch<void>(`/rest/api/3/issue/${issueKey}/comment/${id}`, { method: "DELETE" }),
};

// ── Watchers & Votes ─────────────────────────────────────────────────────────

export interface Watchers { self: string; isWatching: boolean; watchCount: number; watchers: JiraUserRef[]; }
export interface Votes { self: string; votes: number; hasVoted: boolean; voters: JiraUserRef[]; }

export const watchers = {
  get: (issueKey: string) => apiFetch<Watchers>(`/rest/api/3/issue/${issueKey}/watchers`),
  watch: (issueKey: string) => apiFetch<void>(`/rest/api/3/issue/${issueKey}/watchers`, { method: "POST" }),
  unwatch: (issueKey: string) => apiFetch<void>(`/rest/api/3/issue/${issueKey}/watchers`, { method: "DELETE" }),
};
export const votes = {
  get: (issueKey: string) => apiFetch<Votes>(`/rest/api/3/issue/${issueKey}/votes`),
  vote: (issueKey: string) => apiFetch<void>(`/rest/api/3/issue/${issueKey}/votes`, { method: "POST" }),
  unvote: (issueKey: string) => apiFetch<void>(`/rest/api/3/issue/${issueKey}/votes`, { method: "DELETE" }),
};

// ── Search & Filters ─────────────────────────────────────────────────────────

export interface SearchIssue {
  id: string;
  key: string;
  self: string;
  fields: {
    summary?: string;
    status?: { name: string; statusCategory?: { key: string; colorName: string } };
    priority?: { name: string };
    assignee?: { displayName: string } | null;
    updated?: string;
  };
}

export interface SearchJqlResponse {
  issues: SearchIssue[];
  nextPageToken?: string;
  isLast: boolean;
}

export interface Filter {
  id: string;
  name: string;
  description?: string;
  jql: string;
  favourite: boolean;
  owner?: { displayName: string };
}

export const search = {
  jql: (jql: string, opts?: { fields?: string[]; nextPageToken?: string; maxResults?: number }) =>
    apiFetch<SearchJqlResponse>("/rest/api/3/search/jql", {
      method: "POST",
      body: JSON.stringify({
        jql,
        fields: opts?.fields ?? ["summary", "status", "priority", "assignee", "updated"],
        nextPageToken: opts?.nextPageToken,
        maxResults: opts?.maxResults ?? 50,
      }),
    }),
};

export const filters = {
  list: () => apiFetch<Filter[]>("/rest/api/3/filter/my"),
  favourites: () => apiFetch<Filter[]>("/rest/api/3/filter/favourite"),
  get: (id: string) => apiFetch<Filter>(`/rest/api/3/filter/${id}`),
  create: (name: string, jql: string, description?: string) =>
    apiFetch<Filter>("/rest/api/3/filter", {
      method: "POST",
      body: JSON.stringify({ name, jql, description }),
    }),
  del: (id: string) => apiFetch<void>(`/rest/api/3/filter/${id}`, { method: "DELETE" }),
  setFavourite: (id: string, fav: boolean) =>
    apiFetch<Filter>(`/rest/api/3/filter/${id}/favourite`, { method: fav ? "PUT" : "DELETE" }),
};

export function textToADF(text: string): ADFNode {
  return { type: "doc", version: 1, content: [{ type: "paragraph", content: [{ type: "text", text }] }] };
}

// ── Agile (Boards & Sprints) ─────────────────────────────────────────────────

export interface AgileBoard {
  id: number;
  name: string;
  type: string;
  location?: { projectKey: string; projectName: string };
}

export interface AgileSprint {
  id: number;
  name: string;
  state: "future" | "active" | "closed";
  goal?: string;
  startDate?: string;
  endDate?: string;
}

// Le liste issue agili usano lo shape SearchResults (issues+total).
export interface AgileIssueList {
  issues: SearchIssue[];
  total: number;
}

export const boards = {
  list: () => apiFetch<{ values: AgileBoard[] }>("/rest/agile/1.0/board"),
  create: (name: string, projectKeyOrId: string, type = "scrum") =>
    apiFetch<AgileBoard>("/rest/agile/1.0/board", {
      method: "POST",
      body: JSON.stringify({ name, projectKeyOrId, type }),
    }),
  get: (boardId: number) => apiFetch<AgileBoard>(`/rest/agile/1.0/board/${boardId}`),
  issues: (boardId: number) => apiFetch<AgileIssueList>(`/rest/agile/1.0/board/${boardId}/issue`),
  backlog: (boardId: number) => apiFetch<AgileIssueList>(`/rest/agile/1.0/board/${boardId}/backlog`),
  sprints: (boardId: number) => apiFetch<{ values: AgileSprint[] }>(`/rest/agile/1.0/board/${boardId}/sprint`),
  configuration: (boardId: number) =>
    apiFetch<{ columnConfig: { columns: { name: string; statuses: { id: string }[] }[] } }>(
      `/rest/agile/1.0/board/${boardId}/configuration`,
    ),
};

export const sprints = {
  create: (name: string, originBoardId: number, goal?: string) =>
    apiFetch<AgileSprint>("/rest/agile/1.0/sprint", {
      method: "POST",
      body: JSON.stringify({ name, originBoardId, goal }),
    }),
  issues: (sprintId: number) => apiFetch<AgileIssueList>(`/rest/agile/1.0/sprint/${sprintId}/issue`),
  setState: (sprintId: number, state: "active" | "closed") =>
    apiFetch<AgileSprint>(`/rest/agile/1.0/sprint/${sprintId}`, {
      method: "POST",
      body: JSON.stringify({ state }),
    }),
  moveIssues: (sprintId: number, issues: string[]) =>
    apiFetch<void>(`/rest/agile/1.0/sprint/${sprintId}/issue`, {
      method: "POST",
      body: JSON.stringify({ issues }),
    }),
};

export const agileIssues = {
  moveToBacklog: (issues: string[]) =>
    apiFetch<void>("/rest/agile/1.0/backlog/issue", { method: "POST", body: JSON.stringify({ issues }) }),
  rank: (issues: string[], rankBeforeIssue?: string, rankAfterIssue?: string) =>
    apiFetch<void>("/rest/agile/1.0/issue/rank", {
      method: "PUT",
      body: JSON.stringify({ issues, rankBeforeIssue, rankAfterIssue }),
    }),
};

// ── Workflow ─────────────────────────────────────────────────────────────────

export interface WorkflowStatus {
  id: string;
  name: string;
  category: "todo" | "inprogress" | "done";
  color: string;
  position: number;
}

export interface WorkflowTransition {
  id: string;
  from_status_id: string;
  to_status_id: string;
  name: string;
  require_assignee: boolean;
  set_resolution: boolean;
}

export interface Workflow {
  id: string;
  name: string;
  statuses: WorkflowStatus[];
  transitions: WorkflowTransition[];
}

export interface AvailableTransition {
  id: string;
  name: string;
  to: { id: string; name: string; statusCategory: { key: string; name: string } };
}

export const workflow = {
  get: (projectKey: string) => apiFetch<Workflow>(`/rest/api/3/project/${projectKey}/workflow`),
  addStatus: (projectKey: string, name: string, category: string, color: string) =>
    apiFetch<WorkflowStatus>(`/rest/api/3/project/${projectKey}/workflow/statuses`, {
      method: "POST",
      body: JSON.stringify({ name, category, color }),
    }),
  updateStatus: (projectKey: string, id: string, patch: { name?: string; category?: string; color?: string }) =>
    apiFetch<WorkflowStatus>(`/rest/api/3/project/${projectKey}/workflow/statuses/${id}`, {
      method: "PATCH",
      body: JSON.stringify(patch),
    }),
  deleteStatus: (projectKey: string, id: string) =>
    apiFetch<void>(`/rest/api/3/project/${projectKey}/workflow/statuses/${id}`, { method: "DELETE" }),
  reorderStatuses: (projectKey: string, statusIds: string[]) =>
    apiFetch<void>(`/rest/api/3/project/${projectKey}/workflow/statuses/order`, {
      method: "PUT",
      body: JSON.stringify({ status_ids: statusIds }),
    }),
  addTransition: (
    projectKey: string,
    t: { from_status_id: string; to_status_id: string; name: string; require_assignee?: boolean; set_resolution?: boolean }
  ) =>
    apiFetch<WorkflowTransition>(`/rest/api/3/project/${projectKey}/workflow/transitions`, {
      method: "POST",
      body: JSON.stringify(t),
    }),
  updateTransition: (
    projectKey: string,
    id: string,
    patch: { name?: string; require_assignee?: boolean; set_resolution?: boolean }
  ) =>
    apiFetch<WorkflowTransition>(`/rest/api/3/project/${projectKey}/workflow/transitions/${id}`, {
      method: "PATCH",
      body: JSON.stringify(patch),
    }),
  deleteTransition: (projectKey: string, id: string) =>
    apiFetch<void>(`/rest/api/3/project/${projectKey}/workflow/transitions/${id}`, { method: "DELETE" }),
};
