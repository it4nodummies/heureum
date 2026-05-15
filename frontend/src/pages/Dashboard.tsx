import { useState } from 'react'
import { Plus, ArrowUp, ArrowDown, Minus } from 'lucide-react'

interface Issue {
  id: string
  key: string
  title: string
  priority: string
  status: string
}

const PriorityIcon = ({ priority }: { priority: string }) => {
  const c = (p: string) => `var(--color-priority-${p})`
  if (priority === 'highest') return <ArrowUp size={14} color={c('highest')} fill={c('highest')} />
  if (priority === 'high') return <ArrowUp size={14} color={c('high')} />
  if (priority === 'low') return <ArrowDown size={14} color={c('low')} />
  if (priority === 'lowest') return <ArrowDown size={14} color={c('lowest')} fill={c('lowest')} />
  return <Minus size={14} color={c('medium')} />
}

export default function DashboardPage() {
  const [assignedIssues] = useState<Issue[]>([
    { id: 'i1', key: 'OJ-3', title: 'Jira-like UI redesign', priority: 'highest', status: 'In Progress' },
    { id: 'i2', key: 'OJ-2', title: 'Configure OAuth providers', priority: 'medium', status: 'To Do' },
  ])

  const activities = [
    { id: 'a1', text: 'You created OJ-3', time: '2 hours ago' },
    { id: 'a2', text: 'Admin changed status of OJ-3 to In Progress', time: '1 hour ago' },
    { id: 'a3', text: 'You commented on OJ-1', time: '30 minutes ago' },
  ]

  return (
    <div className="h-full overflow-auto">
      <div className="px-6 py-3 border-b border-default flex items-center justify-between shrink-0">
        <h2 className="text-base font-semibold text-primary">Dashboard</h2>
        <button className="text-xs text-accent-blue hover:underline flex items-center gap-1">
          <Plus className="w-3.5 h-3.5" /> Add widget
        </button>
      </div>
      <div className="p-6 grid grid-cols-2 gap-6">
        <div className="bg-card rounded-sm border border-default">
          <div className="px-4 py-3 border-b border-default">
            <h3 className="text-sm font-semibold text-primary">Assigned to Me</h3>
          </div>
          <div className="divide-y divide-default">
            {assignedIssues.map(issue => (
              <div key={issue.id} className="px-4 py-3 hover:bg-card-hover cursor-pointer transition-colors">
                <div className="flex items-center gap-2">
                  <span className="text-xs text-secondary font-medium">{issue.key}</span>
                  <PriorityIcon priority={issue.priority} />
                </div>
                <div className="text-sm text-primary mt-0.5">{issue.title}</div>
                <div className="text-xs text-subtlest mt-1">{issue.status}</div>
              </div>
            ))}
          </div>
        </div>

        <div className="bg-card rounded-sm border border-default">
          <div className="px-4 py-3 border-b border-default">
            <h3 className="text-sm font-semibold text-primary">Activity Stream</h3>
          </div>
          <div className="divide-y divide-default">
            {activities.map(a => (
              <div key={a.id} className="px-4 py-3 text-sm text-primary">
                <div>{a.text}</div>
                <div className="text-xs text-subtlest mt-1">{a.time}</div>
              </div>
            ))}
          </div>
        </div>

        <div className="bg-card rounded-sm border border-default">
          <div className="px-4 py-3 border-b border-default">
            <h3 className="text-sm font-semibold text-primary">Projects</h3>
          </div>
          <div className="px-4 py-3">
            <div className="flex items-center gap-3 hover:bg-card-hover rounded-sm p-2 -m-2 cursor-pointer transition-colors">
              <div className="w-8 h-8 rounded-sm bg-accent-blue flex items-center justify-center text-white text-xs font-bold">OJ</div>
              <div>
                <div className="text-sm text-primary font-medium">Open Jira</div>
                <div className="text-xs text-subtlest">Software project</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
