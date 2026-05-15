import TopBar from './TopBar'
import Sidebar from './Sidebar'

interface Props {
  activePage: string
  onNavigate: (page: string) => void
  children: React.ReactNode
}

export default function Layout({ activePage, onNavigate, children }: Props) {
  return (
    <div className="h-screen flex flex-col bg-surface text-primary font-sans">
      <TopBar />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar activePage={activePage} onNavigate={onNavigate} />
        <main className="flex-1 overflow-auto">
          {children}
        </main>
      </div>
    </div>
  )
}
