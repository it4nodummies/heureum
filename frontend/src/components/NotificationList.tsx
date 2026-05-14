interface Notif {
  id: string
  user_id: string
  type: string
  title: string
  body: string
  link: string
  is_read: boolean
  created_at: number
}

interface Props {
  notifs: Notif[]
  onMarkRead: (id: string) => void
}

const TYPE_ICONS: Record<string, string> = {
  assignment: '\u{1F4CC}',
  comment: '\u{1F4AC}',
  mention: '\u{0040}',
  status_change: '\u{1F504}',
  sprint_started: '\u{1F680}',
  sprint_completed: '\u{2705}',
}

function timeAgo(ts: number): string {
  const seconds = Math.floor((Date.now() - ts) / 1000)
  if (seconds < 60) return 'just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  if (days < 7) return `${days}d ago`
  return new Date(ts).toLocaleDateString()
}

export default function NotificationList({ notifs, onMarkRead }: Props) {
  if (notifs.length === 0) {
    return (
      <div className="px-4 py-6 text-center text-gray-400 text-sm">
        No notifications yet.
      </div>
    )
  }

  return (
    <div className="divide-y divide-gray-700">
      {notifs.map((n) => (
        <div
          key={n.id}
          className={`px-4 py-3 cursor-pointer transition-colors hover:bg-gray-700/50 ${
            !n.is_read ? 'bg-blue-900/20 border-l-2 border-l-blue-500' : ''
          }`}
          onClick={() => !n.is_read && onMarkRead(n.id)}
        >
          <div className="flex items-start gap-2">
            <span className="text-sm flex-shrink-0">{TYPE_ICONS[n.type] || '\u{1F514}'}</span>
            <div className="min-w-0 flex-1">
              <p className={`text-sm ${!n.is_read ? 'text-white font-medium' : 'text-gray-300'}`}>
                {n.title}
              </p>
              {n.body && (
                <p className="text-xs text-gray-400 mt-0.5 truncate">{n.body}</p>
              )}
              <p className="text-xs text-gray-500 mt-1">{timeAgo(n.created_at)}</p>
            </div>
            {!n.is_read && (
              <span className="w-2 h-2 bg-blue-500 rounded-full flex-shrink-0 mt-1.5" />
            )}
          </div>
        </div>
      ))}
    </div>
  )
}
