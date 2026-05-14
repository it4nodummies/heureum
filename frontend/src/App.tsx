import { useState } from 'react'
import BoardPage from './pages/Board'
import BacklogPage from './pages/Backlog'
import IssueDetail from './pages/IssueDetail'
import SearchPage from './pages/Search'

type Page = 'board' | 'backlog' | 'issue' | 'search'

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
        <button
          className={page === 'search' ? 'font-bold underline' : ''}
          onClick={() => setPage('search')}
        >
          Search
        </button>
      </nav>
      {page === 'board' && <BoardPage onNavigateIssue={navigateToIssue} />}
      {page === 'backlog' && <BacklogPage />}
      {page === 'issue' && <IssueDetail issueKey={issueKey} onBack={() => setPage('board')} />}
      {page === 'search' && <SearchPage />}
    </div>
  )
}

export default App
