import { useState } from 'react'
import { ArrowUp, ArrowDown, Minus } from 'lucide-react'
import GitIntegration from '../components/GitIntegration'

interface Issue {
  id: string
  key: string
  title: string
  description: string
  priority: string
  status: string
  assignee?: string
  reporter: string
  story_points?: number
  sprint?: string
  labels?: string[]
  created_at: string
  updated_at?: string
}

interface Comment {
  id: string
  author: { name: string; avatar: string }
  body: string
  created_at: string
}

interface HistoryEntry {
  id: string
  actor: string
  field: string
  old: string
  new: string
  created_at: string
}

const PriorityIcon = ({ priority }: { priority: string }) => {
  const c = (p: string) => `var(--color-priority-${p})`
  if (priority === 'highest') return <ArrowUp size={14} color={c('highest')} fill={c('highest')} />
  if (priority === 'high') return <ArrowUp size={14} color={c('high')} />
  if (priority === 'low') return <ArrowDown size={14} color={c('low')} />
  if (priority === 'lowest') return <ArrowDown size={14} color={c('lowest')} fill={c('lowest')} />
  return <Minus size={14} color={c('medium')} />
}

export default function IssueDetail({ issueKey }: { issueKey: string; onBack?: () => void }) {
  const [issue, setIssue] = useState<Issue | null>(null)
  const [activeTab, setActiveTab] = useState<'comments' | 'history' | 'git'>('comments')
  const [comments, setComments] = useState<Comment[]>([])
  const [history, setHistory] = useState<HistoryEntry[]>([])
  const [commentText, setCommentText] = useState('')

  useState(() => {
    setIssue({
      id: 'i3',
      key: issueKey || 'OJ-3',
      title: 'Jira-like UI redesign',
      description: 'Refactor all pages with ADS tokens',
      priority: 'highest',
      status: 'In Progress',
      assignee: 'Admin',
      reporter: 'Admin',
      story_points: 8,
      created_at: '2026-05-15',
      labels: ['ui', 'frontend'],
    })
    setComments([
      {
        id: 'c1',
        author: { name: 'Admin', avatar: '' },
        body: 'Starting the redesign today.',
        created_at: '2026-05-15',
      },
    ])
    setHistory([
      {
        id: 'h1',
        actor: 'Admin',
        field: 'status',
        old: 'To Do',
        new: 'In Progress',
        created_at: '2026-05-15',
      },
      {
        id: 'h2',
        actor: 'Admin',
        field: 'created',
        old: '',
        new: issueKey || 'OJ-3',
        created_at: '2026-05-14',
      },
    ])
  })

  const projectKey = (issueKey || 'OJ-3').split('-')[0]

  if (!issue) return <div className="p-6 text-subtlest">Loading...</div>

  return (
    <div className="h-full flex flex-col">
      <div className="px-6 py-3 border-b border-default flex items-center gap-3 shrink-0">
        <span className="text-xs text-secondary">Projects / {projectKey} /</span>
        <span className="text-xs text-secondary font-medium">{issue.key}</span>
      </div>

      <div className="flex-1 flex overflow-hidden">
        <div className="flex-1 overflow-auto p-6">
          <h1 className="text-xl font-semibold text-primary mb-6">{issue.title}</h1>

          <div className="mb-8">
            <h3 className="text-sm font-semibold text-primary mb-2">Description</h3>
            <div className="text-sm text-primary bg-transparent border border-default rounded-sm p-3 min-h-[100px]">
              {issue.description || 'Add a description...'}
            </div>
          </div>

          <div>
            <div className="flex gap-0 border-b border-default mb-4">
              {(['comments', 'history', 'git'] as const).map(tab => (
                <button
                  key={tab}
                  onClick={() => setActiveTab(tab)}
                  className={`px-4 py-2 text-sm font-medium capitalize transition-colors border-b-2 -mb-[1px] ${
                    activeTab === tab
                      ? 'text-accent-blue border-accent-blue'
                      : 'text-secondary border-transparent hover:text-primary'
                  }`}
                >
                  {tab}
                </button>
              ))}
            </div>

            {activeTab === 'comments' && (
              <div className="space-y-4">
                {comments.map(c => (
                  <div key={c.id} className="flex gap-3">
                    <div className="w-8 h-8 rounded-full bg-card-hover flex items-center justify-center text-xs font-medium text-secondary shrink-0 mt-0.5">
                      {c.author?.name?.slice(0, 2).toUpperCase() || '??'}
                    </div>
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium text-primary">{c.author?.name}</span>
                        <span className="text-xs text-subtlest">{c.created_at}</span>
                      </div>
                      <div className="text-sm text-primary mt-1">{c.body}</div>
                    </div>
                  </div>
                ))}
                <div className="flex gap-3">
                  <div className="w-8 h-8 rounded-full bg-card-hover flex items-center justify-center text-xs font-medium text-secondary shrink-0 mt-1">
                    AM
                  </div>
                  <div className="flex-1">
                    <textarea
                      className="w-full bg-input border border-default rounded-sm p-3 text-sm text-primary outline-none focus:border-focus resize-none placeholder:text-subtlest"
                      placeholder="Add a comment..."
                      rows={2}
                      value={commentText}
                      onChange={e => setCommentText(e.target.value)}
                    />
                    <button className="mt-2 bg-accent-blue text-white text-sm font-medium rounded-sm h-8 px-4 hover:brightness-110">
                      Save
                    </button>
                  </div>
                </div>
              </div>
            )}

            {activeTab === 'history' && (
              <div className="relative pl-6">
                <div className="absolute left-[7px] top-2 bottom-2 w-0.5 bg-default" />
                {history.map(h => (
                  <div key={h.id} className="relative pb-5">
                    <div className="absolute -left-[22px] top-1.5 w-2.5 h-2.5 rounded-full bg-default border-2 border-surface" />
                    <div className="text-sm">
                      <span className="font-medium text-primary">{h.actor}</span>
                      <span className="text-secondary"> changed </span>
                      <span className="text-primary">{h.field}</span>
                      {h.old && (
                        <>
                          <span className="text-secondary"> from </span>
                          <span className="text-primary line-through">{h.old}</span>
                        </>
                      )}
                      {h.new && (
                        <>
                          <span className="text-secondary"> to </span>
                          <span className="text-primary">{h.new}</span>
                        </>
                      )}
                    </div>
                    <div className="text-xs text-subtlest mt-0.5">{h.created_at}</div>
                  </div>
                ))}
              </div>
            )}

            {activeTab === 'git' && (
              <GitIntegration projectKey={projectKey} issueKey={issueKey} />
            )}
          </div>
        </div>

        <div className="w-[280px] border-l border-default shrink-0 overflow-auto">
          <div className="p-4 space-y-4">
            <h3 className="text-xs font-semibold uppercase text-subtlest tracking-wide">Details</h3>
            {[
              { label: 'Status', value: issue.status },
              { label: 'Assignee', value: issue.assignee || 'Unassigned' },
              { label: 'Priority', value: issue.priority, icon: <PriorityIcon priority={issue.priority} /> },
              { label: 'Labels', value: issue.labels?.join(', ') || 'None' },
              { label: 'Sprint', value: issue.sprint || 'Backlog' },
              { label: 'Story Points', value: issue.story_points?.toString() || '-' },
              { label: 'Reporter', value: issue.reporter },
            ].map(field => (
              <div key={field.label}>
                <div className="text-xs text-subtlest font-medium mb-0.5">{field.label}</div>
                <div className="flex items-center gap-2 text-sm text-primary hover:bg-card-hover rounded-sm px-2 py-1 -mx-2 cursor-pointer transition-colors">
                  {field.icon}
                  <span>{field.value}</span>
                </div>
              </div>
            ))}
            <div className="border-t border-default pt-3">
              <div className="text-xs text-subtlest">Created</div>
              <div className="text-sm text-primary">{issue.created_at}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
