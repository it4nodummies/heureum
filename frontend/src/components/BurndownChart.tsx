import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts'

interface Props {
  labels: string[]
  ideal: number[]
  actual: number[]
  title?: string
  lineNames?: [string, string]
}

export default function BurndownChart({ labels, ideal, actual, title, lineNames }: Props) {
  const data = labels.map((label, i) => ({
    day: label,
    ideal: ideal[i] ?? 0,
    actual: actual[i] ?? 0,
  }))

  const [idealName, actualName] = lineNames ?? ['Ideal', 'Actual']

  return (
    <div className="bg-gray-800 rounded-lg p-4">
      {title && <h3 className="text-lg font-semibold mb-4 text-gray-200">{title}</h3>}
      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
          <XAxis dataKey="day" stroke="#9CA3AF" fontSize={12} />
          <YAxis stroke="#9CA3AF" fontSize={12} />
          <Tooltip
            contentStyle={{ backgroundColor: '#1F2937', border: '1px solid #374151', borderRadius: '6px' }}
            labelStyle={{ color: '#F9FAFB' }}
          />
          <Legend />
          <Line type="monotone" dataKey="ideal" stroke="#6B7280" strokeDasharray="5 5" name={idealName} dot={false} />
          <Line type="monotone" dataKey="actual" stroke="#3B82F6" name={actualName} dot={false} strokeWidth={2} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
