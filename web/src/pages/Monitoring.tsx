import { useState, useEffect, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api, getBaseURL } from '../api/client'
import { Wifi, WifiOff, Circle } from 'lucide-react'

interface LogLine {
  id: number
  raw: string
  level: string
  time: string
  message: string
}

function parseLogLine(raw: string, id: number): LogLine {
  try {
    const obj = JSON.parse(raw) as Record<string, unknown>
    return {
      id,
      raw,
      level: String(obj.level ?? 'info'),
      time: String(obj.time ?? ''),
      message: String(obj.message ?? raw),
    }
  } catch {
    return { id, raw, level: 'info', time: '', message: raw }
  }
}

const levelColors: Record<string, string> = {
  debug: 'text-muted',
  info: 'text-white',
  warn: 'text-warn',
  error: 'text-danger',
}

export default function Monitoring() {
  const [lines, setLines] = useState<LogLine[]>([])
  const [connected, setConnected] = useState(false)
  const [paused, setPaused] = useState(false)
  const [levelFilter, setLevelFilter] = useState('')
  const logRef = useRef<HTMLDivElement>(null)
  const lineId = useRef(0)

  const { data: health } = useQuery({ queryKey: ['health'], queryFn: api.health, refetchInterval: 10_000 })
  const { data: conns } = useQuery({ queryKey: ['connections'], queryFn: api.server.connections, refetchInterval: 5_000 })
  const { data: tls } = useQuery({ queryKey: ['tls'], queryFn: api.tls.info, refetchInterval: 60_000 })

  useEffect(() => {
    const url = getBaseURL() + '/admin/v1/logs/stream' + (levelFilter ? `?level=${levelFilter}` : '')
    const es = new EventSource(url)
    es.onopen = () => setConnected(true)
    es.onerror = () => setConnected(false)
    es.onmessage = (e) => {
      const line = parseLogLine(e.data as string, ++lineId.current)
      setLines(prev => {
        const next = [...prev, line]
        return next.length > 500 ? next.slice(next.length - 500) : next
      })
    }
    return () => { es.close(); setConnected(false) }
  }, [levelFilter])

  useEffect(() => {
    if (!paused && logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight
    }
  }, [lines, paused])

  const daysLeft = tls?.days_until_expiry
  const tlsExpiring = daysLeft !== undefined && daysLeft < 30

  return (
    <div className="p-6 max-w-5xl mx-auto space-y-6">
      <h1 className="text-xl font-semibold">Monitoring</h1>

      {/* Status cards */}
      <div className="grid grid-cols-2 lg:grid-cols-3 gap-4">
        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-xs text-muted uppercase tracking-wide mb-2">Health</div>
          <div className={`flex items-center gap-2 font-semibold ${health?.status === 'ok' ? 'text-success' : 'text-danger'}`}>
            <Circle size={10} className="fill-current" />
            {health?.status === 'ok' ? 'Online' : 'Offline'}
          </div>
        </div>

        <div className="bg-surface border border-border rounded-lg p-4">
          <div className="text-xs text-muted uppercase tracking-wide mb-2">Active Connections</div>
          <div className="text-2xl font-bold text-accent">{conns?.active_connections ?? '—'}</div>
        </div>

        <div className={`bg-surface border rounded-lg p-4 ${tlsExpiring ? 'border-warn' : 'border-border'}`}>
          <div className="text-xs text-muted uppercase tracking-wide mb-2">TLS Certificate</div>
          {tls?.status ? (
            <div className="text-muted text-sm">{tls.status}</div>
          ) : daysLeft !== undefined ? (
            <div className={`font-semibold ${tlsExpiring ? 'text-warn' : 'text-success'}`}>
              {daysLeft}d until expiry
            </div>
          ) : (
            <div className="text-muted text-sm">—</div>
          )}
          {tls?.subject && <div className="text-xs text-muted mt-1 truncate">{tls.subject}</div>}
        </div>
      </div>

      {/* Live log viewer */}
      <div className="bg-surface border border-border rounded-lg">
        <div className="flex items-center gap-3 px-4 py-3 border-b border-border">
          <div className="flex-1 font-semibold text-sm">Live Logs</div>
          <div className={`flex items-center gap-1.5 text-xs ${connected ? 'text-success' : 'text-muted'}`}>
            {connected ? <Wifi size={12} /> : <WifiOff size={12} />}
            {connected ? 'Connected' : 'Connecting…'}
          </div>
          <select
            value={levelFilter}
            onChange={e => setLevelFilter(e.target.value)}
            className="bg-bg border border-border rounded px-2 py-1 text-xs text-muted focus:outline-none"
          >
            <option value="">All levels</option>
            <option value="debug">debug</option>
            <option value="info">info</option>
            <option value="warn">warn</option>
            <option value="error">error</option>
          </select>
          <button
            onClick={() => setPaused(p => !p)}
            className="text-xs px-2 py-1 rounded border border-border text-muted hover:text-white transition"
          >
            {paused ? 'Resume' : 'Pause'}
          </button>
          <button
            onClick={() => setLines([])}
            className="text-xs px-2 py-1 rounded border border-border text-muted hover:text-white transition"
          >
            Clear
          </button>
        </div>

        <div
          ref={logRef}
          className="h-80 overflow-y-auto font-mono text-xs p-3 space-y-0.5"
          onMouseEnter={() => setPaused(true)}
          onMouseLeave={() => setPaused(false)}
        >
          {!lines.length ? (
            <div className="text-muted text-center pt-8">Waiting for log lines…</div>
          ) : (
            lines.map(l => (
              <div key={l.id} className={`${levelColors[l.level] ?? 'text-white'} leading-5`}>
                {l.time && <span className="text-muted mr-2">{l.time.slice(11, 19)}</span>}
                <span className="mr-2 uppercase w-5 inline-block">{l.level.slice(0, 4)}</span>
                {l.message}
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  )
}
