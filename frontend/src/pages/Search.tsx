import { useState, useEffect, useCallback } from 'react'

interface Issue {
  id: string
  key: string
  title: string
  priority: string
  status_id: string | null
  assignee_id: string | null
  created_at: string
}

interface SavedFilter {
  id: string
  name: string
  jql: string
  is_shared: boolean
}

const API = 'http://localhost:8080/api/v1'

const PRIORITY_COLORS: Record<string, string> = {
  highest: 'bg-red-600',
  high: 'bg-orange-500',
  medium: 'bg-yellow-500',
  low: 'bg-green-500',
  lowest: 'bg-gray-400',
}

function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem('token') || ''
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  }
}

export default function SearchPage() {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<Issue[]>([])
  const [savedFilters, setSavedFilters] = useState<SavedFilter[]>([])
  const [loading, setLoading] = useState(false)

  const doSearch = useCallback(async (q: string) => {
    if (!q.trim()) {
      setResults([])
      return
    }
    setLoading(true)
    const res = await fetch(`${API}/search?q=${encodeURIComponent(q)}`, {
      headers: getAuthHeaders(),
    })
    if (res.ok) {
      setResults(await res.json())
    }
    setLoading(false)
  }, [])

  const fetchFilters = useCallback(async () => {
    const res = await fetch(`${API}/filters`, { headers: getAuthHeaders() })
    if (res.ok) {
      setSavedFilters(await res.json())
    }
  }, [])

  useEffect(() => {
    fetchFilters()
  }, [fetchFilters])

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    doSearch(query)
  }

  const applyFilter = (jql: string) => {
    setQuery(jql)
    doSearch(jql)
  }

  const chipFilters = [
    { label: 'Project', value: 'project=' },
    { label: 'Type', value: 'type=' },
    { label: 'Status', value: 'status=' },
    { label: 'Assignee', value: 'assignee=' },
    { label: 'Priority', value: 'priority=' },
  ]

  const addChip = (prefix: string) => {
    setQuery((prev) => (prev ? `${prev} ${prefix}` : prefix))
  }

  return (
    <div className="max-w-5xl mx-auto p-6">
      <h1 className="text-2xl font-bold mb-6 text-white">Search Issues</h1>

      <form onSubmit={handleSearch} className="mb-4">
        <div className="flex gap-2">
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search issues (e.g. project=PROJ status=Done)..."
            className="flex-1 bg-gray-700 text-white rounded-lg px-4 py-2 border border-gray-600 focus:border-blue-500 focus:outline-none"
          />
          <button
            type="submit"
            className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
          >
            Search
          </button>
        </div>
      </form>

      <div className="flex flex-wrap gap-2 mb-4">
        {chipFilters.map((chip) => (
          <button
            key={chip.label}
            onClick={() => addChip(chip.value)}
            className="px-3 py-1 text-sm bg-gray-700 text-gray-300 rounded-full hover:bg-gray-600 border border-gray-600"
          >
            {chip.label}
          </button>
        ))}
      </div>

      {savedFilters.length > 0 && (
        <div className="mb-6">
          <label className="text-sm text-gray-400 block mb-2">Saved Filters</label>
          <div className="flex flex-wrap gap-2">
            {savedFilters.map((f) => (
              <button
                key={f.id}
                onClick={() => applyFilter(f.jql)}
                className="px-3 py-1 text-sm bg-gray-700 text-blue-400 rounded-full hover:bg-gray-600 border border-gray-600"
              >
                {f.name}
              </button>
            ))}
          </div>
        </div>
      )}

      {loading && <p className="text-gray-400">Searching...</p>}

      {!loading && results.length === 0 && query && (
        <p className="text-gray-500">No results found.</p>
      )}

      {results.length > 0 && (
        <div className="space-y-3">
          <p className="text-sm text-gray-400">{results.length} result(s)</p>
          {results.map((issue) => (
            <div
              key={issue.id}
              className="bg-gray-800 rounded-lg p-4 hover:bg-gray-750 transition-colors"
            >
              <div className="flex items-start justify-between">
                <div>
                  <span className="text-xs text-gray-500">{issue.key}</span>
                  <h3 className="text-lg font-semibold text-white">{issue.title}</h3>
                </div>
                <span
                  className={`px-2 py-1 rounded text-xs font-semibold text-white ${
                    PRIORITY_COLORS[issue.priority] || 'bg-gray-500'
                  }`}
                >
                  {issue.priority}
                </span>
              </div>
              <div className="flex gap-4 mt-2 text-sm text-gray-400">
                <span>
                  Status:{' '}
                  <span className="text-white">{issue.status_id || 'None'}</span>
                </span>
                <span>
                  Assignee:{' '}
                  <span className="text-white">
                    {issue.assignee_id || 'Unassigned'}
                  </span>
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
