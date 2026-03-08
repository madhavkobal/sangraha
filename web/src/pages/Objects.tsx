import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ObjectRecord } from '../api/client'
import { Trash2, Download, RefreshCw, ChevronRight, FolderClosed, File } from 'lucide-react'

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString()
}

interface DeleteConfirmProps {
  bucket: string
  key: string
  onClose: () => void
  onDeleted: () => void
}

function DeleteConfirmDialog({ bucket, key, onClose, onDeleted }: DeleteConfirmProps) {
  const [input, setInput] = useState('')
  const [error, setError] = useState('')
  const shortKey = key.split('/').pop() ?? key

  const mutation = useMutation({
    mutationFn: () => api.buckets.deleteObject(bucket, key),
    onSuccess: () => { onDeleted(); onClose() },
    onError: (err: Error) => setError(err.message),
  })

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
      <div className="bg-surface border border-border rounded-lg p-6 w-full max-w-md">
        <h2 className="text-lg font-semibold mb-2">Delete Object</h2>
        <p className="text-muted text-sm mb-4">
          Type <span className="text-white font-mono">{shortKey}</span> to confirm deletion.
        </p>
        <input
          className="w-full bg-bg border border-border rounded px-3 py-2 text-sm text-white focus:outline-none focus:border-red-500 mb-3"
          value={input}
          onChange={e => setInput(e.target.value)}
          placeholder={shortKey}
          autoFocus
        />
        {error && <p className="text-red-400 text-sm mb-3">{error}</p>}
        <div className="flex justify-end gap-2">
          <button onClick={onClose} className="px-4 py-2 text-sm text-muted hover:text-white rounded transition">Cancel</button>
          <button
            onClick={() => mutation.mutate()}
            disabled={input !== shortKey || mutation.isPending}
            className="px-4 py-2 text-sm bg-red-600 text-white rounded hover:bg-red-700 disabled:opacity-50 transition"
          >
            {mutation.isPending ? 'Deleting…' : 'Delete'}
          </button>
        </div>
      </div>
    </div>
  )
}

interface ObjectsProps {
  bucket: string
}

