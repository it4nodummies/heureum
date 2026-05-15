import { useState, useMemo } from 'react'
import { Plus, ArrowUp, ArrowDown, Minus, ChevronDown, ChevronUp, MoreHorizontal, Bookmark } from 'lucide-react'

interface Issue {
  id: string
  key: string
  title: string
  priority: string
  type: string
  story_points: number | null
  assignee: string | null
  sprint_id: string | null
}

interface Sprint {
  id: string
  name: string
  goal: string
  state: 'planning' | 'active' | 'completed'
  issues: Issue[]
  start_date: string | null
  end_date: string | null
}

const DEMO_BACKLOG: Issue[] = [
  { id: 'i1', key: 'OJ-1', title: 'Implement user authentication with OAuth2', priority: 'highest', type: 'story', story_points: 8, assignee: 'Alex Chen', sprint_id: null },
  { id: 'i2', key: 'OJ-2', title: 'Design database schema for issue tracking', priority: 'high', type: 'task', story_points: 5, assignee: 'Maria Silva', sprint_id: null },
  { id: 'i3', key: 'OJ-3', title: 'Set up monitoring and logging infrastructure', priority: 'medium', type: 'task', story_points: 3, assignee: null, sprint_id: null },
  { id: 'i4', key: 'OJ-4', title: 'Add email notification service', priority: 'medium', type: 'story', story_points: 5, assignee: 'James Wilson', sprint_id: null },
  { id: 'i5', key: 'OJ-5', title: 'Setup CI/CD pipeline with GitHub Actions', priority: 'high', type: 'task', story_points: 3, assignee: 'Alex Chen', sprint_id: null },
  { id: 'i6', key: 'OJ-6', title: 'Write comprehensive API documentation', priority: 'low', type: 'docs', story_points: 2, assignee: null, sprint_id: null },
  { id: 'i7', key: 'OJ-7', title: 'Implement drag and drop for sprint planning', priority: 'high', type: 'story', story_points: 8, assignee: 'Maria Silva', sprint_id: null },
  { id: 'i8', key: 'OJ-8', title: 'Add keyboard shortcuts for power users', priority: 'low', type: 'improvement', story_points: 3, assignee: null, sprint_id: null },
  { id: 'i9', key: 'OJ-9', title: 'Fix race condition in WebSocket handler', priority: 'highest', type: 'bug', story_points: 5, assignee: 'James Wilson', sprint_id: null },
  { id: 'i10', key: 'OJ-10', title: 'Optimize database query performance', priority: 'medium', type: 'improvement', story_points: 5, assignee: null, sprint_id: null },
]

function getTypeIcon(type: string) {
  switch (type) {
    case 'story': return <Bookmark className="w-4 h-4" style={{ color: 'var(--color-accent-green)' }} />
    case 'task': return <span className="w-4 h-4 flex items-center justify-center text-xs font-bold" style={{ color: 'var(--color-accent-blue)' }}>✓</span>
    case 'bug': return <span className="w-4 h-4 flex items-center justify-center text-xs font-bold" style={{ color: 'var(--color-accent-red)' }}>!</span>
    case 'epic': return <span className="w-4 h-4 flex items-center justify-center text-xs font-bold" style={{ color: 'var(--color-accent-purple)' }}>⚡</span>
    case 'docs': return <span className="w-4 h-4 flex items-center justify-center text-xs" style={{ color: 'var(--color-text-subtlest)' }}>📄</span>
    default: return <span className="w-4 h-4 flex items-center justify-center text-xs" style={{ color: 'var(--color-accent-blue)' }}>●</span>
  }
}

