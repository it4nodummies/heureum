"use client";

// Access / People tab: lists a project's members with their per-project role,
// lets an admin change a member's role (upsert via members.add), remove a
// member, and add a new member found through the GLOBAL user search
// (profile.searchUsers — NOT users.assignableSearch, which only returns
// existing members). Every mutation invalidates ["members", projectKey].

import { useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  projects as projectsApi,
  projectTeams as projectTeamsApi,
  groups as groupsApi,
  profile,
  type ProjectMember,
  type ProjectRole,
  type ProjectTeam,
  type JiraUser,
} from "@/lib/api";

const ROLES: ProjectRole[] = ["admin", "member", "viewer"];

export function AccessTab({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();

  const members = useQuery({
    queryKey: ["members", projectKey],
    queryFn: () => projectsApi.members.list(projectKey),
  });
  const invalidate = () => qc.invalidateQueries({ queryKey: ["members", projectKey] });

  const changeRole = useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: ProjectRole }) =>
      projectsApi.members.add(projectKey, { user_id: userId, role }),
    onSuccess: invalidate,
  });

  const remove = useMutation({
    mutationFn: (userId: string) => projectsApi.members.remove(projectKey, userId),
    onSuccess: invalidate,
  });

  // ── Add member (global search) ─────────────────────────────────────────────
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [selected, setSelected] = useState<JiraUser | null>(null);
  const [newRole, setNewRole] = useState<ProjectRole>("member");

  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(query), 250);
    return () => clearTimeout(t);
  }, [query]);

  const search = useQuery({
    queryKey: ["userSearch", debouncedQuery],
    queryFn: () => profile.searchUsers(debouncedQuery),
    enabled: debouncedQuery.trim().length > 0,
  });

  const existingIds = new Set((members.data ?? []).map((m) => m.accountId));

  const add = useMutation({
    mutationFn: (userId: string) =>
      projectsApi.members.add(projectKey, { user_id: userId, role: newRole }),
    onSuccess: () => {
      setSelected(null);
      setQuery("");
      setDebouncedQuery("");
      setNewRole("member");
      invalidate();
    },
  });

  // ── Teams (group→project role) ─────────────────────────────────────────────
  const teams = useQuery({
    queryKey: ["projectTeams", projectKey],
    queryFn: () => projectTeamsApi.list(projectKey),
  });
  const invalidateTeams = () =>
    qc.invalidateQueries({ queryKey: ["projectTeams", projectKey] });

  const changeTeamRole = useMutation({
    mutationFn: ({ groupId, role }: { groupId: string; role: ProjectRole }) =>
      projectTeamsApi.updateRole(projectKey, groupId, role),
    onSuccess: invalidateTeams,
  });

  const removeTeam = useMutation({
    mutationFn: (groupId: string) => projectTeamsApi.remove(projectKey, groupId),
    onSuccess: invalidateTeams,
  });

  // All teams (groups) available to associate, minus the already-associated ones.
  const groupPicker = useQuery({
    queryKey: ["groupPicker", ""],
    queryFn: () => groupsApi.picker(""),
  });

  const [newTeamId, setNewTeamId] = useState("");
  const [newTeamRole, setNewTeamRole] = useState<ProjectRole>("member");

  const associatedGroupIds = new Set((teams.data ?? []).map((t) => t.groupId));
  const availableTeams = (groupPicker.data?.groups ?? []).filter(
    (g) => !associatedGroupIds.has(g.groupId)
  );

  const addTeam = useMutation({
    mutationFn: ({ groupId, role }: { groupId: string; role: ProjectRole }) =>
      projectTeamsApi.add(projectKey, groupId, role),
    onSuccess: () => {
      setNewTeamId("");
      setNewTeamRole("member");
      invalidateTeams();
      qc.invalidateQueries({ queryKey: ["groupPicker", ""] });
    },
  });

  return (
    <div className="space-y-6" data-testid="access-tab">
      <section>
        <h3 className="mb-2 text-sm font-semibold text-slate-700">Members</h3>
        {members.isLoading && <p className="py-2 text-sm text-slate-400">Loading members…</p>}
        {members.isError && (
          <p className="py-2 text-sm text-red-600">
            {members.error instanceof Error ? members.error.message : "Failed to load members"}
          </p>
        )}
        <ul className="space-y-1" data-testid="members-list">
          {(members.data ?? []).map((m: ProjectMember) => (
            <li
              key={m.accountId}
              data-testid="member-row"
              className="flex items-center gap-3 border-b border-slate-100 py-2 text-sm"
            >
              <div className="min-w-0 flex-1">
                <span className="text-[#1a1f36]">{m.displayName || m.accountId}</span>
                {m.emailAddress && (
                  <span className="ml-2 text-xs text-slate-400">{m.emailAddress}</span>
                )}
              </div>
              <select
                aria-label={`Role for ${m.displayName || m.accountId}`}
                value={m.role}
                onChange={(e) =>
                  changeRole.mutate({ userId: m.accountId, role: e.target.value as ProjectRole })
                }
                className="rounded border border-slate-300 px-2 py-1 text-sm"
              >
                {ROLES.map((r) => (
                  <option key={r} value={r}>
                    {r}
                  </option>
                ))}
              </select>
              <button
                onClick={() => remove.mutate(m.accountId)}
                className="text-xs text-red-600 hover:underline"
                aria-label={`Remove ${m.displayName || m.accountId}`}
              >
                Remove
              </button>
            </li>
          ))}
          {members.data && members.data.length === 0 && (
            <li className="py-2 text-sm text-slate-400">No members yet</li>
          )}
        </ul>
      </section>

      <section className="rounded-xl border border-slate-200 p-4 space-y-3">
        <h3 className="text-sm font-semibold text-slate-700">Add member</h3>

        {selected ? (
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm text-[#1a1f36]">
              {selected.displayName}
              {selected.emailAddress && (
                <span className="ml-2 text-xs text-slate-400">{selected.emailAddress}</span>
              )}
            </span>
            <button
              onClick={() => setSelected(null)}
              className="text-xs text-slate-500 hover:underline"
            >
              Change
            </button>
            <select
              aria-label="New member role"
              value={newRole}
              onChange={(e) => setNewRole(e.target.value as ProjectRole)}
              className="rounded border border-slate-300 px-2 py-1 text-sm"
            >
              {ROLES.map((r) => (
                <option key={r} value={r}>
                  {r}
                </option>
              ))}
            </select>
            <button
              onClick={() => add.mutate(selected.accountId)}
              disabled={add.isPending}
              className="rounded bg-[#0052cc] px-4 py-1 text-sm text-white disabled:opacity-60"
            >
              {add.isPending ? "Adding…" : "Add"}
            </button>
          </div>
        ) : (
          <div className="relative">
            <input
              aria-label="Search people"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search people to add…"
              className="w-full rounded border border-slate-300 px-2 py-1.5 text-sm"
            />
            {debouncedQuery.trim().length > 0 && (
              <div className="mt-1 max-h-56 overflow-y-auto rounded border border-slate-200">
                {search.isFetching && (
                  <div className="px-2 py-1.5 text-xs text-slate-400">Searching…</div>
                )}
                {!search.isFetching &&
                  (search.data ?? [])
                    .filter((u) => !existingIds.has(u.accountId))
                    .map((u) => (
                      <button
                        key={u.accountId}
                        type="button"
                        onClick={() => setSelected(u)}
                        className="flex w-full items-center gap-2 px-2 py-1.5 text-left text-sm hover:bg-slate-50"
                      >
                        <span className="text-[#1a1f36]">{u.displayName}</span>
                        {u.emailAddress && (
                          <span className="text-xs text-slate-400">{u.emailAddress}</span>
                        )}
                      </button>
                    ))}
                {!search.isFetching &&
                  (search.data ?? []).filter((u) => !existingIds.has(u.accountId)).length === 0 && (
                    <div className="px-2 py-1.5 text-xs text-slate-400">No matching people</div>
                  )}
              </div>
            )}
          </div>
        )}

        {add.isError && (
          <p className="text-sm text-red-600">
            {add.error instanceof Error ? add.error.message : "Failed to add member"}
          </p>
        )}
      </section>

      <section>
        <h3 className="mb-2 text-sm font-semibold text-slate-700">Teams</h3>
        {teams.isLoading && <p className="py-2 text-sm text-slate-400">Loading teams…</p>}
        {teams.isError && (
          <p className="py-2 text-sm text-red-600">
            {teams.error instanceof Error ? teams.error.message : "Failed to load teams"}
          </p>
        )}
        <ul className="space-y-1" data-testid="teams-list">
          {(teams.data ?? []).map((t: ProjectTeam) => (
            <li
              key={t.groupId}
              data-testid="team-row"
              className="flex items-center gap-3 border-b border-slate-100 py-2 text-sm"
            >
              <div className="min-w-0 flex-1">
                <span className="text-[#1a1f36]">{t.name || t.groupId}</span>
              </div>
              <select
                aria-label={`Role for ${t.name || t.groupId}`}
                value={t.role}
                onChange={(e) =>
                  changeTeamRole.mutate({ groupId: t.groupId, role: e.target.value as ProjectRole })
                }
                className="rounded border border-slate-300 px-2 py-1 text-sm"
              >
                {ROLES.map((r) => (
                  <option key={r} value={r}>
                    {r}
                  </option>
                ))}
              </select>
              <button
                onClick={() => removeTeam.mutate(t.groupId)}
                className="text-xs text-red-600 hover:underline"
                aria-label={`Remove ${t.name || t.groupId}`}
              >
                Remove
              </button>
            </li>
          ))}
          {teams.data && teams.data.length === 0 && (
            <li className="py-2 text-sm text-slate-400">No teams yet</li>
          )}
        </ul>
      </section>

      <section className="rounded-xl border border-slate-200 p-4 space-y-3">
        <h3 className="text-sm font-semibold text-slate-700">Add team</h3>
        <div className="flex flex-wrap items-center gap-2">
          <select
            aria-label="Team to add"
            value={newTeamId}
            onChange={(e) => setNewTeamId(e.target.value)}
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          >
            <option value="">Select a team…</option>
            {availableTeams.map((g) => (
              <option key={g.groupId} value={g.groupId}>
                {g.name}
              </option>
            ))}
          </select>
          <select
            aria-label="New team role"
            value={newTeamRole}
            onChange={(e) => setNewTeamRole(e.target.value as ProjectRole)}
            className="rounded border border-slate-300 px-2 py-1 text-sm"
          >
            {ROLES.map((r) => (
              <option key={r} value={r}>
                {r}
              </option>
            ))}
          </select>
          <button
            onClick={() => addTeam.mutate({ groupId: newTeamId, role: newTeamRole })}
            disabled={!newTeamId || addTeam.isPending}
            className="rounded bg-[#0052cc] px-4 py-1 text-sm text-white disabled:opacity-60"
          >
            {addTeam.isPending ? "Adding…" : "Add"}
          </button>
        </div>

        {addTeam.isError && (
          <p className="text-sm text-red-600">
            {addTeam.error instanceof Error ? addTeam.error.message : "Failed to add team"}
          </p>
        )}
      </section>
    </div>
  );
}
