export default function BoardPage(_props: { onNavigateIssue?: (key: string) => void }) {
  return (
    <div className="p-6">
      <h1 className="text-2xl font-bold mb-4">Board</h1>
      <div className="flex gap-4 overflow-x-auto">
        <p className="text-gray-400">Board columns will appear here.</p>
      </div>
    </div>
  )
}
