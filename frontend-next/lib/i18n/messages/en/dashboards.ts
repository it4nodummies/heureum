// Dashboards list + detail. Dashboard names are data; gadget titles are chrome.
export const dashboards = {
  title: "Dashboards",
  newName: "New dashboard name",
  namePlaceholder: "Dashboard name",
  empty: "No dashboards yet",
  detail: {
    fallbackTitle: "Dashboard",
    gadgetType: "Gadget type",
    addGadget: "Add gadget",
    noGadgets: "No gadgets",
  },
  gadgets: {
    assignedToMe: "Assigned to me",
    activityStream: "Activity stream",
    noAssigned: "No assigned issues",
    noActivity: "No recent activity",
    // One issue_history row. {actor}, {field} and {key} are spliced in as nodes;
    // the field id is labelled via issues.history.fields (raw id if unknown);
    // actor and values are data.
    changed: "{actor} changed {field} on {key}: {from} → {to}",
    // The "created" row (English keeps the generic sentence's output).
    created: "{actor} changed created on {key}: {from} → {to}",
  },
} as const;
