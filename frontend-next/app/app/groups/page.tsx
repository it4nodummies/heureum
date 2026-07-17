"use client";

// Global Groups admin page. There is no "list all groups" endpoint, so the
// list is driven off groups.picker("") (an empty query returns all/most
// groups; FoundGroups has a `groups` array of {name, groupId}). Group
// mutations require GLOBAL ADMIN (the demo admin@example.com is one). Every
// mutation invalidates the ["groups"] query. Mirrors the AccessTab pattern for
// the debounced global user-search add flow.

import { useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { groups as groupsApi, profile, type JiraUser } from "@/lib/api";

export default function GroupsPage() {
  const qc = useQueryClient();

  const list = useQuery({
    queryKey: ["groups"],
    queryFn: () => groupsApi.picker(""),
  });
  const invalidate = () => qc.invalidateQueries({ queryKey: ["groups"] });

  const [newName, setNewName] = useState("");
  const create = useMutation({
    mutationFn: (name: string) => groupsApi.create(name),
    onSuccess: () => {
      setNewName("");
      invalidate();
    },
  });

  const del = useMutation({
    mutationFn: (name: string) => groupsApi.del(name),
    onSuccess: invalidate,
  });

  const [expanded, setExpanded] = useState<string | null>(null);

  const groupList = list.data?.groups ?? [];

  return (
    <div className="mx-auto max-w-3xl px-6 py-8" data-testid="groups-admin">
      <header className="mb-6">
        <h1 className="text-xl font-semibold text-[#1a1f36]">Groups</h1>
        <p className="mt-1 text-sm text-slate-500">
          Manage global groups and their members. Requires global admin.
        </p>
      </header>

      {/* Create group */}
      <section className="mb-6 rounded-xl border border-slate-200 p-4">
        <h2 className="mb-3 text-sm font-semibold text-slate-700">Create group</h2>
        <form
          className="flex flex-wrap items-center gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (newName.trim()) create.mutate(newName.trim());
          }}
        >
          <input
            aria-label="Group name"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder="Group name"
            className="flex-1 rounded border border-slate-300 px-2 py-1.5 text-sm"
          />
          <button
            type="submit"
            disabled={create.isPending || !newName.trim()}
            className="rounded bg-[#0052cc] px-4 py-1.5 text-sm text-white disabled:opacity-60"
          >
            {create.isPending ? "Creating…" : "Create group"}
          </button>
        </form>
        {create.isError && (
          <p className="mt-2 text-sm text-red-600">
            {create.error instanceof Error ? create.error.message : "Failed to create group"}
          </p>
        )}
      </section>

      {/* Group list */}
      <section>
        <h2 className="mb-2 text-sm font-semibold text-slate-700">All groups</h2>
        {list.isLoading && <p className="py-2 text-sm text-slate-400">Loading groups…</p>}
        {list.isError && (
          <p className="py-2 text-sm text-red-600">
            {list.error instanceof Error ? list.error.message : "Failed to load groups"}
          </p>
        )}
        {list.data && groupList.length === 0 && (
          <p className="py-2 text-sm text-slate-400">No groups yet</p>
        )}
        <ul className="space-y-2" data-testid="groups-list">
          {groupList.map((g) => (
            <li key={g.groupId || g.name} className="rounded-xl border border-slate-200">
              <div className="flex items-center gap-3 px-4 py-3">
                <button
                  type="button"
                  onClick={() => setExpanded(expanded === g.name ? null : g.name)}
                  className="flex-1 text-left text-sm font-medium text-[#1a1f36] hover:text-[#0052cc]"
                  aria-expanded={expanded === g.name}
                >
                  {g.name}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    if (confirm(`Delete group "${g.name}"?`)) del.mutate(g.name);
                  }}
                  className="text-xs text-red-600 hover:underline"
                  aria-label={`Delete ${g.name}`}
                >
                  Delete
                </button>
              </div>
              {expanded === g.name && <GroupMembers groupname={g.name} />}
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}

// ── Per-group member management ───────────────────────────────────────────────

function GroupMembers({ groupname }: { groupname: string }) {
  const qc = useQueryClient();

  const members = useQuery({
    queryKey: ["group-members", groupname],
    queryFn: () => groupsApi.members(groupname),
  });
  const invalidate = () => qc.invalidateQueries({ queryKey: ["group-members", groupname] });

  const removeUser = useMutation({
    mutationFn: (accountId: string) => groupsApi.removeUser(groupname, accountId),
    onSuccess: invalidate,
  });

  // Add user via GLOBAL user search (profile.searchUsers), debounced.
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(query), 250);
    return () => clearTimeout(t);
  }, [query]);

  const searchQ = useQuery({
    queryKey: ["userSearch", debouncedQuery],
    queryFn: () => profile.searchUsers(debouncedQuery),
    enabled: debouncedQuery.trim().length > 0,
  });

  const addUser = useMutation({
    mutationFn: (accountId: string) => groupsApi.addUser(groupname, accountId),
    onSuccess: () => {
      setQuery("");
      setDebouncedQuery("");
      invalidate();
    },
  });

  const memberList = members.data?.values ?? [];
  const existingIds = new Set(memberList.map((m: JiraUser) => m.accountId));

  return (
    <div className="border-t border-slate-100 px-4 py-3 space-y-3">
      {members.isLoading && <p className="text-sm text-slate-400">Loading members…</p>}
      {members.isError && (
        <p className="text-sm text-red-600">
          {members.error instanceof Error ? members.error.message : "Failed to load members"}
        </p>
      )}
      <ul className="space-y-1" data-testid="group-members-list">
        {memberList.map((m: JiraUser) => (
          <li
            key={m.accountId}
            data-testid="group-member-row"
            className="flex items-center gap-3 text-sm"
          >
            <div className="min-w-0 flex-1">
              <span className="text-[#1a1f36]">{m.displayName || m.accountId}</span>
              {m.emailAddress && (
                <span className="ml-2 text-xs text-slate-400">{m.emailAddress}</span>
              )}
            </div>
            <button
              type="button"
              onClick={() => removeUser.mutate(m.accountId)}
              className="text-xs text-red-600 hover:underline"
              aria-label={`Remove ${m.displayName || m.accountId} from ${groupname}`}
            >
              Remove
            </button>
          </li>
        ))}
        {members.data && memberList.length === 0 && (
          <li className="text-sm text-slate-400">No members yet</li>
        )}
      </ul>

      <div className="relative">
        <input
          aria-label={`Add user to ${groupname}`}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search people to add…"
          className="w-full rounded border border-slate-300 px-2 py-1.5 text-sm"
        />
        {debouncedQuery.trim().length > 0 && (
          <div className="mt-1 max-h-56 overflow-y-auto rounded border border-slate-200">
            {searchQ.isFetching && (
              <div className="px-2 py-1.5 text-xs text-slate-400">Searching…</div>
            )}
            {!searchQ.isFetching &&
              (searchQ.data ?? [])
                .filter((u) => !existingIds.has(u.accountId))
                .map((u) => (
                  <button
                    key={u.accountId}
                    type="button"
                    onClick={() => addUser.mutate(u.accountId)}
                    className="flex w-full items-center gap-2 px-2 py-1.5 text-left text-sm hover:bg-slate-50"
                  >
                    <span className="text-[#1a1f36]">{u.displayName}</span>
                    {u.emailAddress && (
                      <span className="text-xs text-slate-400">{u.emailAddress}</span>
                    )}
                  </button>
                ))}
            {!searchQ.isFetching &&
              (searchQ.data ?? []).filter((u) => !existingIds.has(u.accountId)).length === 0 && (
                <div className="px-2 py-1.5 text-xs text-slate-400">No matching people</div>
              )}
          </div>
        )}
        {addUser.isError && (
          <p className="mt-1 text-sm text-red-600">
            {addUser.error instanceof Error ? addUser.error.message : "Failed to add user"}
          </p>
        )}
      </div>
    </div>
  );
}
