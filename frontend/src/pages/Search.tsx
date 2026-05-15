import { useState } from 'react'
import { Search, X, ArrowUp, ArrowDown, Minus } from 'lucide-react'

const PriorityIcon = ({ priority }: { priority: string }) => {
  const c = (p: string) => `var(--color-priority-${p})`
  if (priority === 'highest') return <ArrowUp size={14} color={c('highest')} fill={c('highest')} />
  if (priority === 'high') return <ArrowUp size={14} color={c('high')} />
  if (priority === 'low') return <ArrowDown size={14} color={c('low')} />
  if (priority === 'lowest') return <ArrowDown size={14} color={c('lowest')} fill={c('lowest')} />
  return <Minus size={14} color={c('medium')} />
}

interface Issue {
  id: string
  key: string
  title: string
  priority: string
  status: string
  assignee?: string
}

export default function SearchPage() {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<Issue[]>([])
  const [activeFilter, setActiveFilter] = useState('')

  const filters = ['All', 'My open issues', 'Reported by me', 'Done issues', 'Recently updated']

  function handleSearch(q: string) {
    setQuery(q)
    if (!q.trim()) {
      setResults([])
      return
    }
    setResults([
      { id: 'i1', key: 'OJ-1', title: 'Setup Kubernetes deployment', priority: 'high', status: 'Done' },
      { id: 'i3', key: 'OJ-3', title: 'Jira-like UI redesign', priority: 'highest', status: 'In Progress' },
    ])
  }

  return (
    <div className="h-full flex flex-col">
      <div className="px-6 py-3 border-b border-default shrink-0">
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 bg-input rounded-sm px-3 h-9 border border-default flex-1 max-w-lg">
            <Search className="w-4 h-4 text-subtlest" />
            <input
              className="bg-transparent text-sm text-primary outline-none flex-1 placeholder:text-subtlest"
              placeholder="Search issues..."
              value={query}
              onChange={e => handleSearch(e.target.value)}
            />
            {query && (
              <button onClick={() => handleSearch('')}>
                <X className="w-4 h-4 text-subtlest" />
              </button>
            )}
          </div>
        </div>
        <div className="flex gap-2 mt-3">
          {filters.map(f => (
            <button
              key={f}
              onClick={() => setActiveFilter(f === activeFilter ? '' : f)}
              className={`text-xs rounded-full px-3 py-1 transition-colors ${
                f === activeFilter
                  ? 'bg-accent-blue text-white'
                  : 'bg-card-hover text-secondary hover:brightness-110'
              }`}
            >
              {f}
            </button>
          ))}
        </div>
      </div>

      <div className="flex-1 overflow-auto p-6">
        {results.length === 0 && query && (
          <div className="text-sm text-subtlest text-center py-12">No issues match your search</div>
        )}
        {results.length === 0 && !query && (
          <div className="text-sm text-subtlest text-center py-12">Type to search for issues</div>
        )}
        <div className="space-y-2 max-w-2xl">
          {results.map(issue => (
            <div
              key={issue.id}
              className="bg-card rounded-sm border border-default p-3 hover:bg-card-hover cursor-pointer transition-colors"
            >
              <div className="flex items-center gap-2">
                <span className="text-xs text-secondary font-medium">{issue.key}</span>
                <PriorityIcon priority={issue.priority} />
                <span className="text-xs text-subtlest bg-card-hover rounded-full px-1.5 h-4 flex items-center font-medium">
                  {issue.status}
                </span>
              </div>
              <div className="text-sm text-primary mt-1">{issue.title}</div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
