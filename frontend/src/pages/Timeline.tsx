import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'

interface TimelineBar {
  id: string
  name: string
  type: string
  start_date: string | null
  end_date: string | null
  progress: number
  parent_id: string | null
  color: string
}

interface TimelineData {
  project_id: string
  zoom: string
  start_date: string
  end_date: string
  bars: TimelineBar[]
  headers: string[]
}

export default function TimelinePage({ projectKey = 'DEMO' }: { projectKey?: string }) {
  const [zoom, setZoom] = useState<'weeks' | 'months' | 'quarters'>('months')

  const { data, isLoading } = useQuery<TimelineData>({
    queryKey: ['timeline', projectKey, zoom],
    queryFn: () => fetch(`/api/v1/projects/${projectKey}/timeline?zoom=${zoom}`).then(r => r.json()),
  })

  if (isLoading) return <div className="p-6 text-gray-400">Loading timeline...</div>

  const barCount = data?.bars.length || 0
  const startDate = data ? new Date(data.start_date) : new Date()
  const endDate = data ? new Date(data.end_date) : new Date()
  const totalDays = Math.max(1, (endDate.getTime() - startDate.getTime()) / (1000 * 60 * 60 * 24))

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-2xl font-bold">Timeline</h1>
        <div className="flex gap-2">
          {(['weeks', 'months', 'quarters'] as const).map(z => (
            <button
              key={z}
              onClick={() => setZoom(z)}
              className={`px-3 py-1 rounded text-sm ${zoom === z ? 'bg-blue-600 text-white' : 'bg-gray-700 text-gray-300 hover:bg-gray-600'}`}
            >
              {z}
            </button>
          ))}
        </div>
      </div>

      <div className="overflow-x-auto border border-gray-700 rounded-lg">
        <div className="min-w-[600px]">
          <div className="flex border-b border-gray-700">
            <div className="w-48 shrink-0 px-3 py-2 text-sm font-medium text-gray-300 border-r border-gray-700">Item</div>
            <div className="flex-1 flex">
              {data?.headers.map((h, i) => (
                <div key={i} className="flex-1 px-2 py-2 text-xs text-gray-400 border-r border-gray-700 text-center">
                  {h}
                </div>
              ))}
            </div>
          </div>

          {data?.bars.map((bar, idx) => {
            if (!bar.start_date || !bar.end_date) return null
            const barStart = new Date(bar.start_date).getTime()
            const barEnd = new Date(bar.end_date).getTime()
            const left = ((barStart - startDate.getTime()) / (totalDays * 1000 * 60 * 60 * 24)) * 100
            const width = Math.max(2, ((barEnd - barStart) / (totalDays * 1000 * 60 * 60 * 24)) * 100)
            const isEpic = bar.type === 'epic'
            return (
              <div key={idx} className="flex border-b border-gray-800 hover:bg-gray-800/50">
                <div className="w-48 shrink-0 px-3 py-2 border-r border-gray-700 flex items-center gap-2">
                  <span
                    className="w-3 h-3 rounded-full shrink-0"
                    style={{ backgroundColor: bar.color }}
                  />
                  <span className="text-sm truncate">{bar.name}</span>
                  <span className="text-xs text-gray-500 ml-auto">{isEpic ? 'Epic' : 'Sprint'}</span>
                </div>
                <div className="flex-1 relative py-2">
                  <div
                    className="absolute h-4 rounded"
                    style={{
                      left: `${left}%`,
                      width: `${width}%`,
                      backgroundColor: bar.color,
                      opacity: 0.8,
                    }}
                    title={`${bar.name}: ${bar.progress}%`}
                  />
                </div>
              </div>
            )
          })}
          {barCount === 0 && (
            <div className="p-8 text-center text-gray-500">No sprints or epics found in this project.</div>
          )}
        </div>
      </div>
    </div>
  )
}
