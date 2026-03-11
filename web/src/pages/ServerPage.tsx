import { useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { api } from '../api/client'
import { RefreshCw, Shield, Trash2 } from 'lucide-react'

export default function ServerPage() {
  const [flash, setFlash] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [gcConfirm, setGcConfirm] = useState(false)

  const { data: info } = useQuery({ queryKey: ['info'], queryFn: api.info, refetchInterval: 10_000 })
  const { data: tls } = useQuery({ queryKey: ['tls'], queryFn: api.tls.info, refetchInterval: 60_000 })
  const [gcPolling, setGcPolling] = useState(false)
  const { data: gcStatus, refetch: refetchGC } = useQuery({
    queryKey: ['gc-status'],
    queryFn: api.gc.status,
    refetchInterval: gcPolling ? 2_000 : false,
  })

  const reloadMut = useMutation({
    mutationFn: api.server.reload,
    onSuccess: (res) => setFlash({ type: 'success', text: res.message }),
    onError: (e: Error) => setFlash({ type: 'error', text: e.message }),
  })

  const tlsRenewMut = useMutation({
    mutationFn: api.tls.renew,
    onSuccess: (res) => setFlash({ type: 'success', text: res.message }),
    onError: (e: Error) => setFlash({ type: 'error', text: e.message }),
  })

  const gcMut = useMutation({
    mutationFn: api.gc.trigger,
    onSuccess: () => {
      setGcConfirm(false)
      setGcPolling(true)
      setFlash({ type: 'success', text: 'Garbage collection started.' })
      void refetchGC()
    },
    onError: (e: Error) => setFlash({ type: 'error', text: e.message }),
  })

  return (
    <div className="p-6 max-w-3xl mx-auto space-y-6">
      <h1 className="text-xl font-semibold">Server</h1>

      {flash && (
        <div className={`rounded p-3 text-sm ${flash.type === 'success' ? 'bg-green-900/20 border border-success text-success' : 'bg-red-900/20 border border-danger text-danger'}`}>
          {flash.text}
          <button onClick={() => setFlash(null)} className="ml-2 underline text-xs">×</button>
        </div>
      )}

      {/* Server info */}
      <Card title="Server Info">
        <Row label="Version" value={info?.version ?? 'dev'} />
        <Row label="Build Time" value={info?.build_time ?? '—'} />
        <Row label="Uptime" value={info ? `${Math.floor(info.uptime_sec / 60)}m` : '—'} />
        <div className="px-4 py-3 flex justify-end">
          <button
            onClick={() => reloadMut.mutate()}
            disabled={reloadMut.isPending}
            className="flex items-center gap-1.5 text-sm px-3 py-1.5 rounded border border-border text-muted hover:text-white transition"
          >
            <RefreshCw size={13} /> Reload Config
          </button>
        </div>
      </Card>

      {/* TLS */}
      <Card title="TLS Certificate">
        {tls?.status ? (
          <div className="px-4 py-3 text-muted text-sm">{tls.status}</div>
        ) : (
          <>
            <Row label="Subject" value={tls?.subject ?? '—'} />
            <Row label="Issuer" value={tls?.issuer ?? '—'} />
            <Row label="Expires" value={tls?.not_after ? new Date(tls.not_after).toLocaleDateString() : '—'} />
            <Row label="Days Until Expiry" value={tls?.days_until_expiry?.toString() ?? '—'} />
            <Row label="Self-Signed" value={tls?.is_self_signed ? 'yes' : 'no'} />
            <Row label="SHA-256" value={tls?.fingerprint_sha256 ?? '—'} />
          </>
        )}
        <div className="px-4 py-3 flex justify-end border-t border-border">
          <button
            onClick={() => tlsRenewMut.mutate()}
            disabled={tlsRenewMut.isPending || !tls?.is_self_signed}
            title={!tls?.is_self_signed ? 'Only available for self-signed certificates' : undefined}
            className="flex items-center gap-1.5 text-sm px-3 py-1.5 rounded border border-border text-muted hover:text-white transition disabled:opacity-40"
          >
            <Shield size={13} /> Renew Self-Signed Cert
          </button>
        </div>
      </Card>

      {/* GC */}
      <Card title="Garbage Collection">
        <Row label="Status" value={gcStatus?.running ? 'Running…' : 'Idle'} />
        <Row label="Last Run" value={gcStatus?.last_run ? new Date(gcStatus.last_run).toLocaleString() : 'Never'} />
        <Row label="Objects Scanned" value={gcStatus?.scanned?.toString() ?? '0'} />
        <Row label="Objects Deleted" value={gcStatus?.deleted?.toString() ?? '0'} />
        <Row label="Bytes Freed" value={gcStatus?.freed_bytes?.toString() ?? '0'} />
        {gcStatus?.running && (
          <div className="px-4 pb-3">
            <div className="h-1.5 bg-border rounded-full overflow-hidden">
              <div className="h-full bg-accent animate-pulse w-1/2" />
            </div>
          </div>
        )}
        <div className="px-4 py-3 flex justify-end border-t border-border">
          <button
            onClick={() => setGcConfirm(true)}
            disabled={gcStatus?.running || gcMut.isPending}
            className="flex items-center gap-1.5 text-sm px-3 py-1.5 rounded bg-danger/10 border border-danger/40 text-danger hover:bg-danger/20 transition disabled:opacity-40"
          >
            <Trash2 size={13} /> Run GC
          </button>
        </div>
      </Card>

      {gcConfirm && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-surface border border-border rounded-lg p-6 w-80">
            <h2 className="font-semibold mb-2">Run Garbage Collection?</h2>
            <p className="text-muted text-sm mb-4">This will scan for and remove orphaned objects. The process runs in the background.</p>
            <div className="flex gap-2">
              <button
                onClick={() => gcMut.mutate()}
                className="flex-1 bg-danger/10 border border-danger text-danger rounded px-3 py-1.5 text-sm hover:bg-danger/20"
              >
                Run GC
              </button>
              <button
                onClick={() => setGcConfirm(false)}
                className="flex-1 bg-surface border border-border text-muted rounded px-3 py-1.5 text-sm hover:text-white"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="bg-surface border border-border rounded-lg overflow-hidden">
      <div className="px-4 py-2.5 border-b border-border bg-white/[0.02]">
        <h2 className="text-xs font-semibold text-muted uppercase tracking-wide">{title}</h2>
      </div>
      {children}
    </div>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="px-4 py-2.5 flex items-center border-b border-border last:border-0 text-sm">
      <div className="text-muted w-44">{label}</div>
      <div className="font-mono text-xs">{value}</div>
    </div>
  )
}
