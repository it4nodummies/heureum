import { useState } from 'react'
import BoardPage from './pages/Board'
import BacklogPage from './pages/Backlog'
import IssueDetail from './pages/IssueDetail'
import SearchPage from './pages/Search'
import ReportsPage from './pages/Reports'
import DashboardPage from './pages/Dashboard'
import TimelinePage from './pages/Timeline'
import CalendarPage from './pages/Calendar'
import NotificationBell from './components/NotificationBell'
import NotificationSettings from './components/NotificationSettings'

type Page = 'board' | 'backlog' | 'issue' | 'search' | 'reports' | 'dashboard' | 'timeline' | 'calendar'

function App() {
  const [page, setPage] = useState<Page>('dashboard')
  const [issueKey, setIssueKey] = useState('')

  function navigateToIssue(key: string) {
    setIssueKey(key)
    setPage('issue')
  }

  return (
    <div>
      <nav className="flex gap-4 p-4 bg-gray-900 text-white items-center">
        <button
          className={page === 'dashboard' ? 'font-bold underline' : ''}
          onClick={() => setPage('dashboard')}
        >
          Dashboard
        </button>
        <button
          className={page === 'board' ? 'font-bold underline' : ''}
          onClick={() => setPage('board')}
        >
          Board
        </button>
        <button
          className={page === 'backlog' ? 'font-bold underline' : ''}
          onClick={() => setPage('backlog')}
        >
          Backlog
        </button>
        <button
          className={page === 'reports' ? 'font-bold underline' : ''}
          onClick={() => setPage('reports')}
        >
          Reports
        </button>
        <button
          className={page === 'search' ? 'font-bold underline' : ''}
          onClick={() => setPage('search')}
        >
          Search
        </button>
        <button
          className={page === 'timeline' ? 'font-bold underline' : ''}
          onClick={() => setPage('timeline')}
        >
          Timeline
        </button>
        <button
          className={page === 'calendar' ? 'font-bold underline' : ''}
          onClick={() => setPage('calendar')}
        >
          Calendar
        </button>
        <div className="flex-1" />
        <NotificationSettings />
        <NotificationBell />
      </nav>
      {page === 'dashboard' && <DashboardPage />}
      {page === 'board' && <BoardPage onNavigateIssue={navigateToIssue} />}
      {page === 'backlog' && <BacklogPage />}
      {page === 'issue' && <IssueDetail issueKey={issueKey} onBack={() => setPage('board')} />}
      {page === 'reports' && <ReportsPage />}
      {page === 'search' && <SearchPage />}
      {page === 'timeline' && <TimelinePage />}
      {page === 'calendar' && <CalendarPage />}
    </div>
  )
}

export default App
