import { FolderKanban, Layout, BarChart3, ListTodo, Settings, CalendarDays, GitBranch } from 'lucide-react'

interface Props {
  activePage: string
  onNavigate: (page: string) => void
}

export default function Sidebar({ activePage, onNavigate }: Props) {
  const navItems = [
    { id: 'board', label: 'Board', icon: Layout },
    { id: 'backlog', label: 'Backlog', icon: ListTodo },
    { id: 'reports', label: 'Reports', icon: BarChart3 },
    { id: 'issues', label: 'Issues', icon: ListTodo },
    { id: 'dashboard', label: 'Dashboard', icon: Layout },
    { id: 'timeline', label: 'Timeline', icon: GitBranch },
    { id: 'calendar', label: 'Calendar', icon: CalendarDays },
  ]

  return (
    <div className="w-60 bg-sidebar border-r border-default flex flex-col shrink-0">
      <div className="p-3 border-b border-default">
        <div className="flex items-center gap-2 text-sm font-medium text-primary px-2 py-1.5 rounded-sm hover:bg-card cursor-pointer">
          <FolderKanban className="w-4 h-4 text-secondary" />
          <span>Open Jira</span>
        </div>
      </div>
      <div className="py-2 flex-1">
        {navItems.map(item => (
          <button
            key={item.id}
            onClick={() => onNavigate(item.id)}
            className={`w-full flex items-center gap-3 px-4 py-2 text-sm transition-colors
              ${activePage === item.id
                ? 'bg-[#1C2B41] text-accent-blue border-l-[3px] border-accent-blue'
                : 'text-secondary hover:bg-card hover:text-primary border-l-[3px] border-transparent'
              }`}
          >
            <item.icon className="w-4 h-4" />
            <span>{item.label}</span>
          </button>
        ))}
      </div>
      <div className="border-t border-default p-2">
        <button className="w-full flex items-center gap-3 px-4 py-2 text-sm text-secondary hover:bg-card hover:text-primary rounded-sm transition-colors">
          <Settings className="w-4 h-4" />
          <span>Settings</span>
        </button>
      </div>
    </div>
  )
}
