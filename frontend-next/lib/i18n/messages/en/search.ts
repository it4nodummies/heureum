// Global search (TopBar) and the shared issue list (SearchResults). JQL text and
// data (summaries, status/priority names) are never translated.
export const search = {
  global: {
    label: "Search",
    placeholder: "Search",
  },
  columns: {
    label: "Columns",
    key: "Key",
    summary: "Summary",
    status: "Status",
    priority: "Priority",
    assignee: "Assignee",
    updated: "Updated",
  },
  bulk: {
    selected: { one: "{count} selected", other: "{count} selected" },
    priority: "Bulk priority",
    priorityPlaceholder: "Priority…",
    assignee: "Bulk assignee",
    assigneePlaceholder: "Assignee…",
    labels: "Set label(s)",
    labelsPlaceholder: "Set label(s), comma-separated",
    labelsHint: "Replaces the existing labels on the selected issues",
    apply: "Apply",
    clear: "Clear",
    failed: "Some issues could not be updated: {keys}",
  },
  table: {
    selectAll: "Select all",
    selectRow: "Select {key}",
    noResults: "No results",
    priorityFor: "Priority for {key}",
    statusFor: "Status for {key}",
    assigneeFor: "Assignee for {key}",
    noTransitions: "No transitions available",
    unassigned: "Unassigned",
  },
  people: {
    search: "Search people",
    searchFor: "Search people for {key}",
    placeholder: "Search people…",
    noMatches: "No matching people",
  },
} as const;
