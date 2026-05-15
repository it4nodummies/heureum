import { useState, useEffect } from 'react'
import { Search, Plus, Menu } from 'lucide-react'

export default function TopBar() {
  const [projectName, setProjectName] = useState('Projects')

  useEffect(() => {
    const token = localStorage.getItem('token')
    if (!token) return
    fetch('/api/v1/projects', {
      headers: { Authorization: `Bearer ${token}` }
    })
      .then(r => r.json())
      .then(p => { if (p && p.length > 0) setProjectName(p[0].name || 'Projects') })
      .catch(() => {})
  }, [])

  return (
    <div className="h-12 bg-sidebar flex items-center px-4 gap-4 border-b border-default shrink-0">
      <button className="text-secondary hover:text-primary"><Menu className="w-5 h-5" /></button>
      <span className="text-white font-bold text-lg tracking-tight mr-2">Open Jira</span>
      <span className="text-secondary text-sm">Projects / {projectName} / Board</span>
      <span className="text-secondary text-xs cursor-pointer hover:text-primary">&#x25BC;</span>
      <div className="flex-1" />
      <div className="flex items-center gap-1 bg-input rounded-sm px-2 h-8 w-56 border border-default">
        <Search className="w-4 h-4 text-subtlest" />
        <input className="bg-transparent text-sm text-primary outline-none w-full placeholder:text-subtlest" placeholder="Search..." />
      </div>
      <button className="bg-accent-blue text-white text-sm font-medium rounded-sm h-8 px-4 hover:brightness-110 flex items-center gap-1.5">
        <Plus className="w-4 h-4" /> Create
      </button>
      <div className="w-8 h-8 rounded-full bg-[#091E420F] flex items-center justify-center text-sm font-medium text-primary cursor-pointer hover:bg-card-hover">
        AM
      </div>
    </div>
  )
}
