import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'

function fmtUptime(sec: number): string {
  if (sec < 60) return `${sec}s`
  if (sec < 3600) return `${Math.floor(sec / 60)}m ${sec % 60}s`
  return `${Math.floor(sec / 3600)}h ${Math.floor((sec % 3600) / 60)}m`
}

export default function Overview() {
  const { data: info, isLoading: infoLoading } = useQuery({
    queryKey: ['info'],
    queryFn: api.info,
    refetchInterval: 30_000,
  })

  const { data: health } = useQuery({
    queryKey: ['health'],
    queryFn: api.health,
    refetchInterval: 15_000,
  })

  const { data: users } = useQuery({
    queryKey: ['users'],
    queryFn: api.users.list,
    refetchInterval: 60_000,
  })

  const { data: conns } = useQuery({
    queryKey: ['connections'],
    queryFn: api.server.connections,
    refetchInterval: 10_000,
  })

  const stats = [
    { label: 'Status', value: health?.status === 'ok' ? '● Online' : '○ Offline', accent: health?.status === 'ok' },
    { label: 'Users', value: users?.length?.toString() ?? '—' },
    { label: 'Active Connections', value: conns?.active_connections?.toString() ?? '—' },
    { label: 'Uptime', value: info ? fmtUptime(info.uptime_sec) : '—' },
  ]

  return (
    <div className="p-6 max-w-5xl mx-auto">
      <h1 className="text-xl font-semibold mb-6">Overview</h1>

      {infoLoading ? (
        <div className="text-muted text-sm">Loading…</div>
      ) : (
        <>
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
            {stats.map(s => (
              <div key={s.label} className="bg-surface border border-border rounded-lg p-5">
                <div className="text-xs text-muted uppercase tracking-wide mb-2">{s.label}</div>
                <div className={`text-2xl font-bold ${s.accent ? 'text-success' : 'text-accent'}`}>
                  {s.value}
                </div>
              </div>
            ))}
          </div>

          <div className="bg-surface border border-border rounded-lg">
            <div className="px-4 py-3 border-b border-border">
              <h2 className="font-semibold text-sm">Server Info</h2>
            </div>
            <table className="w-full text-sm">
              <tbody>
                {[
                  ['Version', info?.version ?? 'dev'],
                  ['Build Time', info?.build_time ?? '—'],
                  ['Uptime', info ? fmtUptime(info.uptime_sec) : '—'],
                ].map(([k, v]) => (
                  <tr key={k} className="border-b border-border last:border-0">
                    <td className="px-4 py-2.5 text-muted w-40">{k}</td>
                    <td className="px-4 py-2.5">{v}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  )
}
