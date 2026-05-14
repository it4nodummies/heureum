interface ActivityItem {
  id: string
  issue_id: string
  issue_key: string
  issue_title: string
  actor_name: string
  field_name: string
  old_value: string
  new_value: string
  created_at: string
}

interface Props {
  items: ActivityItem[]
}

function fieldLabel(field: string): string {
  const labels: Record<string, string> = {
    title: 'changed title',
    description: 'updated description',
    priority: 'changed priority',
    assignee: 'changed assignee',
    status: 'moved',
    story_points: 'changed story points',
    created: 'created',
    sprint_id: 'updated sprint',
  }
  return labels[field] || field
}

export default function WidgetActivityStream({ items }: Props) {
  if (items.length === 0) {
    return <p className="text-gray-400 text-sm">No recent activity.</p>
  }

  return (
    <div className="space-y-2 max-h-96 overflow-y-auto">
      {items.map((item) => (
        <div key={item.id} className="flex items-start gap-3 p-2 rounded bg-gray-700/50">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <span className="text-xs font-mono text-blue-400 flex-shrink-0">{item.issue_key}</span>
              <span className="text-xs text-gray-200 truncate">{item.issue_title}</span>
            </div>
            <p className="text-xs text-gray-400 mt-1">
              <span className="text-gray-300">{item.actor_name || 'System'}</span>{' '}
              {fieldLabel(item.field_name)}
              {item.field_name === 'status' && (
                <span> from <span className="text-gray-500">{item.old_value || 'none'}</span> to <span className="text-green-400">{item.new_value}</span></span>
              )}
              {item.field_name !== 'status' && item.field_name !== 'created' && item.new_value && (
                <span> to <span className="text-gray-300">{item.new_value}</span></span>
              )}
            </p>
            <p className="text-xs text-gray-600 mt-0.5">
              {new Date(item.created_at).toLocaleString()}
            </p>
          </div>
        </div>
      ))}
    </div>
  )
}
