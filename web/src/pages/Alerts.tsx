import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, AlertRule, AlertEvent, CreateAlertRuleRequest } from '../api/client'
import { Plus, Trash2, RefreshCw, Bell } from 'lucide-react'

const METRIC_LABELS: Record<string, string> = {
  disk_usage_pct: 'Disk Usage %',
  error_rate_pct: 'Error Rate %',
  req_per_sec: 'Requests / sec',
}

const OPERATOR_LABELS: Record<string, string> = {
  gt: '>',
  lt: '<',
  gte: '≥',
  lte: '≤',
}

interface CreateAlertDialogProps {
  onClose: () => void
  onCreated: () => void
}

function CreateAlertDialog({ onClose, onCreated }: CreateAlertDialogProps) {
  const [metric, setMetric] = useState('disk_usage_pct')
  const [operator, setOperator] = useState('gt')
  const [threshold, setThreshold] = useState('80')
  const [label, setLabel] = useState('')
  const [error, setError] = useState('')

  const mutation = useMutation({
    mutationFn: (req: CreateAlertRuleRequest) => api.alerts.createRule(req),
    onSuccess: () => { onCreated(); onClose() },
    onError: (err: Error) => setError(err.message),
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const t = parseFloat(threshold)
    if (isNaN(t)) { setError('Threshold must be a number'); return }
    if (!label.trim()) { setError('Label is required'); return }
    mutation.mutate({ metric, operator, threshold: t, label: label.trim() })
  }

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
      <div className="bg-surface border border-border rounded-lg p-6 w-full max-w-md">
        <h2 className="text-lg font-semibold mb-4">Create Alert Rule</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm text-muted mb-1">Metric</label>
            <select
              className="w-full bg-bg border border-border rounded px-3 py-2 text-sm text-white focus:outline-none focus:border-accent"
              value={metric}
              onChange={e => setMetric(e.target.value)}
            >
              {Object.entries(METRIC_LABELS).map(([v, l]) => (
                <option key={v} value={v}>{l}</option>
              ))}
            </select>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-sm text-muted mb-1">Operator</label>
              <select
                className="w-full bg-bg border border-border rounded px-3 py-2 text-sm text-white focus:outline-none focus:border-accent"
                value={operator}
                onChange={e => setOperator(e.target.value)}
              >
                {Object.entries(OPERATOR_LABELS).map(([v, l]) => (
                  <option key={v} value={v}>{l}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-sm text-muted mb-1">Threshold</label>
              <input
                type="number"
                step="any"
                className="w-full bg-bg border border-border rounded px-3 py-2 text-sm text-white focus:outline-none focus:border-accent"
                value={threshold}
                onChange={e => setThreshold(e.target.value)}
              />
            </div>
          </div>
          <div>
            <label className="block text-sm text-muted mb-1">Label</label>
            <input
              className="w-full bg-bg border border-border rounded px-3 py-2 text-sm text-white focus:outline-none focus:border-accent"
              value={label}
              onChange={e => setLabel(e.target.value)}
              placeholder="High disk usage warning"
              autoFocus
            />
          </div>
          {error && <p className="text-red-400 text-sm">{error}</p>}
          <div className="flex justify-end gap-2 pt-2">
            <button type="button" onClick={onClose} className="px-4 py-2 text-sm text-muted hover:text-white rounded transition">Cancel</button>
            <button
              type="submit"
              disabled={mutation.isPending}
              className="px-4 py-2 text-sm bg-accent text-white rounded hover:bg-accent/80 disabled:opacity-50 transition"
            >
              {mutation.isPending ? 'Creating…' : 'Create'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

function RuleCard({ rule, onDelete }: { rule: AlertRule; onDelete: () => void }) {
  const [confirm, setConfirm] = useState(false)

  return (
    <div className="bg-surface border border-border rounded-lg p-4 flex items-start justify-between">
      <div>
        <div className="flex items-center gap-2">
          <Bell size={14} className="text-accent" />
          <span className="font-medium text-sm">{rule.label}</span>
        </div>
        <p className="text-muted text-xs mt-1 font-mono">
          {METRIC_LABELS[rule.metric] ?? rule.metric} {OPERATOR_LABELS[rule.operator] ?? rule.operator} {rule.threshold}
        </p>
        <p className="text-muted text-xs mt-0.5">
          Created {new Date(rule.created_at).toLocaleDateString()}
        </p>
      </div>
      <div>
        {!confirm ? (
          <button
            onClick={() => setConfirm(true)}
            className="p-1.5 text-muted hover:text-red-400 rounded transition"
            title="Delete rule"
          >
            <Trash2 size={14} />
          </button>
        ) : (
          <div className="flex items-center gap-1.5">
            <button onClick={() => setConfirm(false)} className="text-xs text-muted hover:text-white">Cancel</button>
            <button onClick={onDelete} className="text-xs text-red-400 hover:text-red-300">Confirm</button>
          </div>
        )}
      </div>
    </div>
  )
}

function HistoryTable({ events }: { events: AlertEvent[] }) {
  if (events.length === 0) {
    return <p className="text-muted text-sm text-center py-8">No alert events in history.</p>
  }

  return (
    <table className="w-full text-xs">
      <thead>
        <tr className="border-b border-border text-muted text-left">
          <th className="pb-2 pr-4 font-medium">Fired At</th>
          <th className="pb-2 pr-4 font-medium">Rule</th>
          <th className="pb-2 pr-4 font-medium">Metric</th>
          <th className="pb-2 pr-4 font-medium">Value</th>
          <th className="pb-2 pr-4 font-medium">Threshold</th>
          <th className="pb-2 font-medium">Status</th>
        </tr>
      </thead>
      <tbody>
        {events.map((e, i) => (
          <tr key={`${e.id}-${i}`} className="border-b border-border/40 hover:bg-white/[0.02] transition">
            <td className="py-2.5 pr-4 text-muted font-mono whitespace-nowrap">
              {new Date(e.fired_at).toLocaleString()}
            </td>
            <td className="py-2.5 pr-4">{e.rule_label}</td>
            <td className="py-2.5 pr-4 text-muted font-mono">{METRIC_LABELS[e.metric] ?? e.metric}</td>
            <td className="py-2.5 pr-4 font-mono">{e.value.toFixed(2)}</td>
            <td className="py-2.5 pr-4 font-mono">{e.threshold}</td>
            <td className="py-2.5">
              {e.resolved ? (
                <span className="text-xs px-2 py-0.5 rounded-full bg-green-900/30 text-green-400">Resolved</span>
              ) : (
                <span className="text-xs px-2 py-0.5 rounded-full bg-red-900/30 text-red-400">Firing</span>
              )}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

export default function Alerts() {
  const qc = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)
  const [tab, setTab] = useState<'rules' | 'history'>('rules')

  const { data: rules = [], isLoading: rulesLoading, refetch: refetchRules } = useQuery({
    queryKey: ['alerts'],
    queryFn: api.alerts.listRules,
    refetchInterval: 60_000,
  })

  const { data: history = [], isLoading: historyLoading, refetch: refetchHistory } = useQuery({
    queryKey: ['alerts-history'],
    queryFn: api.alerts.history,
    refetchInterval: 60_000,
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.alerts.deleteRule(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alerts'] }),
  })

  const refresh = () => { refetchRules(); refetchHistory() }

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-xl font-semibold">Alerts</h1>
          <p className="text-muted text-sm mt-0.5">{rules.length} rule{rules.length !== 1 ? 's' : ''} · {history.length} event{history.length !== 1 ? 's' : ''}</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={refresh} className="p-2 text-muted hover:text-white rounded transition" title="Refresh">
            <RefreshCw size={16} />
          </button>
          <button
            onClick={() => setShowCreate(true)}
            className="flex items-center gap-2 px-3 py-2 text-sm bg-accent text-white rounded hover:bg-accent/80 transition"
          >
            <Plus size={15} /> New Rule
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-border mb-6">
        {(['rules', 'history'] as const).map(t => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-2 text-sm capitalize transition border-b-2 -mb-px ${
              tab === t ? 'border-accent text-accent' : 'border-transparent text-muted hover:text-white'
            }`}
          >
            {t === 'rules' ? `Rules (${rules.length})` : `History (${history.length})`}
          </button>
        ))}
      </div>

      {tab === 'rules' && (
        <>
          {rulesLoading && <p className="text-muted text-sm">Loading…</p>}
          {!rulesLoading && rules.length === 0 && (
            <p className="text-muted text-sm text-center py-12">No alert rules configured.</p>
          )}
          <div className="grid gap-3">
            {rules.map(rule => (
              <RuleCard
                key={rule.id}
                rule={rule}
                onDelete={() => deleteMutation.mutate(rule.id)}
              />
            ))}
          </div>
        </>
      )}

      {tab === 'history' && (
        <>
          {historyLoading && <p className="text-muted text-sm">Loading…</p>}
          <div className="overflow-x-auto">
            <HistoryTable events={history} />
          </div>
        </>
      )}

      {showCreate && (
        <CreateAlertDialog
          onClose={() => setShowCreate(false)}
          onCreated={() => qc.invalidateQueries({ queryKey: ['alerts'] })}
        />
      )}
    </div>
  )
}
