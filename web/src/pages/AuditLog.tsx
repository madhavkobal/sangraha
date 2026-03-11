import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'
import { Search, RefreshCw, Download } from 'lucide-react'

function statusColor(code: number): string {
  if (code >= 500) return 'text-red-400'
  if (code >= 400) return 'text-yellow-400'
  if (code >= 200 && code < 300) return 'text-green-400'
  return 'text-muted'
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString()
}

function formatBytes(bytes: number): string {
  if (!bytes) return '—'
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}

export default function AuditLog() {
  const [filterUser, setFilterUser] = useState('')
  const [filterBucket, setFilterBucket] = useState('')
  const [filterAction, setFilterAction] = useState('')
  const [limitStr, setLimitStr] = useState('200')
  const [expanded, setExpanded] = useState<string | null>(null)

  const limit = parseInt(limitStr, 10) || 200

  const { data: entries = [], isLoading, error, refetch } = useQuery({
    queryKey: ['audit', filterUser, filterBucket, filterAction, limit],
    queryFn: () =>
      api.audit.query({
        user: filterUser || undefined,
        bucket: filterBucket || undefined,
        action: filterAction || undefined,
        limit,
      }),
    refetchInterval: 30_000,
  })

  const exportCSV = () => {
    const header = 'time,request_id,user,action,bucket,key,source_ip,status,bytes,duration_ms,error'
    const rows = entries.map(e =>
      [
        e.time,
        e.request_id,
        e.user,
        e.action,
        e.bucket ?? '',
        e.key ?? '',
        e.source_ip ?? '',
        e.status,
        e.bytes ?? '',
        e.duration_ms,
        e.error ?? '',
      ]
        .map(v => `"${String(v).replace(/"/g, '""')}"`)
        .join(','),
    )
    const csv = [header, ...rows].join('\n')
    const blob = new Blob([csv], { type: 'text/csv' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `audit-${new Date().toISOString().slice(0, 10)}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-xl font-semibold">Audit Log</h1>
          <p className="text-muted text-sm mt-0.5">{entries.length} event{entries.length !== 1 ? 's' : ''} loaded</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={exportCSV} className="flex items-center gap-1.5 px-3 py-2 text-sm text-muted hover:text-white border border-border rounded transition">
            <Download size={14} /> Export CSV
          </button>
          <button onClick={() => refetch()} className="p-2 text-muted hover:text-white rounded transition" title="Refresh">
            <RefreshCw size={16} />
          </button>
        </div>
      </div>

      {/* Filters */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
        <div className="relative">
          <Search size={13} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted" />
          <input
            className="w-full bg-surface border border-border rounded pl-8 pr-3 py-2 text-sm text-white focus:outline-none focus:border-accent"
            placeholder="User"
            value={filterUser}
            onChange={e => setFilterUser(e.target.value)}
          />
        </div>
        <div className="relative">
          <Search size={13} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted" />
          <input
            className="w-full bg-surface border border-border rounded pl-8 pr-3 py-2 text-sm text-white focus:outline-none focus:border-accent"
            placeholder="Bucket"
            value={filterBucket}
            onChange={e => setFilterBucket(e.target.value)}
          />
        </div>
        <div className="relative">
          <Search size={13} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted" />
          <input
            className="w-full bg-surface border border-border rounded pl-8 pr-3 py-2 text-sm text-white focus:outline-none focus:border-accent"
            placeholder="Action (e.g. s3:PutObject)"
            value={filterAction}
            onChange={e => setFilterAction(e.target.value)}
          />
        </div>
        <select
          className="bg-surface border border-border rounded px-3 py-2 text-sm text-white focus:outline-none focus:border-accent"
          value={limitStr}
          onChange={e => setLimitStr(e.target.value)}
        >
          <option value="50">Last 50</option>
          <option value="200">Last 200</option>
          <option value="500">Last 500</option>
          <option value="1000">Last 1000</option>
        </select>
      </div>

      {isLoading && <p className="text-muted text-sm">Loading…</p>}
      {error && <p className="text-red-400 text-sm">{(error as Error).message}</p>}

      {!isLoading && entries.length === 0 && (
        <p className="text-muted text-sm text-center py-12">No audit events found.</p>
      )}

      <div className="overflow-x-auto">
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b border-border text-muted text-left">
              <th className="pb-2 pr-4 font-medium">Time</th>
              <th className="pb-2 pr-4 font-medium">User</th>
              <th className="pb-2 pr-4 font-medium">Action</th>
              <th className="pb-2 pr-4 font-medium">Bucket / Key</th>
              <th className="pb-2 pr-4 font-medium">Status</th>
              <th className="pb-2 pr-4 font-medium">Bytes</th>
              <th className="pb-2 pr-4 font-medium">Duration</th>
              <th className="pb-2 font-medium">Source IP</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((e, i) => {
              const rowKey = `${e.request_id}-${i}`
              const isExpanded = expanded === rowKey
              return (
                <>
                  <tr
                    key={rowKey}
                    className="border-b border-border/40 hover:bg-white/[0.02] transition cursor-pointer"
                    onClick={() => setExpanded(isExpanded ? null : rowKey)}
                  >
                    <td className="py-2.5 pr-4 text-muted font-mono whitespace-nowrap">{formatDate(e.time)}</td>
                    <td className="py-2.5 pr-4 font-mono">{e.user}</td>
                    <td className="py-2.5 pr-4 font-mono">{e.action}</td>
                    <td className="py-2.5 pr-4 font-mono text-muted truncate max-w-[200px]">
                      {e.bucket ? (e.key ? `${e.bucket}/${e.key}` : e.bucket) : '—'}
                    </td>
                    <td className={`py-2.5 pr-4 font-mono ${statusColor(e.status)}`}>{e.status}</td>
                    <td className="py-2.5 pr-4 text-muted">{formatBytes(e.bytes ?? 0)}</td>
                    <td className="py-2.5 pr-4 text-muted">{e.duration_ms}ms</td>
                    <td className="py-2.5 text-muted font-mono">{e.source_ip || '—'}</td>
                  </tr>
                  {isExpanded && (
                    <tr key={`${rowKey}-detail`} className="border-b border-border/40 bg-white/[0.015]">
                      <td colSpan={8} className="px-4 py-3">
                        <div className="grid grid-cols-2 gap-2 font-mono">
                          <div><span className="text-muted">Request ID:</span> {e.request_id}</div>
                          {e.error && <div className="col-span-2"><span className="text-muted">Error:</span> <span className="text-red-400">{e.error}</span></div>}
                        </div>
                      </td>
                    </tr>
                  )}
                </>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}
