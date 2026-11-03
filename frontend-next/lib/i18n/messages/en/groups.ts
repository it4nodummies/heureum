// Teams admin page (/app/groups). Team names and member names are data.
export const groups = {
  title: "Teams",
  subtitle: "Manage teams and their members. Requires global admin.",
  create: {
    title: "Create team",
    name: "Team name",
    submit: "Create team",
    creating: "Creating…",
    failed: "Failed to create team",
  },
  list: {
    title: "All teams",
    loading: "Loading teams…",
    failed: "Failed to load teams",
    empty: "No teams yet",
    confirmDelete: 'Delete team "{name}"?',
    deleteAria: "Delete {name}",
  },
  members: {
    loading: "Loading members…",
    failed: "Failed to load members",
    empty: "No members yet",
    removeAria: "Remove {user} from {group}",
    addAria: "Add user to {group}",
    addPlaceholder: "Search people to add…",
    searching: "Searching…",
    noMatches: "No matching people",
    addFailed: "Failed to add user",
  },
} as const;
