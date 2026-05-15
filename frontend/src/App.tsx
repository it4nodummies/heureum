import { useState } from 'react'
import Layout from './components/Layout'
import BoardPage from './pages/Board'
import BacklogPage from './pages/Backlog'
import IssueDetail from './pages/IssueDetail'
import SearchPage from './pages/Search'
import Reports from './pages/Reports'
import DashboardPage from './pages/Dashboard'
import Timeline from './pages/Timeline'
import Calendar from './pages/Calendar'

type Page = 'board' | 'backlog' | 'issue' | 'issues' | 'reports' | 'dashboard' | 'timeline' | 'calendar'

function App() {
  const [page, setPage] = useState<Page>('board')
  const [issueKey, setIssueKey] = useState('')

  function navigateToIssue(key: string) {
    setIssueKey(key)
    setPage('issue')
  }

  return (
    <Layout activePage={page} onNavigate={(p) => setPage(p as Page)}>
      {page === 'board' && <BoardPage onNavigateIssue={navigateToIssue} />}
      {page === 'backlog' && <BacklogPage />}
      {page === 'issue' && <IssueDetail issueKey={issueKey} onBack={() => setPage('board')} />}
      {page === 'issues' && <SearchPage />}
      {page === 'reports' && <Reports />}
      {page === 'dashboard' && <DashboardPage />}
      {page === 'timeline' && <Timeline />}
      {page === 'calendar' && <Calendar />}
    </Layout>
  )
}

export default App
