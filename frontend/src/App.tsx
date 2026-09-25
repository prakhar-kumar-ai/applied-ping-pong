import { useState, useEffect, createContext, useContext } from 'react'
import Sidebar from './components/Sidebar'
import Home from './components/Home'
import IntegrationBar from './components/IntegrationBar'
import Register from './components/Register'
import Bracket from './components/Bracket'
import AdminDashboard from './components/AdminDashboard'

export type Tab = 'home'

interface BotStatus {
  ready: boolean
  message: string
}

const StatusContext = createContext<BotStatus>({ ready: false, message: 'Loading...' })

export function useStatus() {
  return useContext(StatusContext)
}

function App() {
  const [activeTab, setActiveTab] = useState<Tab>('home')
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const [status, setStatus] = useState<BotStatus>({ ready: false, message: 'Loading...' })

  useEffect(() => {
    const fetchStatus = async () => {
      try {
        const response = await fetch('/slack/status')
        const data = await response.json()
        setStatus({ ready: data.ready, message: data.message })
      } catch {
        setStatus({ ready: false, message: 'Failed to connect to server' })
      }
    }
    fetchStatus()
    const interval = setInterval(fetchStatus, 10000)
    return () => clearInterval(interval)
  }, [])

  const path = window.location.pathname

  // Full-page routes (no sidebar)
  if (path === '/bracket') return <Bracket />
  if (path === '/admin') return <AdminDashboard />
  if (path === '/' || path === '/register') return <Register />

  // Default: existing sidebar layout (for any other paths)
  return (
    <StatusContext.Provider value={status}>
      <div className="flex h-screen bg-gray-100">
        <Sidebar activeTab={activeTab} setActiveTab={setActiveTab} isOpen={sidebarOpen} onToggle={() => setSidebarOpen(v => !v)} />
        <main className="flex-1 overflow-auto flex flex-col">
          <IntegrationBar />
          <div className="flex-1 overflow-auto p-6">
            <Home />
          </div>
          <div className="text-right px-6 py-2 text-xs text-gray-600 font-semibold">
            Made with the{' '}
            <a
              href="https://agentic-app-platform.experimental.staging.apps.applied.dev"
              target="_blank"
              rel="noopener noreferrer"
              className="underline hover:text-gray-600"
            >
              Agentic App Builder
            </a>
          </div>
        </main>
      </div>
    </StatusContext.Provider>
  )
}

export default App
