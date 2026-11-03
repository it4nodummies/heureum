// Shared words used across areas. English is the source of truth: every key
// added here must also be added to messages/it/common.ts (the build fails otherwise).
export const common = {
  actions: {
    save: "Save",
    cancel: "Cancel",
    delete: "Delete",
    create: "Create",
    edit: "Edit",
    close: "Close",
    add: "Add",
    remove: "Remove",
    confirm: "Confirm",
    search: "Search",
  },
  loading: "Loading…",
  never: "never",
  error: "An error occurred",
  comingSoon: "Coming soon",
  issueCount: { one: "{count} issue", other: "{count} issues" },
} as const;
