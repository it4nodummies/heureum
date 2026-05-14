import { useState, useEffect, useRef, useCallback } from 'react'
import { Bell } from 'lucide-react'
import NotificationList from './NotificationList'

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

const API = 'http://localhost:8080/api/v1'

function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem('token') || ''
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  }
}

export default function NotificationBell() {
  const [unreadCount, setUnreadCount] = useState(0)
  const [open, setOpen] = useState(false)
  const [notifs, setNotifs] = useState<Notif[]>([])
  const ref = useRef<HTMLDivElement>(null)

  const fetchUnreadCount = useCallback(async () => {
    try {
      const res = await fetch(`${API}/notifications/unread-count`, { headers: getAuthHeaders() })
      if (res.ok) {
        const data = await res.json()
        setUnreadCount(data.count || 0)
      }
    } catch {
      // ignore
    }
  }, [])

  const fetchNotifications = useCallback(async () => {
    try {
      const res = await fetch(`${API}/notifications?unread=false`, { headers: getAuthHeaders() })
      if (res.ok) {
        const data = await res.json()
        setNotifs(data || [])
      }
    } catch {
      // ignore
    }
  }, [])

  useEffect(() => {
    fetchUnreadCount()
    const interval = setInterval(fetchUnreadCount, 30000)
    return () => clearInterval(interval)
  }, [fetchUnreadCount])

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  const handleToggle = () => {
    if (!open) {
      fetchNotifications()
    }
    setOpen(!open)
  }

  const handleMarkRead = async (id: string) => {
    await fetch(`${API}/notifications/${id}/read`, {
      method: 'PATCH',
      headers: getAuthHeaders(),
    })
    setNotifs((prev) =>
      prev.map((n) => (n.id === id ? { ...n, is_read: true } : n))
    )
    fetchUnreadCount()
  }

  const handleMarkAllRead = async () => {
    await fetch(`${API}/notifications/read-all`, {
      method: 'PATCH',
      headers: getAuthHeaders(),
    })
    setNotifs((prev) => prev.map((n) => ({ ...n, is_read: true })))
    fetchUnreadCount()
  }

  return (
    <div ref={ref} className="relative">
      <button
        onClick={handleToggle}
        className="relative p-2 rounded hover:bg-gray-700 transition-colors"
      >
        <Bell size={20} />
        {unreadCount > 0 && (
          <span className="absolute -top-1 -right-1 bg-red-500 text-white text-xs rounded-full h-5 min-w-5 flex items-center justify-center px-1 font-bold">
            {unreadCount > 99 ? '99+' : unreadCount}
          </span>
        )}
      </button>
      {open && (
        <div className="absolute right-0 top-full mt-2 w-80 max-h-96 bg-gray-800 border border-gray-700 rounded-lg shadow-xl z-50 flex flex-col">
          <div className="flex items-center justify-between px-4 py-3 border-b border-gray-700">
            <h3 className="font-semibold text-white text-sm">Notifications</h3>
            {unreadCount > 0 && (
              <button
                onClick={handleMarkAllRead}
                className="text-xs text-blue-400 hover:text-blue-300"
              >
                Mark all read
              </button>
            )}
          </div>
          <div className="overflow-y-auto flex-1">
            <NotificationList notifs={notifs} onMarkRead={handleMarkRead} />
          </div>
        </div>
      )}
    </div>
  )
}
