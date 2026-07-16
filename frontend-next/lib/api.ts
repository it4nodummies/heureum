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
    parentKey?: string;
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
  velocity: (key: string) => apiFetch<VelocityData>(`/rest/api/3/project/${key}/reports/velocity`),
  cfd: (key: string) => apiFetch<CFDData>(`/rest/api/3/project/${key}/reports/cfd`),
  pie: (key: string, field: string) => apiFetch<PieSlice[]>(`/rest/api/3/project/${key}/reports/pie?field=${field}`),
  createdVsResolved: (key: string, days = 30) =>
    apiFetch<CreatedVsResolvedData>(`/rest/api/3/project/${key}/reports/created-vs-resolved?days=${days}`),
  summary: (key: string) => apiFetch<ProjectSummary>(`/rest/api/3/project/${key}/summary`),
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
  // Path segment is named {projectID} in the router — ListRules reads it
  // straight off the path with no project-key lookup, so pass the project's
  // internal id (Project.id), not its key.
  automationRules: (projectID: string) => apiFetch<AutomationRule[]>(`/rest/api/3/project/${projectID}/automation`),
};

export const issueGit = {
  info: (issueKey: string) => apiFetch<IssueGitInfo>(`/rest/api/3/issue/${issueKey}/git`),
};
