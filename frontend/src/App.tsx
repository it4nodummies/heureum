import { useState } from 'react'
import BoardPage from './pages/Board'
import BacklogPage from './pages/Backlog'
import IssueDetail from './pages/IssueDetail'

type Page = 'board' | 'backlog' | 'issue'

function App() {
  const [page, setPage] = useState<Page>('board')
  const [issueKey, setIssueKey] = useState('')

  function navigateToIssue(key: string) {
    setIssueKey(key)
    setPage('issue')
  }

  return (
    <div>
      <nav className="flex gap-4 p-4 bg-gray-900 text-white">
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
      </nav>
      {page === 'board' && <BoardPage onNavigateIssue={navigateToIssue} />}
      {page === 'backlog' && <BacklogPage />}
      {page === 'issue' && <IssueDetail issueKey={issueKey} onBack={() => setPage('board')} />}
    </div>
  )
}

export default App
