// Filters page (/app/filters): JQL search, saved filters and sharing. The JQL
// text and filter names are data.
export const filters = {
  title: "Filters",
  filterName: "Filter name",
  shareWithTeam: "Share with team",
  save: "Save",
  invalidJql: "Invalid JQL",
  saved: "Saved filters",
  noneYet: "None yet",
  resultCount: { one: "{count} result", other: "{count} results" },
  prev: "Prev",
  next: "Next",
  row: {
    editName: "Edit filter name",
    shared: "Shared",
    sharedTitle: "Shared with team",
    edit: "Edit filter",
    editTitle: "Edit {name}",
    delete: "Delete filter",
    deleteTitle: "Delete {name}",
  },
} as const;
