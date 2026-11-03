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
    remove: "Remove",
    search: "Search",
  },
  never: "never",
} as const;
