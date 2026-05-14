import { useState, useEffect, useCallback } from 'react'
import BurndownChart from '../components/BurndownChart'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts'
import { BarChart3, Activity, TrendingUp, Layers } from 'lucide-react'

const API = 'http://localhost:8080/api/v1'

function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem('token')
  return token ? { Authorization: `Bearer ${token}` } : {}
}

interface BurndownData {
  labels: string[]
  ideal: number[]
  actual: number[]
}

interface ProjectSummary {
  issue_count_by_status: Record<string, number>
  created_last_7_days: number
  updated_last_7_days: number
  completed_last_7_days: number
}

interface SprintVelocity {
  sprint_id: string
  sprint_name: string
  completed: number
  total_planned: number
}

interface VelocityData {
  sprints: SprintVelocity[]
}

interface CFDData {
  categories: string[]
  dates: string[]
  data: Record<string, number[]>
}

interface Project {
  id: string
  key: string
  name: string
}

interface Sprint {
  id: string
  name: string
  state: string
}

export default function ReportsPage() {
  const [projects, setProjects] = useState<Project[]>([])
  const [selectedProject, setSelectedProject] = useState('')
  const [sprints, setSprints] = useState<Sprint[]>([])
  const [selectedSprint, setSelectedSprint] = useState('')
  const [burndown, setBurndown] = useState<BurndownData | null>(null)
  const [burnup, setBurnup] = useState<BurndownData | null>(null)
  const [velocity, setVelocity] = useState<VelocityData | null>(null)
  const [summary, setSummary] = useState<ProjectSummary | null>(null)
  const [cfd, setCfd] = useState<CFDData | null>(null)
  const [tab, setTab] = useState<'burndown' | 'velocity' | 'burnup' | 'cfd'>('burndown')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    fetch(`${API}/projects`, { headers: getAuthHeaders() })
      .then((r) => r.json())
      .then((data: Project[] | { error: string }) => {
        if (!Array.isArray(data)) return
        setProjects(data)
        if (data.length > 0) setSelectedProject(data[0].key)
      })
      .catch(() => {})
  }, [])

  useEffect(() => {
    if (!selectedProject) return
    fetch(`${API}/projects/${selectedProject}/sprints`, { headers: getAuthHeaders() })
      .then((r) => r.json())
      .then((data: Sprint[] | { error: string }) => {
        if (!Array.isArray(data)) return
        setSprints(data)
        const active = data.find((s) => s.state === 'active')
        if (active) setSelectedSprint(active.id)
        else if (data.length > 0) setSelectedSprint(data[0].id)
      })
      .catch(() => {})
  }, [selectedProject])

  const fetchReports = useCallback(async () => {
    if (!selectedProject || !selectedSprint) return
    setLoading(true)
    try {
      const headers = getAuthHeaders()
      const projKey = selectedProject
      const sprintId = selectedSprint

      const [burstResp, velResp, summaryResp, burnupResp, cfdResp] = await Promise.all([
        fetch(`${API}/projects/${projKey}/reports/burndown?sprintId=${sprintId}`, { headers }),
        fetch(`${API}/projects/${projKey}/reports/velocity`, { headers }),
        fetch(`${API}/projects/${projKey}/summary`, { headers }),
        fetch(`${API}/projects/${projKey}/reports/burnup?sprintId=${sprintId}`, { headers }),
        fetch(`${API}/projects/${projKey}/reports/cfd`, { headers }),
      ])

      const [bd, vel, s, bup, cf] = await Promise.all([
        burstResp.json(),
        velResp.json(),
        summaryResp.json(),
        burnupResp.json(),
        cfdResp.json(),
      ])

      if (!bd.error) setBurndown(bd)
      if (!vel.error) setVelocity(vel)
      if (!s.error) setSummary(s)
      if (!bup.error) setBurnup(bup)
      if (!cf.error) setCfd(cf)
    } finally {
      setLoading(false)
    }
  }, [selectedProject, selectedSprint])

  useEffect(() => {
    fetchReports()
  }, [fetchReports])

  const statusEntries = summary ? Object.entries(summary.issue_count_by_status) : []

  const cfdChartData = cfd && cfd.dates.length > 0
    ? cfd.dates.map((date, i) => {
        const row: Record<string, string | number> = { date }
        cfd.categories.forEach((cat) => {
          row[cat] = (cfd.data[cat] ?? [])[i] ?? 0
        })
        return row
      })
    : []

  const velocityChartData = velocity
    ? velocity.sprints.map((s) => ({
        name: s.sprint_name,
        completed: s.completed,
        planned: s.total_planned,
      }))
    : []

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-white">Reports</h1>
        <div className="flex gap-3">
          <select
            value={selectedProject}
            onChange={(e) => setSelectedProject(e.target.value)}
            className="bg-gray-700 text-gray-200 rounded px-3 py-1.5 text-sm border border-gray-600"
          >
            {projects.map((p) => (
              <option key={p.id} value={p.key}>{p.name}</option>
            ))}
          </select>
          <select
            value={selectedSprint}
            onChange={(e) => setSelectedSprint(e.target.value)}
            className="bg-gray-700 text-gray-200 rounded px-3 py-1.5 text-sm border border-gray-600"
          >
            {sprints.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name} {s.state === 'active' ? '(Active)' : s.state === 'future' ? '(Future)' : '(Closed)'}
              </option>
            ))}
          </select>
        </div>
      </div>

      {summary && (
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
          <div className="bg-gray-800 rounded-lg p-4 flex items-center gap-3">
            <Activity className="w-5 h-5 text-blue-400" />
            <div>
              <p className="text-xs text-gray-400">Created (7d)</p>
              <p className="text-xl font-bold text-white">{summary.created_last_7_days}</p>
            </div>
          </div>
          <div className="bg-gray-800 rounded-lg p-4 flex items-center gap-3">
            <TrendingUp className="w-5 h-5 text-green-400" />
            <div>
              <p className="text-xs text-gray-400">Completed (7d)</p>
              <p className="text-xl font-bold text-white">{summary.completed_last_7_days}</p>
            </div>
          </div>
          <div className="bg-gray-800 rounded-lg p-4 flex items-center gap-3">
            <BarChart3 className="w-5 h-5 text-yellow-400" />
            <div>
              <p className="text-xs text-gray-400">Updated (7d)</p>
              <p className="text-xl font-bold text-white">{summary.updated_last_7_days}</p>
            </div>
          </div>
          <div className="bg-gray-800 rounded-lg p-4 flex items-center gap-3">
            <Layers className="w-5 h-5 text-purple-400" />
            <div>
              <p className="text-xs text-gray-400">Active Sprint</p>
              <p className="text-xl font-bold text-white">
                {sprints.find((s) => s.state === 'active')?.name || 'None'}
              </p>
            </div>
          </div>
        </div>
      )}

      <div className="flex gap-2 mb-4">
        {(['burndown', 'burnup', 'velocity', 'cfd'] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-2 rounded text-sm font-medium transition-colors ${
              tab === t
                ? 'bg-blue-600 text-white'
                : 'bg-gray-700 text-gray-300 hover:bg-gray-600'
            }`}
          >
            {t.charAt(0).toUpperCase() + t.slice(1)}
          </button>
        ))}
      </div>

      {loading && <p className="text-gray-400 text-center py-8">Loading reports...</p>}

      {!loading && tab === 'burndown' && burndown && (
        <BurndownChart
          labels={burndown.labels}
          ideal={burndown.ideal}
          actual={burndown.actual}
          title="Burndown Chart"
          lineNames={['Ideal Burn', 'Remaining']}
        />
      )}

      {!loading && tab === 'burnup' && burnup && (
        <BurndownChart
          labels={burnup.labels}
          ideal={burnup.ideal}
          actual={burnup.actual}
          title="Burnup Chart"
          lineNames={['Total Scope', 'Completed']}
        />
      )}

      {!loading && tab === 'velocity' && velocityChartData.length > 0 && (
        <div className="bg-gray-800 rounded-lg p-4">
          <h3 className="text-lg font-semibold mb-4 text-gray-200">Velocity Report</h3>
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={velocityChartData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
              <XAxis dataKey="name" stroke="#9CA3AF" fontSize={12} />
              <YAxis stroke="#9CA3AF" fontSize={12} />
              <Tooltip
                contentStyle={{ backgroundColor: '#1F2937', border: '1px solid #374151', borderRadius: '6px' }}
                labelStyle={{ color: '#F9FAFB' }}
              />
              <Legend />
              <Bar dataKey="completed" fill="#22C55E" name="Completed" />
              <Bar dataKey="planned" fill="#374151" name="Planned" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}

      {!loading && tab === 'cfd' && cfdChartData.length > 0 && (
        <div className="bg-gray-800 rounded-lg p-4">
          <h3 className="text-lg font-semibold mb-4 text-gray-200">Cumulative Flow Diagram</h3>
          <ResponsiveContainer width="100%" height={300}>
            <BarChart data={cfdChartData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
              <XAxis dataKey="date" stroke="#9CA3AF" fontSize={12} />
              <YAxis stroke="#9CA3AF" fontSize={12} />
              <Tooltip
                contentStyle={{ backgroundColor: '#1F2937', border: '1px solid #374151', borderRadius: '6px' }}
                labelStyle={{ color: '#F9FAFB' }}
              />
              <Legend />
              <Bar dataKey="todo" stackId="a" fill="#6B7280" name="To Do" />
              <Bar dataKey="inprogress" stackId="a" fill="#3B82F6" name="In Progress" />
              <Bar dataKey="done" stackId="a" fill="#22C55E" name="Done" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}

      {!loading && summary && statusEntries.length > 0 && (
        <div className="bg-gray-800 rounded-lg p-4 mt-6">
          <h3 className="text-lg font-semibold mb-4 text-gray-200">Issue Status Breakdown</h3>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            {statusEntries.map(([status, count]) => (
              <div key={status} className="bg-gray-700/50 rounded p-3 text-center">
                <p className="text-lg font-bold text-white">{count}</p>
                <p className="text-xs text-gray-400">{status}</p>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
