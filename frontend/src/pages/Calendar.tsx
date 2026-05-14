import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'

interface CalendarIssue {
  id: string
  key: string
  title: string
  priority: string
  status: string
  due_date: string
  start_date: string | null
}

interface CalendarDay {
  date: string
  day: number
  issues: CalendarIssue[]
}

interface CalendarData {
  year: number
  month: number
  days: CalendarDay[]
  total_days: number
}

function priorityColor(p: string): string {
  switch (p) {
    case 'highest':
    case 'high':
      return 'bg-red-600'
    case 'medium':
      return 'bg-yellow-500'
    case 'low':
    case 'lowest':
      return 'bg-blue-500'
    default:
      return 'bg-gray-500'
  }
}

function getMonthName(m: number): string {
  return new Date(2020, m - 1).toLocaleString('default', { month: 'long' })
}

export default function CalendarPage({ projectKey = 'DEMO' }: { projectKey?: string }) {
  const now = new Date()
  const [year, setYear] = useState(now.getFullYear())
  const [month, setMonth] = useState(now.getMonth() + 1)

  const { data, isLoading } = useQuery<CalendarData>({
    queryKey: ['calendar', projectKey, year, month],
    queryFn: () =>
      fetch(`/api/v1/projects/${projectKey}/calendar?year=${year}&month=${month}`).then(r => r.json()),
  })

  const firstDay = new Date(year, month - 1, 1).getDay()
  const prevMonth = () => {
    if (month === 1) { setYear(y => y - 1); setMonth(12) }
    else setMonth(m => m - 1)
  }
  const nextMonth = () => {
    if (month === 12) { setYear(y => y + 1); setMonth(1) }
    else setMonth(m => m + 1)
  }

  if (isLoading) return <div className="p-6 text-gray-400">Loading calendar...</div>

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-2xl font-bold">Calendar</h1>
        <div className="flex items-center gap-4">
          <button onClick={prevMonth} className="px-3 py-1 rounded bg-gray-700 hover:bg-gray-600">&larr;</button>
          <span className="text-lg font-medium">{getMonthName(month)} {year}</span>
          <button onClick={nextMonth} className="px-3 py-1 rounded bg-gray-700 hover:bg-gray-600">&rarr;</button>
        </div>
      </div>

      <div className="grid grid-cols-7 gap-px bg-gray-700 rounded-lg overflow-hidden">
        {['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'].map(d => (
          <div key={d} className="bg-gray-800 text-center text-sm font-medium py-2 text-gray-300">{d}</div>
        ))}
        {Array.from({ length: firstDay }).map((_, i) => (
          <div key={`empty-${i}`} className="bg-gray-900 min-h-[80px]" />
        ))}
        {data?.days.map(day => (
          <div key={day.day} className="bg-gray-900 min-h-[80px] p-1">
            <div className="text-xs text-gray-400 mb-1">{day.day}</div>
            <div className="flex flex-wrap gap-1">
              {day.issues.map(iss => (
                <div
                  key={iss.id}
                  title={`${iss.key}: ${iss.title}`}
                  className={`w-2 h-2 rounded-full ${priorityColor(iss.priority)}`}
                />
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
