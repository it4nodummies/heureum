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
    // "<actor> changed <field> on <ISSUE-1>: old → new" — actor/field/values are data.
    changedOn: "changed {field} on",
  },
} as const;
