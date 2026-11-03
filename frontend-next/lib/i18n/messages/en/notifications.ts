// Notification titles/bodies are rendered client-side from the notification
// `type`. The English strings reproduce exactly what the backend stores
// (internal/domain/notification/service.go), so English output is unchanged.
export const notifications = {
  title: "Notifications",
  markAllRead: "Mark all read",
  direct: "Direct",
  watching: "Watching",
  withCount: "{label} ({count})",
  other: "Other",
  read: "Read",
  markRead: "Mark {title} read",
  empty: "No notifications",
  types: {
    assignment: "You were assigned to an issue",
    comment: "New comment on {key}",
    mention: "You were mentioned in {key}",
    sprint_started: "Sprint started",
    sprint_completed: "Sprint completed",
    status_change: "Status changed on {key}",
  },
  bodies: {
    assignment: "You were assigned to {key}: {summary}",
    mention: "Someone mentioned you in the comments of {key}: {summary}",
    sprint_started: "Sprint \"{name}\" has started",
    sprint_completed: "Sprint \"{name}\" has been completed",
    status_change: "{summary} status changed to {status}",
  },
} as const;
