import { useState } from 'react'
import { LineChart, Line, BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, AreaChart, Area } from 'recharts'

export default function Reports() {
  const [activeReport, setActiveReport] = useState('burndown')

  const burndownData = [
    { day: 'Day 1', ideal: 20, actual: 20 },
    { day: 'Day 2', ideal: 16, actual: 18 },
    { day: 'Day 3', ideal: 12, actual: 14 },
    { day: 'Day 4', ideal: 8, actual: 10 },
    { day: 'Day 5', ideal: 4, actual: 6 },
    { day: 'Day 6', ideal: 0, actual: 2 },
  ]

  const velocityData = [
    { sprint: 'Sprint 1', planned: 20, completed: 18 },
    { sprint: 'Sprint 2', planned: 20, completed: 20 },
    { sprint: 'Sprint 3', planned: 25, completed: 22 },
  ]

  const cfdData = [
    { day: 'Day 1', todo: 15, inprogress: 3, done: 2 },
    { day: 'Day 3', todo: 12, inprogress: 4, done: 4 },
    { day: 'Day 5', todo: 8, inprogress: 3, done: 9 },
  ]

  const stats = [
    { label: 'Total Issues', value: 20, color: 'var(--color-text-secondary)' },
    { label: 'Completed', value: 9, color: 'var(--color-accent-green)' },
    { label: 'In Progress', value: 4, color: 'var(--color-accent-blue)' },
    { label: 'To Do', value: 7, color: 'var(--color-text-subtlest)' },
  ]

  return (
    <div className="h-full overflow-auto">
      <div className="px-6 py-3 border-b border-default flex items-center gap-4 shrink-0">
        <h2 className="text-base font-semibold text-primary">Reports</h2>
        <select className="bg-input border border-default rounded-sm px-3 py-1.5 text-sm text-primary outline-none">
          <option>OJ - Open Jira</option>
        </select>
      </div>

      <div className="p-6 max-w-4xl">
        <div className="grid grid-cols-4 gap-4 mb-6">
          {stats.map(s => (
            <div key={s.label} className="bg-card rounded-sm border border-default p-4">
              <div className="text-xs text-subtlest uppercase tracking-wide font-medium">{s.label}</div>
              <div className="text-2xl font-bold mt-1" style={{ color: s.color }}>{s.value}</div>
            </div>
          ))}
        </div>

        <div className="flex gap-0 mb-4 border-b border-default">
          {[
            { id: 'burndown', label: 'Burndown Chart' },
            { id: 'velocity', label: 'Velocity Chart' },
            { id: 'cfd', label: 'Cumulative Flow' },
          ].map(r => (
            <button key={r.id}
              onClick={() => setActiveReport(r.id)}
              className={`px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-[1px] ${
                r.id === activeReport
                  ? 'text-accent-blue border-accent-blue'
                  : 'text-secondary border-transparent hover:text-primary'
              }`}
            >
              {r.label}
            </button>
          ))}
        </div>

        <div className="bg-card rounded-sm border border-default p-6">
          <h3 className="text-sm font-semibold text-primary mb-4">
            {activeReport === 'burndown' && 'Sprint Burndown'}
            {activeReport === 'velocity' && 'Sprint Velocity'}
            {activeReport === 'cfd' && 'Cumulative Flow Diagram'}
          </h3>
          <ResponsiveContainer width="100%" height={350}>
            {activeReport === 'burndown' && (
              <LineChart data={burndownData}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border-default)" />
                <XAxis dataKey="day" tick={{ fill: 'var(--color-text-secondary)', fontSize: 12 }} />
                <YAxis tick={{ fill: 'var(--color-text-secondary)', fontSize: 12 }} />
                <Tooltip contentStyle={{ background: 'var(--color-card)', border: '1px solid var(--color-border-default)', borderRadius: '3px' }} itemStyle={{ color: 'var(--color-text-primary)' }} labelStyle={{ color: 'var(--color-text-secondary)' }} />
                <Line type="monotone" dataKey="ideal" stroke="var(--color-text-subtlest)" strokeDasharray="5 5" dot={false} />
                <Line type="monotone" dataKey="actual" stroke="var(--color-accent-blue)" strokeWidth={2} dot={{ fill: 'var(--color-accent-blue)', r: 3 }} />
              </LineChart>
            )}
            {activeReport === 'velocity' && (
              <BarChart data={velocityData}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border-default)" />
                <XAxis dataKey="sprint" tick={{ fill: 'var(--color-text-secondary)', fontSize: 12 }} />
                <YAxis tick={{ fill: 'var(--color-text-secondary)', fontSize: 12 }} />
                <Tooltip contentStyle={{ background: 'var(--color-card)', border: '1px solid var(--color-border-default)' }} itemStyle={{ color: 'var(--color-text-primary)' }} />
                <Bar dataKey="planned" fill="var(--color-text-subtlest)" radius={[2,2,0,0]} />
                <Bar dataKey="completed" fill="var(--color-accent-green)" radius={[2,2,0,0]} />
              </BarChart>
            )}
            {activeReport === 'cfd' && (
              <AreaChart data={cfdData}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border-default)" />
                <XAxis dataKey="day" tick={{ fill: 'var(--color-text-secondary)', fontSize: 12 }} />
                <YAxis tick={{ fill: 'var(--color-text-secondary)', fontSize: 12 }} />
                <Tooltip contentStyle={{ background: 'var(--color-card)', border: '1px solid var(--color-border-default)' }} itemStyle={{ color: 'var(--color-text-primary)' }} />
                <Area type="monotone" dataKey="todo" stackId="1" stroke="var(--color-text-subtlest)" fill="var(--color-text-subtlest)" fillOpacity={0.3} />
                <Area type="monotone" dataKey="inprogress" stackId="1" stroke="var(--color-accent-blue)" fill="var(--color-accent-blue)" fillOpacity={0.3} />
                <Area type="monotone" dataKey="done" stackId="1" stroke="var(--color-accent-green)" fill="var(--color-accent-green)" fillOpacity={0.3} />
              </AreaChart>
            )}
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  )
}