function PriorityIcon({ priority }: { priority: string }) {
  const color = (p: string) => `var(--color-priority-${p})`
  switch (priority) {
    case 'highest':
      return (
        <span className="inline-flex items-center gap-0.5">
          <ArrowUp size={12} color={color('highest')} fill={color('highest')} />
          <ArrowUp size={12} color={color('highest')} fill={color('highest')} className="-ml-1.5" />
        </span>
      )
    case 'high':
      return <ArrowUp size={14} color={color('high')} />
    case 'medium':
      return <Minus size={14} color={color('medium')} />
    case 'low':
      return <ArrowDown size={14} color={color('low')} />
    case 'lowest':
      return (
        <span className="inline-flex items-center gap-0.5">
          <ArrowDown size={12} color={color('lowest')} fill={color('lowest')} />
          <ArrowDown size={12} color={color('lowest')} fill={color('lowest')} className="-ml-1.5" />
        </span>
      )
    default:
      return <Minus size={14} color={color('medium')} />
  }
}

function getTimeRemaining(endDate: string): string {
  const now = new Date()
  const end = new Date(endDate)
  const diff = end.getTime() - now.getTime()
  if (diff <= 0) return 'Ended'
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
  if (days === 0) return 'Ends today'
  return `${days} day${days !== 1 ? 's' : ''} remaining`
}

