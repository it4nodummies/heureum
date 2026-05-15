import { useState, useEffect } from 'react'
import { DndContext, closestCorners } from '@dnd-kit/core'
import type { DragEndEvent } from '@dnd-kit/core'
import { SortableContext, verticalListSortingStrategy, useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { GripVertical, ArrowUp, ArrowDown, Minus } from 'lucide-react'

interface Issue {
  id: string
  key: string
  title: string
  priority: string
  type_id?: string
  assignee?: string
  story_points?: number
}

interface Column {
  id: string
  name: string
  category: string
  color: string
  issues: Issue[]
}

function PriorityIcon({ priority }: { priority: string }) {
  const colors: Record<string, string> = {
    highest: 'var(--color-priority-highest)',
    high: 'var(--color-priority-high)',
    medium: 'var(--color-priority-medium)',
    low: 'var(--color-priority-low)',
    lowest: 'var(--color-priority-lowest)',
  }
  const color = colors[priority] || 'var(--color-text-subtlest)'
  const size = 14

  if (priority === 'highest') return <ArrowUp size={size} color={color} fill={color} />
  if (priority === 'high') return <ArrowUp size={size} color={color} />
  if (priority === 'low') return <ArrowDown size={size} color={color} />
  if (priority === 'lowest') return <ArrowDown size={size} color={color} fill={color} />
  return <Minus size={size} color={color} />
}

function IssueCard({ issue, onNavigateIssue }: { issue: Issue; onNavigateIssue?: (key: string) => void }) {
  const { attributes, listeners, setNodeRef, transform, transition } = useSortable({ id: issue.id })
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  return (
    <div
      ref={setNodeRef}
      style={style}
      {...attributes}
      onClick={() => onNavigateIssue?.(issue.key)}
      className="bg-card rounded-sm p-3 shadow-sm hover:bg-card-hover cursor-pointer group transition-colors"
    >
      <div className="flex items-start gap-2">
        <button
          {...listeners}
          className="mt-0.5 opacity-0 group-hover:opacity-100 transition-opacity cursor-grab text-subtlest"
        >
          <GripVertical size={14} />
        </button>
        <div className="flex-1 min-w-0">
          <div className="text-xs text-secondary font-medium">{issue.key}</div>
          <div className="text-sm text-primary leading-snug mt-0.5">{issue.title}</div>
          <div className="flex items-center gap-2 mt-2">
            <PriorityIcon priority={issue.priority || 'medium'} />
            {issue.assignee && (
              <div
                className="w-5 h-5 rounded-full bg-[#2C333A] flex items-center justify-center text-[10px] font-medium text-secondary"
                title={issue.assignee}
              >
                {issue.assignee.slice(0, 2).toUpperCase()}
              </div>
            )}
            {issue.story_points != null && (
              <span className="text-xs text-subtlest bg-[#38414A] rounded-full px-1.5 h-4 flex items-center font-medium">
                {issue.story_points}
              </span>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function BoardColumn({ column, onNavigateIssue }: { column: Column; onNavigateIssue?: (key: string) => void }) {
  return (
    <div className="w-[280px] shrink-0 flex flex-col">
      <div className="flex items-center gap-2 px-2 py-3">
        <span className="w-2 h-2 rounded-full shrink-0" style={{ backgroundColor: column.color }} />
        <span className="text-xs font-semibold uppercase text-secondary tracking-wide">{column.name}</span>
        <span className="text-xs text-subtlest bg-[#38414A] rounded-full px-1.5 h-4 flex items-center font-medium ml-auto">
          {column.issues.length}
        </span>
      </div>
      <div className="flex-1 px-1">
        <SortableContext items={column.issues.map(i => i.id)} strategy={verticalListSortingStrategy}>
          <div className="flex flex-col gap-2 min-h-[100px]">
            {column.issues.map(issue => (
              <IssueCard key={issue.id} issue={issue} onNavigateIssue={onNavigateIssue} />
            ))}
            {column.issues.length === 0 && (
              <div className="text-xs text-subtlest text-center py-8 border border-dashed border-default rounded-sm">
                No issues
              </div>
            )}
          </div>
        </SortableContext>
      </div>
    </div>
  )
}

export default function BoardPage({ onNavigateIssue }: { onNavigateIssue?: (key: string) => void }) {
  const [columns, setColumns] = useState<Column[]>([])

  useEffect(() => {
    fetch('/api/v1/projects/OJ/board', {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    })
      .then(r => r.json())
      .then(data => setColumns(data.columns || []))
      .catch(() => {
        setColumns([
          { id: '1', name: 'To Do', category: 'todo', color: 'var(--color-status-todo)', issues: [
            { id: 'i1', key: 'OJ-1', title: 'Setup homelab deployment', priority: 'high', story_points: 3 },
            { id: 'i2', key: 'OJ-2', title: 'Configure OAuth providers', priority: 'medium', story_points: 5 },
          ]},
          { id: '2', name: 'In Progress', category: 'inprogress', color: 'var(--color-status-inprogress)', issues: [
            { id: 'i3', key: 'OJ-3', title: 'Jira-like UI redesign', priority: 'highest', story_points: 8 },
          ]},
          { id: '3', name: 'Done', category: 'done', color: 'var(--color-status-done)', issues: [] },
        ])
      })
  }, [])

  function handleDragEnd(event: DragEndEvent) {
    if (!event.over) return
  }

  return (
    <div className="h-full flex flex-col">
      <div className="px-6 py-3 border-b border-default flex items-center gap-4 shrink-0">
        <h2 className="text-base font-semibold text-primary">OJ Board</h2>
        <div className="flex items-center gap-2 ml-auto" />
      </div>
      <DndContext collisionDetection={closestCorners} onDragEnd={handleDragEnd}>
        <div className="flex-1 flex gap-0 overflow-x-auto p-4">
          {columns.map(col => (
            <BoardColumn key={col.id} column={col} onNavigateIssue={onNavigateIssue} />
          ))}
        </div>
      </DndContext>
    </div>
  )
}
