import { useState } from 'react'
import BoardPage from './pages/Board'
import BacklogPage from './pages/Backlog'

function App() {
  const [page, setPage] = useState<'board' | 'backlog'>('board')

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
      {page === 'board' ? <BoardPage /> : <BacklogPage />}
    </div>
  )
}

export default App
