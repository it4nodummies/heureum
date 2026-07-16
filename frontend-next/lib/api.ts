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
  customfield_10016?: number | null;
  // Absent (not null) when neither estimate nor logged time has ever been
  // set — mirrors the backend's omitempty on v3.TimeTracking (internal/api/v3/issue.go).
  timetracking?: {
    originalEstimateSeconds?: number;
    timeSpentSeconds?: number;
    remainingEstimateSeconds?: number;
  };
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
  // Some 201 responses (e.g. POST /rest/api/3/issueLink) are body-less —
  // guard against `res.json()` throwing "Unexpected end of JSON input" on an
  // empty string rather than assuming every non-204 response has a body.
  const text = await res.text();
  if (!text) return undefined as unknown as T;
  return JSON.parse(text) as T;
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

  // CSV export must go through fetch + bearer header (auth is header-based, so a
  // plain <a href> would 401). Mirrors attachments.contentBlobUrl: fetch bytes,
  // wrap in an object URL, click a synthetic anchor, then revoke.
  exportCsv: async (key: string): Promise<void> => {
    const res = await fetch(`${BASE_URL}/rest/api/3/project/${key}/issues/export`, {
      headers: authHeaders(),
    });
    handleUnauthorized(res);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${key}-issues.csv`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  },

  create: (payload: {
    projectKey: string;
    summary: string;
    issueTypeName?: string;
    priorityId?: string;
    description?: ADFNode;
    labels?: string[];
    parentKey?: string;
    assigneeId?: string;
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
          ...(payload.parentKey ? { parent: { key: payload.parentKey } } : {}),
          ...(payload.assigneeId ? { assignee: { accountId: payload.assigneeId } } : {}),
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

  subtasks: (idOrKey: string) =>
    apiFetch<{ values: Issue[]; total: number }>(`/rest/api/3/issue/${idOrKey}/subtasks`),

  changelog: (idOrKey: string) => apiFetch<PageOfChangelogs>(`/rest/api/3/issue/${idOrKey}/changelog`),
};

// ── Changelog (issue history) ───────────────────────────────────────────────
// v3.Changelog / v3.ChangeItem (internal/api/v3/collab.go). `author` is
// usually absent: the backend logs history entries with ActorID="" (see
// HistoryHandler.GetHistory), so the UI falls back to "System".

export interface ChangeItem {
  field: string;
  fieldId?: string;
  fieldtype: string;
  from?: string;
  fromString?: string;
  to?: string;
  toString?: string;
}

export interface Changelog {
  id: string;
  author?: JiraUserRef | null;
  created: string;
  items: ChangeItem[];
}

export interface PageOfChangelogs {
  startAt: number;
  maxResults: number;
  total: number;
  isLast: boolean;
  values: Changelog[];
}

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

// ── Worklogs (time tracking) ────────────────────────────────────────────────

// Mirrors parseJiraDuration in internal/api/handlers/worklog_handler.go exactly:
// w(eek)=5*8h, d(ay)=8h, h(our)=3600s, m(inute)=60s, space-separated tokens,
// a token with no recognized unit suffix is treated as minutes.
export function parseJiraDuration(s: string): number {
  let total = 0;
  for (const tok of s.trim().split(/\s+/).filter(Boolean)) {
    const lastChar = tok.slice(-1);
    let unit = lastChar;
    let numStr = tok;
    if (unit === "w" || unit === "d" || unit === "h" || unit === "m") {
      numStr = tok.slice(0, -1);
    } else {
      unit = "m";
    }
    const n = parseInt(numStr, 10);
    if (Number.isNaN(n)) continue;
    switch (unit) {
      case "w":
        total += n * 5 * 8 * 3600;
        break;
      case "d":
        total += n * 8 * 3600;
        break;
      case "h":
        total += n * 3600;
        break;
      case "m":
        total += n * 60;
        break;
    }
  }
  return total;
}

// Mirrors formatSeconds in internal/api/v3/worklog.go exactly (e.g. "1h 30m",
// "2h", "45m"; zero or negative → "0m").
export function formatSeconds(sec: number): string {
  if (sec <= 0) return "0m";
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  if (h > 0 && m > 0) return `${h}h ${m}m`;
  if (h > 0) return `${h}h`;
  return `${m}m`;
}

// v3.Worklog (internal/api/v3/worklog.go).
export interface Worklog {
  self: string;
  id: string;
  issueId: string;
  author: JiraUserRef | null;
  updateAuthor?: JiraUserRef | null;
  comment: ADFNode | null;
  created: string;
  updated: string;
  started: string;
  timeSpent: string;
  timeSpentSeconds: number;
}

export interface PageOfWorklogs {
  startAt: number;
  maxResults: number;
  total: number;
  worklogs: Worklog[];
}

export const worklogs = {
  list: (issueKey: string) => apiFetch<PageOfWorklogs>(`/rest/api/3/issue/${issueKey}/worklog`),
  // Sends timeSpentSeconds (parsed client-side via parseJiraDuration) rather
  // than the raw timeSpent string, to avoid depending on the server's own
  // duration parsing for a value we already parsed to validate the form.
  add: (issueKey: string, payload: { timeSpentSeconds: number; comment?: ADFNode }) =>
    apiFetch<Worklog>(`/rest/api/3/issue/${issueKey}/worklog`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  delete: (issueKey: string, id: string) =>
    apiFetch<void>(`/rest/api/3/issue/${issueKey}/worklog/${id}`, { method: "DELETE" }),
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

// ── Reports ──────────────────────────────────────────────────────────────────

export interface BurndownData { labels: string[]; ideal: number[]; actual: number[] }

export interface SprintVelocity {
  sprint_id: string;
  sprint_name: string;
  completed: number;
  total_planned: number;
}
export interface VelocityData { sprints: SprintVelocity[] }

export interface CFDData { categories: string[]; dates: string[]; data: Record<string, number[]> }

export interface PieSlice { label: string; count: number }

export interface CreatedVsResolvedData { dates: string[]; created: number[]; resolved: number[] }

// Sottoinsieme di sprint.Sprint (internal/domain/sprint/model.go) usato dal Summary.
export interface ReportActiveSprint {
  id: string;
  name: string;
  state: "future" | "active" | "closed";
  goal?: string;
  start_date?: string;
  end_date?: string;
}

export interface ProjectSummary {
  issue_count_by_status: Record<string, number>;
  created_last_7_days: number;
  updated_last_7_days: number;
  completed_last_7_days: number;
  active_sprint?: ReportActiveSprint | null;
}

export const reports = {
  burndown: (key: string, sprintId: string) =>
    apiFetch<BurndownData>(`/rest/api/3/project/${key}/reports/burndown?sprintId=${sprintId}`),
  burnup: (key: string, sprintId: string) =>
    apiFetch<BurndownData>(`/rest/api/3/project/${key}/reports/burnup?sprintId=${sprintId}`),
  velocity: (key: string) => apiFetch<VelocityData>(`/rest/api/3/project/${key}/reports/velocity`),
  cfd: (key: string) => apiFetch<CFDData>(`/rest/api/3/project/${key}/reports/cfd`),
  pie: (key: string, field: string) => apiFetch<PieSlice[]>(`/rest/api/3/project/${key}/reports/pie?field=${field}`),
  createdVsResolved: (key: string, days = 30) =>
    apiFetch<CreatedVsResolvedData>(`/rest/api/3/project/${key}/reports/created-vs-resolved?days=${days}`),
  summary: (key: string) => apiFetch<ProjectSummary>(`/rest/api/3/project/${key}/summary`),
};

// ── Timeline (Gantt) ─────────────────────────────────────────────────────────

export interface TimelineBar {
  id: string;
  name: string;
  type: string; // "epic" | "sprint"
  start_date: string | null;
  end_date: string | null;
  progress: number; // 0..100
  parent_id?: string;
  color: string; // hex
}
export interface TimelineData {
  project_id: string;
  zoom: string;
  start_date: string;
  end_date: string;
  bars: TimelineBar[];
  headers: string[];
}
export const timeline = {
  get: (key: string, zoom: "weeks" | "months" | "quarters" = "weeks") =>
    apiFetch<TimelineData>(`/rest/api/3/project/${key}/timeline?zoom=${zoom}`),
};

// ── Calendar ─────────────────────────────────────────────────────────────────

export interface CalendarIssue {
  id: string;
  key: string;
  title: string;
  priority: string;
  status: string;
  due_date: string | null;
  start_date: string | null;
}
export interface CalendarDay {
  date: string; // "YYYY-MM-DD"
  day: number;
  issues: CalendarIssue[];
}
export interface CalendarData {
  year: number;
  month: number;
  days: CalendarDay[];
  total_days: number;
}
export const calendar = {
  get: (key: string, year: number, month: number) =>
    apiFetch<CalendarData>(`/rest/api/3/project/${key}/calendar?year=${year}&month=${month}`),
};

// ── Dashboards ───────────────────────────────────────────────────────────────
//
// Il backend espone due famiglie di rotte parallele per le dashboard
// (`/rest/api/3/dashboards` custom e `/rest/api/3/dashboard` v3-shaped, vedi
// internal/api/router.go); usiamo quella plurale perché List/Get/Create
// rispondono con lo shape "piatto" (array/oggetto nudo, non wrappato in
// {values,total} come /dashboard/search).

export interface Dashboard {
  id: string;
  name: string;
  owner_id: string;
  is_public: boolean;
  layout_json: string;
  created_at: string;
}

export interface DashboardWidget {
  id: string;
  dashboard_id: string;
  widget_type: string;
  config_json: string;
  position_json: string;
  data?: unknown;
}

// Computed `data` shape for the "assigned_to_me" widget type.
export interface AssignedIssue {
  id: string;
  key: string;
  title: string;
  priority: string;
  project_id: string;
  project_name: string;
  updated_at: string;
  status_name: string;
}

// Computed `data` shape for the "activity_stream" widget type.
export interface ActivityItem {
  id: string;
  issue_id: string;
  issue_key: string;
  issue_title: string;
  actor_id?: string;
  actor_name: string;
  field_name: string;
  old_value: string;
  new_value: string;
  created_at: string;
}

export const dashboards = {
  list: () => apiFetch<Dashboard[]>("/rest/api/3/dashboards"),
  get: (id: string) => apiFetch<Dashboard & { widgets: DashboardWidget[] }>(`/rest/api/3/dashboards/${id}`),
  create: (name: string) =>
    apiFetch<Dashboard>("/rest/api/3/dashboards", { method: "POST", body: JSON.stringify({ name }) }),
};

// ── Notifications ────────────────────────────────────────────────────────────

export interface AppNotification {
  id: string;
  user_id: string;
  type: string;
  title: string;
  body: string;
  link: string;
  is_read: boolean;
  created_at: number;
}

export interface NotificationSetting {
  user_id: string;
  project_id: string;
  event_type: string;
  via_email: boolean;
  via_app: boolean;
}

export const notifications = {
  list: () => apiFetch<AppNotification[]>("/rest/api/3/notifications"),
  unreadCount: () => apiFetch<{ count: number }>("/rest/api/3/notifications/unread-count"),
  markRead: (id: string) => apiFetch<void>(`/rest/api/3/notifications/${id}/read`, { method: "PATCH" }),
  markAllRead: () => apiFetch<void>("/rest/api/3/notifications/read-all", { method: "PATCH" }),
  settings: () => apiFetch<NotificationSetting[]>("/rest/api/3/notifications/settings"),
  // Handler decodes {project_id, event_type(required), via_email, via_app}; user_id
  // comes from the auth context, not the body.
  updateSettings: (s: { project_id?: string; event_type: string; via_email: boolean; via_app: boolean }) =>
    apiFetch<{ status: string }>("/rest/api/3/notifications/settings", { method: "PATCH", body: JSON.stringify(s) }),
};

// ── Profile ──────────────────────────────────────────────────────────────────

// Shape of v3.User (internal/api/v3/user.go), returned by /myself, /user/search, etc.
export interface JiraUser {
  self: string;
  accountId: string;
  accountType: string;
  emailAddress?: string;
  displayName: string;
  active: boolean;
  timeZone?: string;
  locale?: string;
  avatarUrls: Record<string, string>;
}

export const profile = {
  me: () => apiFetch<JiraUser>("/rest/api/3/myself"),
  update: (patch: { displayName?: string; timeZone?: string; locale?: string; avatarUrl?: string }) =>
    apiFetch<JiraUser>("/rest/api/3/myself", { method: "PUT", body: JSON.stringify(patch) }),
  searchUsers: (query: string) => apiFetch<JiraUser[]>(`/rest/api/3/user/search?query=${encodeURIComponent(query)}`),
};

// ── Users (assignable search, for the UserPicker) ───────────────────────────

export const users = {
  // GET /rest/api/3/user/assignable/search?project={KEY}&query={q} —
  // membership-scoped (the caller must be a member of the project /
  // BROWSE_PROJECTS, see internal/api/handlers/user_handler.go
  // AssignableSearch); an empty query returns all project members ordered by
  // displayName rather than an empty list.
  assignableSearch: (projectKey: string, query = "") =>
    apiFetch<JiraUserRef[]>(`/rest/api/3/user/assignable/search${buildQuery({ project: projectKey, query })}`),
};

// ── Permissions ──────────────────────────────────────────────────────────────

export interface UserPermission {
  id: string;
  key: string;
  name: string;
  description: string;
  type: string;
  // Handler tags this `omitempty`: absent (not just false) when the caller
  // lacks the permission.
  havePermission?: boolean;
}

export const permissions = {
  mine: (projectKey?: string) =>
    apiFetch<{ permissions: Record<string, UserPermission> }>(
      `/rest/api/3/mypermissions${projectKey ? `?projectKey=${encodeURIComponent(projectKey)}` : ""}`
    ),
};

// ── Groups ───────────────────────────────────────────────────────────────────

export interface GroupRef {
  name: string;
  groupId: string;
  self: string;
}

export const groups = {
  picker: (query: string) =>
    apiFetch<{ header: string; total: number; groups: { name: string; groupId: string }[] }>(
      `/rest/api/3/groups/picker?query=${encodeURIComponent(query)}`
    ),
  create: (name: string) =>
    apiFetch<GroupRef>("/rest/api/3/group", { method: "POST", body: JSON.stringify({ name }) }),
};

// ── Integrations ─────────────────────────────────────────────────────────────

// webhookOut (internal/api/handlers/webhook_handler.go) — secret is never returned.
export interface Webhook {
  id: string;
  project_id: string;
  url: string;
  events: string[];
  is_active: boolean;
}

// git.GitProviderConfig (internal/domain/git/config.go). Note the server
// returns token_encrypted (not a plain "token") and webhook_secret as-is.
export interface GitProviderConfig {
  id: string;
  project_id: string;
  provider_type: string;
  base_url: string;
  token_encrypted: string;
  webhook_secret: string;
  created_at: string;
}

// git.IssueCommit / IssueBranch / IssuePullRequest (internal/domain/git/config.go),
// as returned by GET /issue/{issueKey}/git.
export interface IssueCommit {
  id: string;
  issue_id: string;
  provider_id?: string;
  commit_sha: string;
  message: string;
  author: string;
  committed_at?: string;
}

export interface IssueBranch {
  id: string;
  issue_id: string;
  provider_id?: string;
  branch_name: string;
  repo_url: string;
}

export interface IssuePullRequest {
  id: string;
  issue_id: string;
  provider_id?: string;
  pr_number: number;
  title: string;
  url: string;
  state: string;
  created_at: string;
  merged_at?: string;
}

export interface IssueGitInfo {
  commits: IssueCommit[];
  branches: IssueBranch[];
  pull_requests: IssuePullRequest[];
}

// automation.AutomationRule (internal/domain/automation/model.go).
export interface AutomationRule {
  id: string;
  project_id: string;
  name: string;
  is_active: boolean;
  trigger_type: string;
  conditions_json: string;
  actions_json: string;
  created_at: string;
}

export interface AutomationRun {
  id: string;
  rule_id: string;
  issue_id?: string;
  triggered_at: string;
  status: string; // success | skipped | error | test
  log: string;
}

export const integrations = {
  webhooks: (projectKey: string) => apiFetch<Webhook[]>(`/rest/api/3/project/${projectKey}/webhooks`),
  createWebhook: (projectKey: string, url: string, events: string[]) =>
    apiFetch<Webhook>(`/rest/api/3/project/${projectKey}/webhooks`, {
      method: "POST",
      body: JSON.stringify({ url, events }),
    }),
  deleteWebhook: (projectKey: string, id: string) =>
    apiFetch<void>(`/rest/api/3/project/${projectKey}/webhooks/${id}`, { method: "DELETE" }),
  // Real route is /project/{key}/git/providers (the plan guessed /git-provider).
  // GetProvider 404s (via apiFetch throw) when none is configured yet, rather
  // than returning null.
  gitProvider: (projectKey: string) =>
    apiFetch<GitProviderConfig>(`/rest/api/3/project/${projectKey}/git/providers`),
  configureGit: (
    projectKey: string,
    body: { provider_type: string; base_url: string; token: string; webhook_secret: string }
  ) =>
    apiFetch<GitProviderConfig>(`/rest/api/3/project/${projectKey}/git/providers`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  automationRules: (key: string) =>
    apiFetch<AutomationRule[]>(`/rest/api/3/project/${key}/automation`),
  automationCreate: (
    key: string,
    body: { name: string; trigger_type: string; conditions_json: string; actions_json: string }
  ) =>
    apiFetch<AutomationRule>(`/rest/api/3/project/${key}/automation`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  automationGet: (ruleId: string) =>
    apiFetch<AutomationRule>(`/rest/api/3/automation/${ruleId}`),
  automationUpdate: (
    ruleId: string,
    patch: Partial<{ name: string; is_active: boolean; trigger_type: string; conditions_json: string; actions_json: string }>
  ) =>
    apiFetch<AutomationRule>(`/rest/api/3/automation/${ruleId}`, {
      method: "PATCH",
      body: JSON.stringify(patch),
    }),
  automationDelete: (ruleId: string) =>
    apiFetch<void>(`/rest/api/3/automation/${ruleId}`, { method: "DELETE" }),
  automationTest: (ruleId: string, issueId: string) =>
    apiFetch<AutomationRun>(`/rest/api/3/automation/${ruleId}/execute`, {
      method: "POST",
      body: JSON.stringify({ issue_id: issueId }),
    }),
  automationRuns: (ruleId: string) =>
    apiFetch<AutomationRun[]>(`/rest/api/3/automation/${ruleId}/runs`),
};

export const issueGit = {
  info: (issueKey: string) => apiFetch<IssueGitInfo>(`/rest/api/3/issue/${issueKey}/git`),
};

// ── Issue links ──────────────────────────────────────────────────────────────

// v3.LinkTypeRef (internal/api/v3/collab.go). "inward"/"outward" are the two
// human-readable phrasings of the same relation depending on which side you
// stand on, e.g. Blocks: outward="blocks", inward="is blocked by".
export interface LinkTypeRef {
  id: string;
  name: string;
  inward: string;
  outward: string;
  self?: string;
}

// v3.LinkedIssueForIssue: the minimal shape (key + summary/status) of "the
// other end" of a link, as returned by GET /issue/{key}/issuelinks.
export interface LinkedIssueForIssue {
  key: string;
  fields: {
    summary: string;
    status?: StatusRef;
  };
}

// v3.IssueLinkForIssue: only the side opposite the requested issue is
// populated — see the handler comment in internal/api/handlers/issuelink_handler.go.
// If `inwardIssue` is populated, the requested issue is the outward/source
// side of the link; if `outwardIssue` is populated, the requested issue is
// the inward/target side.
export interface IssueLinkForIssue {
  id: string;
  type: LinkTypeRef;
  inwardIssue?: LinkedIssueForIssue;
  outwardIssue?: LinkedIssueForIssue;
}

export interface IssueLinksResponse {
  issuelinks: IssueLinkForIssue[];
}

export type IssueLinkTypeName = "Blocks" | "Duplicate" | "Relates";

export const issueLinks = {
  list: (issueKey: string) => apiFetch<IssueLinksResponse>(`/rest/api/3/issue/${issueKey}/issuelinks`),
  // Convention (matches internal/api/handlers/issuelink_handler.go Create):
  // outwardKey is always the link's source, inwardKey its target. Callers
  // decide which one is "this issue" based on the chosen relation.
  create: (payload: { typeName: IssueLinkTypeName; outwardKey: string; inwardKey: string }) =>
    apiFetch<void>("/rest/api/3/issueLink", {
      method: "POST",
      body: JSON.stringify({
        type: { name: payload.typeName },
        outwardIssue: { key: payload.outwardKey },
        inwardIssue: { key: payload.inwardKey },
      }),
    }),
  delete: (id: string) => apiFetch<void>(`/rest/api/3/issueLink/${id}`, { method: "DELETE" }),
};

// ── Attachments ──────────────────────────────────────────────────────────────

// v3.Attachment (internal/api/v3/attachment.go). `content` is the path to
// GET /rest/api/3/attachment/content/{id} — bearer-protected like every other
// route (see internal/api/router.go, internal/api/middleware/auth.go: no
// cookie/query-token fallback), so it can NEVER be used as a naked <a href>/
// <img src>: fetch it with the token via contentBlobUrl() and use the
// resulting object URL instead.
export interface Attachment {
  id: string;
  filename: string;
  size: number;
  mimeType: string;
  created: string;
  content: string;
}

// Shared by upload/contentBlobUrl below: apiFetch forces
// Content-Type: application/json, which breaks multipart (the browser needs
// to set its own boundary) and isn't meaningful for a binary GET either — so
// both bypass apiFetch and rebuild just the auth + 401-redirect handling.
function authHeaders(): HeadersInit {
  const token = getToken();
  return token ? { Authorization: `Bearer ${token}` } : {};
}

function handleUnauthorized(res: Response): void {
  if (res.status === 401) {
    if (typeof window !== "undefined") {
      localStorage.removeItem("token");
      window.location.href = "/login";
    }
    throw new Error("Unauthorized");
  }
}

export const attachments = {
  list: (issueKey: string) => apiFetch<Attachment[]>(`/rest/api/3/issue/${issueKey}/attachments`),

  // Multipart upload — deliberately does NOT go through apiFetch. Field name
  // must be "file" (see AttachmentHandler.Upload's r.FormFile("file")).
  upload: async (issueKey: string, file: File): Promise<Attachment> => {
    const form = new FormData();
    form.append("file", file);
    const res = await fetch(`${BASE_URL}/rest/api/3/issue/${issueKey}/attachments`, {
      method: "POST",
      headers: authHeaders(),
      body: form,
    });
    handleUnauthorized(res);
    if (!res.ok) {
      const text = await res.text();
      let msg = `HTTP ${res.status}`;
      try {
        const json = JSON.parse(text);
        if (json.error) msg = json.error;
      } catch {
        /* ignore */
      }
      throw new Error(msg);
    }
    return res.json() as Promise<Attachment>;
  },

  delete: (id: string) => apiFetch<void>(`/rest/api/3/attachment/${id}`, { method: "DELETE" }),

  // Fetches the attachment's bytes with the bearer token and returns an
  // object URL suitable for <img src>/download links. AttachmentHandler.
  // ServeFile always answers with Content-Type: application/octet-stream
  // regardless of the real file type, so the blob is re-tagged with the
  // mimeType already known from the list/upload response — otherwise image
  // previews would never render. Callers must URL.revokeObjectURL() the
  // result once done with it.
  contentBlobUrl: async (att: Attachment): Promise<string> => {
    const res = await fetch(`${BASE_URL}${att.content}`, { headers: authHeaders() });
    handleUnauthorized(res);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const raw = await res.blob();
    const typed = att.mimeType ? new Blob([raw], { type: att.mimeType }) : raw;
    return URL.createObjectURL(typed);
  },
};
