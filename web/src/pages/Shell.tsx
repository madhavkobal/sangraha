import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'
import Overview from './Overview'
import Users from './Users'
import Configuration from './Configuration'
import Monitoring from './Monitoring'
import { LayoutDashboard, Users as UsersIcon, Settings, Activity, LogOut, Server } from 'lucide-react'
import ServerPage from './ServerPage'

type Page = 'overview' | 'users' | 'monitoring' | 'configuration' | 'server'

interface Props {
  onLogout: () => void
}

const navItems: { id: Page; label: string; Icon: React.ElementType }[] = [
  { id: 'overview', label: 'Overview', Icon: LayoutDashboard },
  { id: 'users', label: 'Users', Icon: UsersIcon },
  { id: 'monitoring', label: 'Monitoring', Icon: Activity },
  { id: 'configuration', label: 'Configuration', Icon: Settings },
  { id: 'server', label: 'Server', Icon: Server },
]

export default function Shell({ onLogout }: Props) {
  const [page, setPage] = useState<Page>('overview')

  const { data: info } = useQuery({
    queryKey: ['info'],
    queryFn: api.info,
    refetchInterval: 60_000,
  })

  return (
    <div className="flex min-h-screen bg-bg text-white">
      {/* Sidebar */}
      <aside className="w-56 bg-surface border-r border-border flex flex-col">
        <div className="px-4 py-4 border-b border-border">
          <div className="text-accent font-bold text-lg">संग्रह</div>
          <div className="text-muted text-xs">sangraha {info?.version ?? 'dev'}</div>
        </div>

        <nav className="flex-1 p-2 space-y-0.5">
          {navItems.map(({ id, label, Icon }) => (
            <button
              key={id}
              onClick={() => setPage(id)}
              className={`w-full flex items-center gap-2.5 px-3 py-2 rounded text-sm transition ${
                page === id
                  ? 'bg-accent/10 text-accent'
                  : 'text-muted hover:bg-white/5 hover:text-white'
              }`}
            >
              <Icon size={15} />
              {label}
            </button>
          ))}
        </nav>

        <div className="p-2 border-t border-border">
          <button
            onClick={onLogout}
            className="w-full flex items-center gap-2.5 px-3 py-2 rounded text-sm text-muted hover:bg-white/5 hover:text-white transition"
          >
            <LogOut size={15} />
            Disconnect
          </button>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        {page === 'overview' && <Overview />}
        {page === 'users' && <Users />}
        {page === 'monitoring' && <Monitoring />}
        {page === 'configuration' && <Configuration />}
        {page === 'server' && <ServerPage />}
      </main>
    </div>
  )
}
