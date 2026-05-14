import { useState, useEffect } from 'react'
import WidgetAssignedToMe from '../components/WidgetAssignedToMe'
import WidgetActivityStream from '../components/WidgetActivityStream'
import { Plus, Trash2, LayoutDashboard } from 'lucide-react'

const API = 'http://localhost:8080/api/v1'

function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem('token')
  return token ? { Authorization: `Bearer ${token}` } : {}
}

interface Dashboard {
  id: string
  name: string
  owner_id: string
  is_public: boolean
  layout_json: string
  created_at: string
  widgets?: Widget[]
}

interface Widget {
  id: string
  widget_type: string
  config_json: string
  position_json: string
  data?: unknown
}

export default function DashboardPage() {
  const [dashboards, setDashboards] = useState<Dashboard[]>([])
  const [selected, setSelected] = useState<Dashboard | null>(null)
  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [loading, setLoading] = useState(false)

  const fetchDashboards = () => {
    setLoading(true)
    fetch(`${API}/dashboards`, { headers: getAuthHeaders() })
      .then((r) => r.json())
      .then((data: Dashboard[]) => {
        if (Array.isArray(data)) {
          setDashboards(data)
          if (!selected && data.length > 0) loadDashboard(data[0].id)
          else if (data.length === 0) setSelected(null)
        }
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }

  const loadDashboard = (id: string) => {
    setLoading(true)
    fetch(`${API}/dashboards/${id}`, { headers: getAuthHeaders() })
      .then((r) => r.json())
      .then((data: Dashboard) => {
        if (data.id) setSelected(data)
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    fetchDashboards()
  }, [])

  const handleCreate = async () => {
    if (!newName.trim()) return
    const res = await fetch(`${API}/dashboards`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...getAuthHeaders() },
      body: JSON.stringify({ name: newName.trim() }),
    })
    if (res.ok) {
      setNewName('')
      setShowCreate(false)
      fetchDashboards()
    }
  }

  const handleAddWidget = async (widgetType: string) => {
    if (!selected) return
    const res = await fetch(`${API}/dashboards/${selected.id}/widgets`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...getAuthHeaders() },
      body: JSON.stringify({ widget_type: widgetType, config_json: '{}' }),
    })
    if (res.ok) loadDashboard(selected.id)
  }

  const handleRemoveWidget = async (widgetId: string) => {
    if (!selected) return
    const res = await fetch(`${API}/dashboards/${selected.id}/widgets/${widgetId}`, {
      method: 'DELETE',
      headers: getAuthHeaders(),
    })
    if (res.ok) loadDashboard(selected.id)
  }

  const renderWidget = (widget: Widget) => {
    switch (widget.widget_type) {
      case 'assigned_to_me':
        return <WidgetAssignedToMe issues={(widget.data as unknown[]) ?? []} />
      case 'activity_stream':
        return <WidgetActivityStream items={(widget.data as unknown[]) ?? []} />
      default:
        return <p className="text-gray-400 text-sm">Unknown widget: {widget.widget_type}</p>
    }
  }

  const widgetLabel = (t: string) => {
    const labels: Record<string, string> = {
      assigned_to_me: 'Assigned to Me',
      activity_stream: 'Activity Stream',
      projects_list: 'Projects List',
    }
    return labels[t] || t
  }

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-white flex items-center gap-2">
          <LayoutDashboard className="w-6 h-6" />
          Dashboard
        </h1>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-1 bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 transition-colors"
        >
          <Plus className="w-4 h-4" />
          New Dashboard
        </button>
      </div>

      {showCreate && (
        <div className="bg-gray-800 rounded-lg p-4 mb-4">
          <input
            type="text"
            placeholder="Dashboard name"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
            className="bg-gray-700 text-gray-200 rounded px-3 py-2 text-sm border border-gray-600 w-full mb-3"
            autoFocus
          />
          <div className="flex gap-2">
            <button
              onClick={handleCreate}
              className="bg-blue-600 text-white px-4 py-1.5 rounded text-sm hover:bg-blue-700"
            >
              Create
            </button>
            <button
              onClick={() => { setShowCreate(false); setNewName('') }}
              className="bg-gray-600 text-gray-300 px-4 py-1.5 rounded text-sm hover:bg-gray-500"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      <div className="flex gap-2 mb-6 overflow-x-auto pb-2">
        {dashboards.map((d) => (
          <button
            key={d.id}
            onClick={() => loadDashboard(d.id)}
            className={`px-4 py-2 rounded text-sm font-medium whitespace-nowrap transition-colors ${
              selected?.id === d.id
                ? 'bg-blue-600 text-white'
                : 'bg-gray-700 text-gray-300 hover:bg-gray-600'
            }`}
          >
            {d.name}
          </button>
        ))}
      </div>

      {loading && <p className="text-gray-400 text-center py-8">Loading...</p>}

      {!loading && selected && (
        <div>
          <div className="flex items-center gap-3 mb-4">
            <p className="text-sm text-gray-400">Add widget:</p>
            {(['assigned_to_me', 'activity_stream'] as const).map((wt) => (
              <button
                key={wt}
                onClick={() => handleAddWidget(wt)}
                className="flex items-center gap-1 bg-gray-700 text-gray-300 px-3 py-1.5 rounded text-xs hover:bg-gray-600 transition-colors"
              >
                <Plus className="w-3 h-3" />
                {widgetLabel(wt)}
              </button>
            ))}
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {selected.widgets?.map((w) => (
              <div key={w.id} className="bg-gray-800 rounded-lg p-4">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-sm font-semibold text-gray-300">{widgetLabel(w.widget_type)}</h3>
                  <button
                    onClick={() => handleRemoveWidget(w.id)}
                    className="text-gray-500 hover:text-red-400 transition-colors"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
                {renderWidget(widget)}
              </div>
            ))}
          </div>
        </div>
      )}

      {!loading && !selected && (
        <div className="text-center py-16">
          <LayoutDashboard className="w-12 h-12 text-gray-600 mx-auto mb-4" />
          <p className="text-gray-500">No dashboards yet. Create one to get started.</p>
        </div>
      )}
    </div>
  )
}