export default function Objects({ bucket }: ObjectsProps) {
  const qc = useQueryClient()
  const [prefix, setPrefix] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)
  const [selectedKeys, setSelectedKeys] = useState<Set<string>>(new Set())

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['objects', bucket, prefix],
    queryFn: () => api.buckets.listObjects(bucket, prefix),
    refetchInterval: 60_000,
  })

  const objects = data?.objects ?? []
  const prefixes = data?.prefixes ?? []

  // Build breadcrumb segments from the current prefix.
  const breadcrumbs = prefix
    ? prefix.split('/').filter(Boolean)
    : []

  const navigateTo = (seg: string) => setPrefix(seg ? seg + '/' : '')

  const toggleSelect = (key: string) => {
    setSelectedKeys(prev => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  const bulkDeleteMutation = useMutation({
    mutationFn: async () => {
      for (const key of selectedKeys) {
        await api.buckets.deleteObject(bucket, key)
      }
    },
    onSuccess: () => {
      setSelectedKeys(new Set())
      qc.invalidateQueries({ queryKey: ['objects', bucket] })
    },
  })

  const handleDownload = (obj: ObjectRecord) => {
    // Open a presigned GET URL or fall back to the admin API URL with the key path.
    // For now, we construct the admin object URL (not directly downloadable without auth);
    // in a real deployment this would use a presigned URL.
    const url = `/admin/v1/buckets/${bucket}/objects/${obj.key}`
    window.open(url, '_blank')
  }

  return (
    <div className="p-6">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div>
          <h1 className="text-xl font-semibold">Objects</h1>
          <div className="flex items-center gap-1 text-sm text-muted mt-0.5">
            <button onClick={() => navigateTo('')} className="hover:text-white transition font-mono">
              {bucket}
            </button>
            {breadcrumbs.map((seg, i) => {
              const path = breadcrumbs.slice(0, i + 1).join('/') + '/'
              return (
                <span key={path} className="flex items-center gap-1">
                  <ChevronRight size={13} />
                  <button onClick={() => navigateTo(path)} className="hover:text-white transition font-mono">
                    {seg}
                  </button>
                </span>
              )
            })}
          </div>
        </div>
        <div className="flex items-center gap-2">
          {selectedKeys.size > 0 && (
            <button
              onClick={() => {
                if (confirm(`Delete ${selectedKeys.size} object(s)?`)) bulkDeleteMutation.mutate()
              }}
              className="flex items-center gap-1.5 px-3 py-2 text-sm bg-red-700 text-white rounded hover:bg-red-800 transition"
            >
              <Trash2 size={14} /> Delete {selectedKeys.size}
            </button>
          )}
          <button onClick={() => refetch()} className="p-2 text-muted hover:text-white rounded transition" title="Refresh">
            <RefreshCw size={16} />
          </button>
        </div>
      </div>

      {isLoading && <p className="text-muted text-sm">Loading…</p>}
      {error && <p className="text-red-400 text-sm">{(error as Error).message}</p>}

      {!isLoading && objects.length === 0 && prefixes.length === 0 && (
        <p className="text-muted text-sm text-center py-12">No objects in this prefix.</p>
      )}

      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-muted text-left">
              <th className="pb-2 pr-3 w-6"><input type="checkbox" className="accent-accent" onChange={e => {
                if (e.target.checked) setSelectedKeys(new Set(objects.map(o => o.key)))
                else setSelectedKeys(new Set())
              }} /></th>
              <th className="pb-2 pr-4 font-medium">Name</th>
              <th className="pb-2 pr-4 font-medium">Size</th>
              <th className="pb-2 pr-4 font-medium">Last Modified</th>
              <th className="pb-2 pr-4 font-medium">Content Type</th>
              <th className="pb-2 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {/* Common prefixes (virtual folders) */}
            {prefixes.map(p => {
              const label = p.slice(prefix.length).replace(/\/$/, '')
              return (
                <tr key={p} className="border-b border-border/40 hover:bg-white/[0.02] transition">
                  <td className="py-3 pr-3" />
                  <td className="py-3 pr-4" colSpan={4}>
                    <button
                      className="flex items-center gap-2 text-accent hover:underline font-mono"
                      onClick={() => navigateTo(p)}
                    >
                      <FolderClosed size={14} /> {label}/
                    </button>
                  </td>
                  <td />
                </tr>
              )
            })}

            {/* Objects */}
            {objects.map(obj => {
              const label = obj.key.slice(prefix.length)
              return (
                <tr key={obj.key} className="border-b border-border/40 hover:bg-white/[0.02] transition">
                  <td className="py-3 pr-3">
                    <input
                      type="checkbox"
                      className="accent-accent"
                      checked={selectedKeys.has(obj.key)}
                      onChange={() => toggleSelect(obj.key)}
                    />
                  </td>
                  <td className="py-3 pr-4 font-mono">
                    <span className="flex items-center gap-2">
                      <File size={13} className="text-muted shrink-0" />
                      {label}
                    </span>
                  </td>
                  <td className="py-3 pr-4 text-muted">{formatBytes(obj.size)}</td>
                  <td className="py-3 pr-4 text-muted">{formatDate(obj.last_modified)}</td>
                  <td className="py-3 pr-4 text-muted text-xs">{obj.content_type || '—'}</td>
                  <td className="py-3">
                    <div className="flex items-center gap-1">
                      <button
                        onClick={() => handleDownload(obj)}
                        className="p-1.5 text-muted hover:text-accent rounded transition"
                        title="Download"
                      >
                        <Download size={14} />
                      </button>
                      <button
                        onClick={() => setDeleteTarget(obj.key)}
                        className="p-1.5 text-muted hover:text-red-400 rounded transition"
                        title="Delete"
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>

      {deleteTarget && (
        <DeleteConfirmDialog
          bucket={bucket}
          key={deleteTarget}
          onClose={() => setDeleteTarget(null)}
          onDeleted={() => {
            qc.invalidateQueries({ queryKey: ['objects', bucket] })
            setDeleteTarget(null)
          }}
        />
      )}
    </div>
  )
}
