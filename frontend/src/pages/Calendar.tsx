import { useState } from 'react'
import { ChevronLeft, ChevronRight } from 'lucide-react'

export default function Calendar() {
  const [currentMonth, setCurrentMonth] = useState(new Date(2026, 4))
  const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
  const daysInMonth = new Date(currentMonth.getFullYear(), currentMonth.getMonth() + 1, 0).getDate()
  const startDay = new Date(currentMonth.getFullYear(), currentMonth.getMonth(), 1).getDay()

  const issues = [
    { key: 'OJ-1', title: 'Setup K8s', date: '2026-05-05', priority: 'high' },
    { key: 'OJ-3', title: 'UI Redesign', date: '2026-05-15', priority: 'highest' },
    { key: 'OJ-5', title: 'CI/CD Pipeline', date: '2026-05-22', priority: 'medium' },
  ]

  function getIssuesForDay(day: number) {
    const dateStr = `2026-05-${String(day).padStart(2, '0')}`
    return issues.filter(i => i.date === dateStr)
  }

  const priorityColors: Record<string, string> = {
    highest: 'var(--color-priority-highest)',
    high: 'var(--color-priority-high)',
    medium: 'var(--color-priority-medium)',
    low: 'var(--color-priority-low)',
    lowest: 'var(--color-priority-lowest)',
  }

  const prevMonth = () => {
    if (currentMonth.getMonth() === 0) {
      setCurrentMonth(new Date(currentMonth.getFullYear() - 1, 11))
    } else {
      setCurrentMonth(new Date(currentMonth.getFullYear(), currentMonth.getMonth() - 1))
    }
  }

  const nextMonth = () => {
    if (currentMonth.getMonth() === 11) {
      setCurrentMonth(new Date(currentMonth.getFullYear() + 1, 0))
    } else {
      setCurrentMonth(new Date(currentMonth.getFullYear(), currentMonth.getMonth() + 1))
    }
  }

  return (
    <div className="h-full flex flex-col">
      <div className="px-6 py-3 border-b border-default flex items-center gap-4 shrink-0">
        <h2 className="text-base font-semibold text-primary">Calendar</h2>
        <div className="flex items-center gap-2 ml-auto">
          <button className="p-1 text-secondary hover:text-primary transition-colors" onClick={prevMonth}>
            <ChevronLeft className="w-4 h-4" />
          </button>
          <span className="text-sm text-primary font-medium w-32 text-center">
            {months[currentMonth.getMonth()]} {currentMonth.getFullYear()}
          </span>
          <button className="p-1 text-secondary hover:text-primary transition-colors" onClick={nextMonth}>
            <ChevronRight className="w-4 h-4" />
          </button>
        </div>
      </div>

      <div className="flex-1 flex flex-col">
        <div className="grid grid-cols-7 border-b border-default">
          {['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'].map(d => (
            <div key={d} className="px-3 py-2 text-xs text-subtlest font-semibold uppercase text-center">
              {d}
            </div>
          ))}
        </div>

        <div className="flex-1 grid grid-cols-7">
          {Array.from({ length: startDay }).map((_, i) => (
            <div key={`empty-${i}`} className="border-b border-r border-default bg-surface" />
          ))}
          {Array.from({ length: daysInMonth }).map((_, i) => {
            const day = i + 1
            const dayIssues = getIssuesForDay(day)
            const isToday = day === 15

            return (
              <div key={day} className={`border-b border-r border-default p-2 min-h-[80px] hover:bg-card-hover transition-colors cursor-pointer ${
                isToday ? 'bg-[#1C2B41]' : ''
              }`}>
                <div className={`text-xs font-medium mb-1 ${isToday ? 'text-accent-blue' : 'text-secondary'}`}>
                  {day}
                </div>
                <div className="space-y-0.5">
                  {dayIssues.map(issue => (
                    <div key={issue.key}
                      className="text-xs bg-card rounded-sm px-1.5 py-0.5 truncate border-l-2"
                      style={{ borderLeftColor: priorityColors[issue.priority] || 'var(--color-text-subtlest)' }}>
                      <span className="text-secondary font-medium">{issue.key}</span>{' '}
                      <span className="text-primary">{issue.title}</span>
                    </div>
                  ))}
                </div>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
