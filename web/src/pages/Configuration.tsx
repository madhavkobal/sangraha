import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ConfigPatch } from '../api/client'
import { Save, CheckCircle, AlertCircle } from 'lucide-react'

export default function Configuration() {
  const qc = useQueryClient()
  const [draft, setDraft] = useState<ConfigPatch>({})
  const [flash, setFlash] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [validationErrors, setValidationErrors] = useState<string[]>([])

  const { data: cfg, isLoading } = useQuery({
    queryKey: ['config'],
    queryFn: api.config.get,
  })

  const validateMut = useMutation({
    mutationFn: (patch: ConfigPatch) => api.config.validate(patch),
    onSuccess: (res) => {
      if (res.valid) {
        setValidationErrors([])
        setFlash({ type: 'success', text: 'Configuration is valid.' })
      } else {
        setValidationErrors(res.errors ?? [])
        setFlash(null)
      }
    },
  })

  const updateMut = useMutation({
    mutationFn: (patch: ConfigPatch) => api.config.update(patch),
    onSuccess: (res) => {
      qc.invalidateQueries({ queryKey: ['config'] })
      setDraft({})
      setValidationErrors([])
      setFlash({
        type: 'success',
        text: res.message + (res.restart_required ? ' — Restart required.' : ''),
      })
    },
    onError: (e: Error) => setFlash({ type: 'error', text: e.message }),
  })

  if (isLoading || !cfg) {
    return <div className="p-6 text-muted text-sm">Loading configuration…</div>
  }

  const logLevel = (draft.logging?.level ?? cfg.logging.level)
  const logFormat = (draft.logging?.format ?? cfg.logging.format)
  const rps = draft.limits?.rate_limit_rps ?? cfg.limits.rate_limit_rps
  const maxBuckets = draft.limits?.max_bucket_count ?? cfg.limits.max_bucket_count

  function setLogLevel(v: string) {
    setDraft(d => ({ ...d, logging: { ...d.logging, level: v } }))
  }
  function setLogFormat(v: string) {
    setDraft(d => ({ ...d, logging: { ...d.logging, format: v } }))
  }
  function setRPS(v: number) {
    setDraft(d => ({ ...d, limits: { ...d.limits, rate_limit_rps: v } }))
  }
  function setMaxBuckets(v: number) {
    setDraft(d => ({ ...d, limits: { ...d.limits, max_bucket_count: v } }))
  }

  const hasDraft = Object.keys(draft).length > 0 &&
    JSON.stringify(draft) !== '{}'

  return (
    <div className="p-6 max-w-3xl mx-auto">
      <div className="flex items-center mb-6">
        <h1 className="text-xl font-semibold flex-1">Configuration</h1>
        {hasDraft && (
          <div className="flex gap-2">
            <button
              onClick={() => validateMut.mutate(draft)}
              className="text-sm px-3 py-1.5 rounded border border-border text-muted hover:text-white transition"
            >
              Validate
            </button>
            <button
              onClick={() => updateMut.mutate(draft)}
              disabled={updateMut.isPending}
              className="flex items-center gap-1.5 text-sm px-3 py-1.5 rounded bg-accent text-bg font-semibold hover:opacity-90 disabled:opacity-50"
            >
              <Save size={13} /> Apply
            </button>
          </div>
        )}
      </div>

      {flash && (
        <div className={`flex items-start gap-2 rounded p-3 mb-4 text-sm ${flash.type === 'success' ? 'bg-green-900/20 border border-success text-success' : 'bg-red-900/20 border border-danger text-danger'}`}>
          {flash.type === 'success' ? <CheckCircle size={14} className="mt-0.5 flex-shrink-0" /> : <AlertCircle size={14} className="mt-0.5 flex-shrink-0" />}
          {flash.text}
        </div>
      )}

      {validationErrors.length > 0 && (
        <div className="bg-red-900/20 border border-danger text-danger rounded p-3 mb-4 text-sm space-y-1">
          {validationErrors.map((e, i) => <div key={i}>• {e}</div>)}
        </div>
      )}

      {/* Read-only server info */}
      <Section title="Server (read-only)">
        <Row label="S3 Address" value={cfg.server.s3_address} />
        <Row label="Admin Address" value={cfg.server.admin_address} />
        <Row label="TLS Enabled" value={cfg.server.tls.enabled ? 'yes' : 'no'} />
        {cfg.server.tls.cert_file && <Row label="TLS Cert" value={cfg.server.tls.cert_file} />}
      </Section>

      <Section title="Storage (read-only)">
        <Row label="Backend" value={cfg.storage.backend} />
        <Row label="Data Dir" value={cfg.storage.data_dir} />
        <Row label="Metadata DB" value={cfg.metadata.path} />
      </Section>

      <Section title="Auth (read-only)">
        <Row label="Root Access Key" value={cfg.auth.root_access_key} />
        <Row label="Root Secret Key" value={cfg.auth.root_secret_key} />
      </Section>

      {/* Editable logging */}
      <Section title="Logging">
        <div className="px-4 py-3 flex items-center gap-4 border-b border-border last:border-0">
          <div className="text-muted text-sm w-40">Log Level</div>
          <select
            value={logLevel}
            onChange={e => setLogLevel(e.target.value)}
            className="bg-bg border border-border rounded px-2 py-1 text-sm text-white focus:outline-none focus:border-accent"
          >
            {['debug', 'info', 'warn', 'error'].map(l => <option key={l} value={l}>{l}</option>)}
          </select>
          {draft.logging?.level && <span className="text-xs text-warn">modified</span>}
        </div>
        <div className="px-4 py-3 flex items-center gap-4">
          <div className="text-muted text-sm w-40">Log Format</div>
          <select
            value={logFormat}
            onChange={e => setLogFormat(e.target.value)}
            className="bg-bg border border-border rounded px-2 py-1 text-sm text-white focus:outline-none focus:border-accent"
          >
            {['json', 'text'].map(f => <option key={f} value={f}>{f}</option>)}
          </select>
          {draft.logging?.format && <span className="text-xs text-warn">modified</span>}
        </div>
        <Row label="Audit Log" value={cfg.logging.audit_log || '(not set)'} />
      </Section>

      {/* Editable limits */}
      <Section title="Limits">
        <Row label="Max Object Size" value={cfg.limits.max_object_size} />
        <div className="px-4 py-3 flex items-center gap-4 border-b border-border">
          <div className="text-muted text-sm w-40">Rate Limit (RPS)</div>
          <input
            type="number"
            value={rps}
            onChange={e => setRPS(parseInt(e.target.value, 10))}
            min={0}
            className="w-24 bg-bg border border-border rounded px-2 py-1 text-sm text-white focus:outline-none focus:border-accent"
          />
          {draft.limits?.rate_limit_rps !== undefined && <span className="text-xs text-warn">modified</span>}
        </div>
        <div className="px-4 py-3 flex items-center gap-4">
          <div className="text-muted text-sm w-40">Max Bucket Count</div>
          <input
            type="number"
            value={maxBuckets}
            onChange={e => setMaxBuckets(parseInt(e.target.value, 10))}
            min={1}
            className="w-24 bg-bg border border-border rounded px-2 py-1 text-sm text-white focus:outline-none focus:border-accent"
          />
          {draft.limits?.max_bucket_count !== undefined && <span className="text-xs text-warn">modified</span>}
        </div>
      </Section>
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="bg-surface border border-border rounded-lg mb-4 overflow-hidden">
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
      <div className="text-muted w-40">{label}</div>
      <div className="font-mono text-xs">{value}</div>
    </div>
  )
}
