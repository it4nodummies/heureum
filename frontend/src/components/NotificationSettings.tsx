import { useState, useEffect } from 'react'
import { BellOff } from 'lucide-react'

interface Setting {
  user_id: string
  project_id: string
  event_type: string
  via_email: boolean
  via_app: boolean
}

const API = 'http://localhost:8080/api/v1'

function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem('token') || ''
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  }
}

const EVENT_LABELS: Record<string, string> = {
  issue_assigned: 'Issue assigned to me',
  issue_commented: 'New comments on watched issues',
  issue_mentioned: 'Mentions in comments',
  sprint_started: 'Sprint started',
  sprint_completed: 'Sprint completed',
  status_changed: 'Status changes on watched issues',
}

export default function NotificationSettings() {
  const [open, setOpen] = useState(false)
  const [settings, setSettings] = useState<Setting[]>([])
  const [loading, setLoading] = useState(false)

  const fetchSettings = async () => {
    try {
      const res = await fetch(`${API}/notifications/settings`, { headers: getAuthHeaders() })
      if (res.ok) {
        const data = await res.json()
        setSettings(data || [])
      }
    } catch {
      // ignore
    }
  }

  useEffect(() => {
    if (open) fetchSettings()
  }, [open])

  const handleToggle = async (eventType: string) => {
    const existing = settings.find((s) => s.event_type === eventType)
    const viaApp = existing ? !existing.via_app : false
    setLoading(true)
    await fetch(`${API}/notifications/settings`, {
      method: 'PATCH',
      headers: getAuthHeaders(),
      body: JSON.stringify({
        project_id: '',
        event_type: eventType,
        via_email: existing?.via_email ?? true,
        via_app: viaApp,
      }),
    })
    setSettings((prev) =>
      prev.some((s) => s.event_type === eventType)
        ? prev.map((s) => (s.event_type === eventType ? { ...s, via_app: viaApp } : s))
        : [
            ...prev,
            {
              user_id: '',
              project_id: '',
              event_type: eventType,
              via_email: true,
              via_app: viaApp,
            },
          ]
    )
    setLoading(false)
  }

  const eventTypes = Object.keys(EVENT_LABELS)

  return (
    <div className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="p-2 rounded hover:bg-gray-700 transition-colors"
        title="Notification settings"
      >
        <BellOff size={20} />
      </button>
      {open && (
        <div className="absolute right-0 top-full mt-2 w-72 bg-gray-800 border border-gray-700 rounded-lg shadow-xl z-50">
          <div className="px-4 py-3 border-b border-gray-700">
            <h3 className="font-semibold text-white text-sm">Notification Settings</h3>
          </div>
          <div className="p-4 space-y-3">
            {eventTypes.map((eventType) => {
              const setting = settings.find((s) => s.event_type === eventType)
              const enabled = setting ? setting.via_app : true
              return (
                <label
                  key={eventType}
                  className="flex items-center justify-between cursor-pointer"
                >
                  <span className="text-sm text-gray-300">{EVENT_LABELS[eventType]}</span>
                  <button
                    role="switch"
                    aria-checked={enabled}
                    disabled={loading}
                    className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
                      enabled ? 'bg-blue-600' : 'bg-gray-600'
                    }`}
                    onClick={() => handleToggle(eventType)}
                  >
                    <span
                      className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                        enabled ? 'translate-x-4.5' : 'translate-x-0.5'
                      }`}
                    />
                  </button>
                </label>
              )
            })}
            <button
              onClick={() => setOpen(false)}
              className="w-full mt-2 px-3 py-1.5 text-sm bg-gray-700 text-gray-300 rounded hover:bg-gray-600"
            >
              Close
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
