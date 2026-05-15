import { useState } from 'react'

export default function Timeline() {
  const [zoom, setZoom] = useState<'weeks' | 'months' | 'quarters'>('weeks')

  const epics = [
    { id: 'e1', key: 'OJ-1', title: 'Infrastructure Setup', start: '2026-05-01', end: '2026-05-15', color: 'var(--color-accent-purple)' },
    { id: 'e2', key: 'OJ-2', title: 'Auth Module', start: '2026-05-10', end: '2026-05-25', color: 'var(--color-accent-blue)' },
    { id: 'e3', key: 'OJ-3', title: 'UI Redesign', start: '2026-05-15', end: '2026-05-30', color: 'var(--color-accent-green)' },
  ]

  const timelineStart = new Date('2026-05-01')
  const timelineEnd = new Date('2026-06-05')
  const totalDays = Math.ceil((timelineEnd.getTime() - timelineStart.getTime()) / (1000 * 60 * 60 * 24))

  function getBarStyle(start: string, end: string) {
    const s = new Date(start)
    const e = new Date(end)
    const left = ((s.getTime() - timelineStart.getTime()) / (totalDays * 86400000)) * 100
    const width = ((e.getTime() - s.getTime()) / (totalDays * 86400000)) * 100
    return { left: `${left}%`, width: `${Math.max(width, 2)}%` }
  }

  const monthLabels = ['May', 'Jun']
  const weekLabels = ['W17', 'W18', 'W19', 'W20', 'W21', 'W22']

  return (
    <div className="h-full flex flex-col">
      <div className="px-6 py-3 border-b border-default flex items-center gap-4 shrink-0">
        <h2 className="text-base font-semibold text-primary">Timeline</h2>
        <div className="flex bg-[#091E4214] rounded-sm">
          {(['weeks', 'months', 'quarters'] as const).map(z => (
            <button key={z}
              onClick={() => setZoom(z)}
              className={`text-xs font-medium px-3 py-1 rounded-sm capitalize transition-colors ${
                z === zoom ? 'bg-accent-blue text-white' : 'text-secondary hover:text-primary'
              }`}
            >
              {z}
            </button>
          ))}
        </div>
      </div>

      <div className="flex-1 flex overflow-hidden">
        <div className="w-[250px] border-r border-default shrink-0 overflow-auto">
          <div className="h-8 border-b border-default px-4 flex items-center">
            <span className="text-xs text-subtlest uppercase font-semibold">Epic</span>
          </div>
          {epics.map(epic => (
            <div key={epic.id} className="px-4 py-2.5 border-b border-default hover:bg-card-hover cursor-pointer transition-colors">
              <div className="flex items-center gap-2">
                <span className="w-2 h-2 rounded-full shrink-0" style={{ backgroundColor: epic.color }} />
                <span className="text-xs text-secondary font-medium">{epic.key}</span>
              </div>
              <div className="text-sm text-primary">{epic.title}</div>
            </div>
          ))}
        </div>

        <div className="flex-1 overflow-auto">
          <div className="h-8 border-b border-default flex">
            {(zoom === 'weeks' ? weekLabels : monthLabels).map((label, i) => (
              <div key={i} className="flex-1 px-2 flex items-center text-xs text-subtlest font-medium border-r border-default">
                {label}
              </div>
            ))}
          </div>
          <div>
            {epics.map(epic => (
              <div key={epic.id} className="relative h-[42px] border-b border-default hover:bg-card-hover transition-colors">
                <div className="absolute top-2 h-6 rounded-sm opacity-80 hover:opacity-100 cursor-pointer transition-opacity"
                  style={{
                    ...getBarStyle(epic.start, epic.end),
                    backgroundColor: epic.color,
                  }}>
                  <div className="px-2 text-xs text-white font-medium truncate leading-6">
                    {epic.title}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