export default function BacklogPage() {
  const [backlogIssues, setBacklogIssues] = useState<Issue[]>(DEMO_BACKLOG)
  const [sprints, setSprints] = useState<Sprint[]>([])
  const [selectedSprintId, setSelectedSprintId] = useState<string | null>(null)
  const [newIssueTitle, setNewIssueTitle] = useState('')
  const [showCreateSprint, setShowCreateSprint] = useState(false)
  const [createSprintName, setCreateSprintName] = useState('')
  const [createSprintGoal, setCreateSprintGoal] = useState('')

  const selectedSprint = useMemo(
    () => sprints.find(s => s.id === selectedSprintId) ?? null,
    [sprints, selectedSprintId],
  )

  function handleCreateIssue() {
    const title = newIssueTitle.trim()
    if (!title) return
    const num = backlogIssues.length + sprints.reduce((sum, s) => sum + s.issues.length, 0) + 1
    const issue: Issue = {
      id: `i-new-${Date.now()}`,
      key: `OJ-${num}`,
      title,
      priority: 'medium',
      type: 'task',
      story_points: null,
      assignee: null,
      sprint_id: null,
    }
    setBacklogIssues(prev => [...prev, issue])
    setNewIssueTitle('')
  }

  function handleCreateSprint() {
    const name = createSprintName.trim()
    if (!name) return
    const sprint: Sprint = {
      id: `sprint-${Date.now()}`,
      name,
      goal: createSprintGoal.trim(),
      state: 'planning',
      issues: [],
      start_date: null,
      end_date: null,
    }
    setSprints(prev => [...prev, sprint])
    setSelectedSprintId(sprint.id)
    setCreateSprintName('')
    setCreateSprintGoal('')
    setShowCreateSprint(false)
  }

  function handleMoveToSprint(issue: Issue) {
    if (!selectedSprintId) return
    const targetSprint = sprints.find(s => s.id === selectedSprintId)
    if (!targetSprint) return
    setBacklogIssues(prev => prev.filter(i => i.id !== issue.id))
    setSprints(prev =>
      prev.map(s =>
        s.id === selectedSprintId
          ? { ...s, issues: [...s.issues, { ...issue, sprint_id: selectedSprintId }] }
          : s,
      ),
    )
  }

  function handleRemoveFromSprint(issue: Issue) {
    if (!selectedSprintId) return
    setSprints(prev =>
      prev.map(s =>
        s.id === selectedSprintId
          ? { ...s, issues: s.issues.filter(i => i.id !== issue.id) }
          : s,
      ),
    )
    setBacklogIssues(prev => [...prev, { ...issue, sprint_id: null }])
  }

  function handleStartSprint() {
    if (!selectedSprintId) return
    const startDate = new Date().toISOString().split('T')[0]
    const endDate = new Date(Date.now() + 14 * 24 * 60 * 60 * 1000).toISOString().split('T')[0]
    setSprints(prev =>
      prev.map(s =>
        s.id === selectedSprintId
          ? { ...s, state: 'active' as const, start_date: startDate, end_date: endDate }
          : s,
      ),
    )
  }

  function handleCompleteSprint() {
    if (!selectedSprintId) return
    setSprints(prev =>
      prev.map(s =>
        s.id === selectedSprintId
          ? { ...s, state: 'completed' as const }
          : s,
      ),
    )
  }

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between px-6 py-3 border-b border-default shrink-0">
        <div className="flex items-center gap-4">
          <h2 className="text-base font-semibold text-primary">Backlog</h2>
          <button
            onClick={() => setShowCreateSprint(true)}
            className="text-xs font-medium rounded-sm h-7 px-3 bg-accent-blue text-text-inverse hover:brightness-110 transition-colors"
          >
            Create sprint
          </button>
        </div>
      </div>

      {/* Split view */}
      <div className="flex-1 flex overflow-hidden">
        {/* Backlog list */}
        <div className="flex-1 flex flex-col">
          <div className="px-6 py-2 border-b border-default flex items-center gap-4 shrink-0">
            <span className="text-xs font-semibold text-subtlest uppercase tracking-wider">
              Backlog
            </span>
            <span className="text-xs text-subtlest bg-elevated rounded-full px-2 h-5 flex items-center font-medium">
              {backlogIssues.length}
            </span>
          </div>

          <div className="flex-1 overflow-auto">
            {backlogIssues.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-20 gap-2">
                <span className="text-sm text-subtlest">No issues in backlog</span>
                <span className="text-xs text-subtlest">
                  Use the quick-create below or plan issues into your sprint
                </span>
              </div>
            ) : (
              backlogIssues.map(issue => (
                <div
                  key={issue.id}
                  className="flex items-center gap-3 px-6 py-2 border-b border-default hover:bg-card-hover cursor-pointer transition-colors group"
                >
                  <input
                    type="checkbox"
                    className="w-4 h-4 rounded-sm border border-default bg-transparent cursor-pointer"
                    style={{ accentColor: 'var(--color-accent-blue)' }}
                  />
                  {getTypeIcon(issue.type)}
                  <span className="text-xs text-link w-[52px] font-medium shrink-0">{issue.key}</span>
                  <PriorityIcon priority={issue.priority} />
                  <span className="text-sm text-primary flex-1 truncate">{issue.title}</span>
                  {issue.story_points != null && (
                    <span className="text-xs text-subtlest bg-elevated rounded-full min-w-[20px] h-5 flex items-center justify-center px-1.5 font-medium shrink-0">
                      {issue.story_points}
                    </span>
                  )}
                  <span className="w-[72px] text-xs text-subtlest truncate shrink-0 text-right">
                    {issue.assignee ?? '—'}
                  </span>
                  <button
                    onClick={(e) => { e.stopPropagation(); handleMoveToSprint(issue) }}
                    disabled={!selectedSprintId}
                    className="opacity-0 group-hover:opacity-100 transition-opacity disabled:opacity-0 shrink-0"
                    title={selectedSprintId ? 'Add to sprint' : 'Select a sprint first'}
                  >
                    <ChevronDown size={14} className="text-subtlest hover:text-primary" />
                  </button>
                </div>
              ))
            )}
          </div>

          {/* Quick create */}
          <div className="px-6 py-2 border-t border-default flex items-center gap-3 shrink-0">
            <Plus className="w-4 h-4 text-subtlest shrink-0" />
            <input
              className="flex-1 bg-transparent text-sm text-primary outline-none placeholder:text-subtlest"
              placeholder="Create issue..."
              value={newIssueTitle}
              onChange={e => setNewIssueTitle(e.target.value)}
              onKeyDown={e => {
                if (e.key === 'Enter') handleCreateIssue()
              }}
            />
          </div>
        </div>

        {/* Sprint panel */}
        <div className="w-[340px] border-l border-default flex flex-col shrink-0">
          <div className="px-4 py-3 border-b border-default shrink-0">
            <div className="flex items-center justify-between mb-1">
              <h3 className="text-sm font-semibold text-primary">Sprint</h3>
              {sprints.length > 0 && (
                <button
                  onClick={() => setShowCreateSprint(true)}
                  className="text-xs text-link hover:underline"
                >
                  Create sprint
                </button>
              )}
            </div>
            {sprints.length > 0 && (
              <select
                value={selectedSprintId ?? ''}
                onChange={e => {
                  const id = e.target.value
                    setSelectedSprintId(id || null)
                }}
                className="w-full bg-input text-sm text-primary border border-default rounded-sm h-8 px-2 outline-none focus:border-focus appearance-none cursor-pointer"
              >
                <option value="">— Select sprint —</option>
                {sprints.map(s => (
                  <option key={s.id} value={s.id}>
                    {s.name} {s.state === 'active' ? '(Active)' : s.state === 'completed' ? '(Completed)' : '(Planning)'}
                  </option>
                ))}
              </select>
            )}
          </div>

          {selectedSprint ? (
            <div className="flex-1 flex flex-col overflow-hidden">
              <div className="px-4 py-3 border-b border-default shrink-0">
                <div className="text-sm font-semibold text-primary">{selectedSprint.name}</div>
                {selectedSprint.goal && (
                  <div className="text-xs text-secondary mt-1 italic">
                    &ldquo;{selectedSprint.goal}&rdquo;
                  </div>
                )}
                {(selectedSprint.start_date && selectedSprint.end_date) && (
                  <div className="text-xs text-subtlest mt-1.5 flex items-center gap-1.5">
                    <span>{selectedSprint.start_date}</span>
                    <span>–</span>
                    <span>{selectedSprint.end_date}</span>
                    <span className="text-xs" style={{ color: 'var(--color-accent-yellow)' }}>
                      · {getTimeRemaining(selectedSprint.end_date)}
                    </span>
                  </div>
                )}
                <div className="flex items-center gap-2 mt-3">
                  {selectedSprint.state === 'planning' && (
                    <button
                      onClick={handleStartSprint}
                      disabled={selectedSprint.issues.length === 0}
                      className="text-xs font-medium rounded-sm h-7 px-3 bg-accent-blue text-text-inverse hover:brightness-110 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                    >
                      Start sprint
                    </button>
                  )}
                  {selectedSprint.state === 'active' && (
                    <button
                      onClick={handleCompleteSprint}
                      className="text-xs font-medium rounded-sm h-7 px-3 bg-accent-green text-text-inverse hover:brightness-110 transition-colors"
                    >
                      Complete sprint
                    </button>
                  )}
                  {selectedSprint.state === 'completed' && (
                    <span className="text-xs font-medium text-accent-green">
                      Sprint completed
                    </span>
                  )}
                  <button className="text-xs p-1 text-subtlest hover:text-primary">
                    <MoreHorizontal size={14} />
                  </button>
                </div>
              </div>

              <div className="flex-1 overflow-auto px-3 py-2">
                {selectedSprint.issues.length === 0 ? (
                  <div className="flex flex-col items-center justify-center py-16 gap-2 text-xs text-subtlest">
                    <span>No issues in this sprint yet</span>
                    <span>
                      {selectedSprint.state !== 'completed'
                        ? 'Click the arrows on backlog issues to add them here'
                        : 'This sprint has been completed'}
                    </span>
                  </div>
                ) : (
                  selectedSprint.issues.map(issue => (
                    <div
                      key={issue.id}
                      className="bg-card rounded-sm p-2.5 mb-1.5 hover:bg-card-hover cursor-pointer transition-colors group"
                    >
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-subtlest font-medium shrink-0">{issue.key}</span>
                        <PriorityIcon priority={issue.priority} />
                        <span className="text-sm text-primary truncate flex-1">{issue.title}</span>
                        {selectedSprint.state !== 'completed' && (
                          <button
                            onClick={(e) => { e.stopPropagation(); handleRemoveFromSprint(issue) }}
                            className="opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
                            title="Remove from sprint"
                          >
                            <ChevronUp size={14} className="text-subtlest hover:text-primary" />
                          </button>
                        )}
                      </div>
                      <div className="flex items-center gap-3 mt-1.5">
                        {issue.story_points != null && (
                          <span className="text-xs text-subtlest bg-elevated rounded-full min-w-[20px] h-4 flex items-center justify-center px-1.5">
                            {issue.story_points} sp
                          </span>
                        )}
                        <span className="text-xs text-subtlest">
                          {issue.assignee ?? 'Unassigned'}
                        </span>
                      </div>
                    </div>
                  ))
                )}
              </div>

              {selectedSprint.state !== 'completed' && (
                <div className="px-4 py-3 border-t border-default text-xs text-subtlest shrink-0 flex items-center justify-between">
                  <span>{selectedSprint.issues.length} issues</span>
                  {selectedSprint.issues.length > 0 && (
                    <span>
                      {selectedSprint.issues.reduce((sum, i) => sum + (i.story_points ?? 0), 0)} story points
                    </span>
                  )}
                </div>
              )}
            </div>
          ) : (
            <div className="flex-1 flex flex-col items-center justify-center text-xs text-subtlest px-6 text-center gap-2">
              {sprints.length === 0 ? (
                <>
                  <span className="text-sm">No sprints yet</span>
                  <span>Create a sprint to start planning your work</span>
                  <button
                    onClick={() => setShowCreateSprint(true)}
                    className="mt-2 text-xs font-medium rounded-sm h-7 px-3 bg-accent-blue text-text-inverse hover:brightness-110 transition-colors"
                  >
                    Create sprint
                  </button>
                </>
              ) : (
                <span>Select a sprint to view and manage its issues</span>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Create sprint dialog */}
      {showCreateSprint && (
        <>
          <div
            className="fixed inset-0"
            style={{ backgroundColor: 'rgba(0,0,0,0.5)' }}
            onClick={() => {
              setShowCreateSprint(false)
              setCreateSprintName('')
              setCreateSprintGoal('')
            }}
          />
          <div className="fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[400px] bg-card rounded-md border border-default shadow-xl p-6 z-50">
            <h3 className="text-base font-semibold text-primary mb-4">Create sprint</h3>
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-secondary mb-1">
                  Sprint name <span className="text-accent-red">*</span>
                </label>
                <input
                  className="w-full bg-input text-sm text-primary border border-default rounded-sm h-9 px-3 outline-none focus:border-focus placeholder:text-subtlest"
                  placeholder="e.g. Sprint 1"
                  value={createSprintName}
                  onChange={e => setCreateSprintName(e.target.value)}
                  onKeyDown={e => {
                    if (e.key === 'Enter' && createSprintName.trim()) handleCreateSprint()
                    if (e.key === 'Escape') {
                      setShowCreateSprint(false)
                      setCreateSprintName('')
                      setCreateSprintGoal('')
                    }
                  }}
                  autoFocus
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-secondary mb-1">
                  Sprint goal
                </label>
                <textarea
                  className="w-full bg-input text-sm text-primary border border-default rounded-sm h-16 px-3 py-2 outline-none focus:border-focus placeholder:text-subtlest resize-none"
                  placeholder="Describe the goal of this sprint..."
                  value={createSprintGoal}
                  onChange={e => setCreateSprintGoal(e.target.value)}
                />
              </div>
            </div>
            <div className="flex items-center justify-end gap-2 mt-5">
              <button
                onClick={() => {
                  setShowCreateSprint(false)
                  setCreateSprintName('')
                  setCreateSprintGoal('')
                }}
                className="text-sm text-secondary hover:text-primary h-8 px-3 rounded-sm transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleCreateSprint}
                disabled={!createSprintName.trim()}
                className="text-sm font-medium rounded-sm h-8 px-4 bg-accent-blue text-text-inverse hover:brightness-110 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
              >
                Create
              </button>
            </div>
          </div>
        </>
      )}
    </div>
  )
}
