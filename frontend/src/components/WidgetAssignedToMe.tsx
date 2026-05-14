interface Issue {
  id: string
  key: string
  title: string
  priority: string
  project_id: string
  project_name: string
  updated_at: string
  status_name: string
}

interface Props {
  issues: Issue[]
}

const priorityColors: Record<string, string> = {
  highest: 'bg-red-600',
  high: 'bg-orange-500',
  medium: 'bg-yellow-500',
  low: 'bg-green-500',
  lowest: 'bg-gray-500',
}

export default function WidgetAssignedToMe({ issues }: Props) {
  if (issues.length === 0) {
    return <p className="text-gray-400 text-sm">No issues assigned to you.</p>
  }

  return (
    <div className="space-y-2 max-h-96 overflow-y-auto">
      {issues.map((iss) => (
        <div key={iss.id} className="flex items-center gap-3 p-2 rounded bg-gray-700/50 hover:bg-gray-700 transition-colors">
          <span className={`w-2 h-2 rounded-full flex-shrink-0 ${priorityColors[iss.priority] || 'bg-gray-500'}`} />
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <span className="text-xs font-mono text-blue-400">{iss.key}</span>
              <span className="text-xs text-gray-400">{iss.status_name}</span>
            </div>
            <p className="text-sm text-gray-200 truncate">{iss.title}</p>
            <p className="text-xs text-gray-500">{iss.project_name}</p>
          </div>
        </div>
      ))}
    </div>
  )
}
