import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, Bucket, CreateBucketRequest } from '../api/client'
import { Plus, Trash2, FolderOpen, RefreshCw } from 'lucide-react'

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`
}

interface CreateBucketDialogProps {
  onClose: () => void
  onCreated: () => void
}

function CreateBucketDialog({ onClose, onCreated }: CreateBucketDialogProps) {
  const [name, setName] = useState('')
  const [region, setRegion] = useState('us-east-1')
  const [acl, setAcl] = useState('private')
  const [error, setError] = useState('')

  const mutation = useMutation({
    mutationFn: (req: CreateBucketRequest) => api.buckets.create(req),
    onSuccess: () => {
      onCreated()
      onClose()
    },
    onError: (err: Error) => setError(err.message),
  })

  const validateName = (n: string): string => {
    if (n.length < 3 || n.length > 63) return 'Name must be 3–63 characters'
    if (!/^[a-z0-9][a-z0-9.-]*[a-z0-9]$/.test(n)) return 'Name must be lowercase, alphanumeric, dots, or hyphens'
    if (/\.\./.test(n)) return 'Name must not contain consecutive dots'
    return ''
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const nameErr = validateName(name)
    if (nameErr) { setError(nameErr); return }
    mutation.mutate({ name, region, acl })
  }

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
      <div className="bg-surface border border-border rounded-lg p-6 w-full max-w-md">
        <h2 className="text-lg font-semibold mb-4">Create Bucket</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm text-muted mb-1">Bucket Name</label>
            <input
              className="w-full bg-bg border border-border rounded px-3 py-2 text-sm text-white focus:outline-none focus:border-accent"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="my-bucket"
              autoFocus
            />
          </div>
          <div>
            <label className="block text-sm text-muted mb-1">Region</label>
            <input
              className="w-full bg-bg border border-border rounded px-3 py-2 text-sm text-white focus:outline-none focus:border-accent"
              value={region}
              onChange={e => setRegion(e.target.value)}
            />
          </div>
          <div>
            <label className="block text-sm text-muted mb-1">ACL</label>
            <select
              className="w-full bg-bg border border-border rounded px-3 py-2 text-sm text-white focus:outline-none focus:border-accent"
              value={acl}
              onChange={e => setAcl(e.target.value)}
            >
              <option value="private">private</option>
              <option value="public-read">public-read</option>
              <option value="public-read-write">public-read-write</option>
              <option value="authenticated-read">authenticated-read</option>
            </select>
          </div>
          {error && <p className="text-red-400 text-sm">{error}</p>}
          <div className="flex justify-end gap-2 pt-2">
            <button type="button" onClick={onClose} className="px-4 py-2 text-sm text-muted hover:text-white rounded transition">
              Cancel
            </button>
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

interface DeleteConfirmProps {
  bucket: Bucket
  onClose: () => void
  onDeleted: () => void
}

function DeleteConfirmDialog({ bucket, onClose, onDeleted }: DeleteConfirmProps) {
  const [input, setInput] = useState('')
  const [error, setError] = useState('')

  const mutation = useMutation({
    mutationFn: () => api.buckets.delete(bucket.name),
    onSuccess: () => { onDeleted(); onClose() },
    onError: (err: Error) => setError(err.message),
  })

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
      <div className="bg-surface border border-border rounded-lg p-6 w-full max-w-md">
        <h2 className="text-lg font-semibold mb-2">Delete Bucket</h2>
        <p className="text-muted text-sm mb-4">
          Type <span className="text-white font-mono">{bucket.name}</span> to confirm deletion.
          This action cannot be undone.
        </p>
        <input
          className="w-full bg-bg border border-border rounded px-3 py-2 text-sm text-white focus:outline-none focus:border-red-500 mb-3"
          value={input}
          onChange={e => setInput(e.target.value)}
          placeholder={bucket.name}
          autoFocus
        />
        {error && <p className="text-red-400 text-sm mb-3">{error}</p>}
        <div className="flex justify-end gap-2">
          <button onClick={onClose} className="px-4 py-2 text-sm text-muted hover:text-white rounded transition">
            Cancel
          </button>
          <button
            onClick={() => mutation.mutate()}
            disabled={input !== bucket.name || mutation.isPending}
            className="px-4 py-2 text-sm bg-red-600 text-white rounded hover:bg-red-700 disabled:opacity-50 transition"
          >
            {mutation.isPending ? 'Deleting…' : 'Delete'}
          </button>
        </div>
      </div>
    </div>
  )
}

interface BucketsProps {
  onBrowse: (name: string) => void
}

export default function Buckets({ onBrowse }: BucketsProps) {
  const qc = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Bucket | null>(null)
  const [search, setSearch] = useState('')

  const { data: buckets = [], isLoading, error, refetch } = useQuery({
    queryKey: ['buckets'],
    queryFn: api.buckets.list,
    refetchInterval: 30_000,
  })

  const filtered = buckets.filter(b =>
    b.name.toLowerCase().includes(search.toLowerCase()),
  )

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-xl font-semibold">Buckets</h1>
          <p className="text-muted text-sm mt-0.5">{buckets.length} bucket{buckets.length !== 1 ? 's' : ''}</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => refetch()}
            className="p-2 text-muted hover:text-white rounded transition"
            title="Refresh"
          >
            <RefreshCw size={16} />
          </button>
          <button
            onClick={() => setShowCreate(true)}
            className="flex items-center gap-2 px-3 py-2 text-sm bg-accent text-white rounded hover:bg-accent/80 transition"
          >
            <Plus size={15} /> New Bucket
          </button>
        </div>
      </div>

      <input
        className="w-full bg-surface border border-border rounded px-3 py-2 text-sm text-white focus:outline-none focus:border-accent mb-4"
        placeholder="Search buckets…"
        value={search}
        onChange={e => setSearch(e.target.value)}
      />

      {isLoading && <p className="text-muted text-sm">Loading…</p>}
      {error && <p className="text-red-400 text-sm">{(error as Error).message}</p>}

      {!isLoading && filtered.length === 0 && (
        <p className="text-muted text-sm text-center py-12">No buckets found.</p>
      )}

      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-muted text-left">
              <th className="pb-2 pr-4 font-medium">Name</th>
              <th className="pb-2 pr-4 font-medium">Objects</th>
              <th className="pb-2 pr-4 font-medium">Size</th>
              <th className="pb-2 pr-4 font-medium">Versioning</th>
              <th className="pb-2 pr-4 font-medium">ACL</th>
              <th className="pb-2 pr-4 font-medium">Region</th>
              <th className="pb-2 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map(b => (
              <tr key={b.name} className="border-b border-border/40 hover:bg-white/[0.02] transition">
                <td className="py-3 pr-4 font-mono">{b.name}</td>
                <td className="py-3 pr-4 text-muted">{b.object_count.toLocaleString()}</td>
                <td className="py-3 pr-4 text-muted">{formatBytes(b.total_bytes)}</td>
                <td className="py-3 pr-4">
                  <span className={`text-xs px-2 py-0.5 rounded-full ${
                    b.versioning === 'enabled'
                      ? 'bg-green-900/40 text-green-400'
                      : b.versioning === 'suspended'
                      ? 'bg-yellow-900/40 text-yellow-400'
                      : 'bg-white/5 text-muted'
                  }`}>
                    {b.versioning}
                  </span>
                </td>
                <td className="py-3 pr-4 text-muted">{b.acl}</td>
                <td className="py-3 pr-4 text-muted">{b.region || '—'}</td>
                <td className="py-3">
                  <div className="flex items-center gap-1">
                    <button
                      onClick={() => onBrowse(b.name)}
                      className="p-1.5 text-muted hover:text-accent rounded transition"
                      title="Browse objects"
                    >
                      <FolderOpen size={15} />
                    </button>
                    <button
                      onClick={() => setDeleteTarget(b)}
                      className="p-1.5 text-muted hover:text-red-400 rounded transition"
                      title="Delete bucket"
                    >
                      <Trash2 size={15} />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {showCreate && (
        <CreateBucketDialog
          onClose={() => setShowCreate(false)}
          onCreated={() => qc.invalidateQueries({ queryKey: ['buckets'] })}
        />
      )}
      {deleteTarget && (
        <DeleteConfirmDialog
          bucket={deleteTarget}
          onClose={() => setDeleteTarget(null)}
          onDeleted={() => qc.invalidateQueries({ queryKey: ['buckets'] })}
        />
      )}
    </div>
  )
}
