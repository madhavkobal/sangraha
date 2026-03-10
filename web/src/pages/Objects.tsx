import { useState, useCallback, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ObjectRecord } from '../api/client'
import {
  Trash2, Download, RefreshCw, ChevronRight, FolderClosed, File,
  Upload, X, Info,
} from 'lucide-react'
import { Button, ConfirmDialog, CopyButton, EmptyState } from '../components'

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

// ---- Upload Drop Zone --------------------------------------------------------

interface DropZoneProps {
  bucket: string
  prefix: string
  onUploaded: () => void
}

function DropZone({ bucket, prefix, onUploaded }: DropZoneProps) {
  const [dragging, setDragging] = useState(false)
  const [uploads, setUploads] = useState<{ name: string; progress: number; error?: string }[]>([])
  const inputRef = useRef<HTMLInputElement>(null)

  const uploadFile = useCallback(async (file: File) => {
    const key = prefix + file.name
    setUploads(prev => [...prev, { name: file.name, progress: 0 }])

    try {
      // Use XMLHttpRequest for progress tracking
      await new Promise<void>((resolve, reject) => {
        const xhr = new XMLHttpRequest()
        xhr.upload.addEventListener('progress', e => {
          if (e.lengthComputable) {
            const pct = Math.round((e.loaded / e.total) * 100)
            setUploads(prev => prev.map(u => u.name === file.name ? { ...u, progress: pct } : u))
          }
        })
        xhr.addEventListener('load', () => {
          if (xhr.status >= 200 && xhr.status < 300) resolve()
          else reject(new Error(`HTTP ${xhr.status}`))
        })
        xhr.addEventListener('error', () => reject(new Error('Network error')))
        xhr.open('PUT', `/admin/v1/buckets/${bucket}/objects/${encodeURIComponent(key)}`)
        xhr.setRequestHeader('Content-Type', file.type || 'application/octet-stream')
        xhr.send(file)
      })

      setUploads(prev => prev.map(u => u.name === file.name ? { ...u, progress: 100 } : u))
      onUploaded()
      // Remove from list after a short delay
      setTimeout(() => setUploads(prev => prev.filter(u => u.name !== file.name)), 2000)
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Upload failed'
      setUploads(prev => prev.map(u => u.name === file.name ? { ...u, error: msg } : u))
    }
  }, [bucket, prefix, onUploaded])

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    setDragging(false)
    const files = Array.from(e.dataTransfer.files)
    files.forEach(uploadFile)
  }, [uploadFile])

  const handleFileInput = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files ?? [])
    files.forEach(uploadFile)
    // reset input so same file can be re-uploaded
    e.target.value = ''
  }

  return (
    <div className="mb-4">
      <div
        onDragOver={e => { e.preventDefault(); setDragging(true) }}
        onDragLeave={() => setDragging(false)}
        onDrop={handleDrop}
        onClick={() => inputRef.current?.click()}
        className={[
          'border-2 border-dashed rounded-lg p-6 flex flex-col items-center justify-center gap-2 cursor-pointer transition',
          dragging ? 'border-accent bg-accent/5' : 'border-border hover:border-accent/50 hover:bg-white/[0.02]',
        ].join(' ')}
        role="button"
        aria-label="Upload files"
      >
        <Upload size={20} className={dragging ? 'text-accent' : 'text-muted'} />
        <p className="text-sm text-muted">
          {dragging ? 'Drop files here' : 'Drag & drop files here, or click to select'}
        </p>
        <input
          ref={inputRef}
          type="file"
          multiple
          className="hidden"
          onChange={handleFileInput}
          onClick={e => e.stopPropagation()}
        />
      </div>

      {uploads.length > 0 && (
        <div className="mt-2 space-y-1">
          {uploads.map(u => (
            <div key={u.name} className="flex items-center gap-2 text-xs">
              <span className="text-muted truncate max-w-xs">{u.name}</span>
              {u.error ? (
                <span className="text-danger">{u.error}</span>
              ) : (
                <>
                  <div className="flex-1 bg-border rounded-full h-1 overflow-hidden">
                    <div
                      className="bg-accent h-1 transition-all"
                      style={{ width: `${u.progress}%` }}
                    />
                  </div>
                  <span className="text-muted w-8 text-right">{u.progress}%</span>
                </>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// ---- Object Detail Sidebar ---------------------------------------------------

interface DetailSidebarProps {
  obj: ObjectRecord
  bucket: string
  onClose: () => void
}

function DetailSidebar({ obj, bucket, onClose }: DetailSidebarProps) {
  // Build a presigned-style URL (in practice this would be a real presigned URL endpoint).
  // For now we provide the admin API URL which can be copied.
  const objectUrl = `${window.location.origin}/admin/v1/buckets/${bucket}/objects/${encodeURIComponent(obj.key)}`

  const rows: { label: string; value: React.ReactNode }[] = [
    { label: 'Key', value: <span className="font-mono text-xs break-all">{obj.key}</span> },
    { label: 'Size', value: formatBytes(obj.size) },
    { label: 'Last Modified', value: formatDate(obj.last_modified) },
    { label: 'Content Type', value: obj.content_type || '—' },
    { label: 'ETag', value: <span className="font-mono text-xs">{obj.etag}</span> },
    { label: 'Owner', value: obj.owner || '—' },
    { label: 'Storage Class', value: obj.storage_class || '—' },
    ...(obj.version_id ? [{ label: 'Version ID', value: <span className="font-mono text-xs">{obj.version_id}</span> }] : []),
  ]

  const tagEntries = obj.tags ? Object.entries(obj.tags) : []

  return (
    <div className="flex flex-col h-full">
      {/* Sidebar header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-border shrink-0">
        <div className="flex items-center gap-2 min-w-0">
          <File size={14} className="text-muted shrink-0" />
          <span className="text-sm font-medium truncate">{obj.key.split('/').pop() ?? obj.key}</span>
        </div>
        <button onClick={onClose} className="p-1 text-muted hover:text-white rounded transition shrink-0" aria-label="Close detail panel">
          <X size={15} />
        </button>
      </div>

      {/* Presigned URL copy */}
      <div className="px-4 py-3 border-b border-border shrink-0">
        <p className="text-xs text-muted mb-1">Object URL</p>
        <div className="flex items-center gap-1 bg-bg border border-border rounded px-2 py-1.5">
          <span className="text-xs text-muted truncate flex-1 font-mono">{objectUrl}</span>
          <CopyButton value={objectUrl} label="URL" />
        </div>
      </div>

      {/* Metadata table */}
      <div className="flex-1 overflow-y-auto px-4 py-3">
        <p className="text-xs text-muted uppercase tracking-wide mb-2">Metadata</p>
        <dl className="space-y-2">
          {rows.map(r => (
            <div key={r.label} className="flex gap-2">
              <dt className="text-xs text-muted w-28 shrink-0">{r.label}</dt>
              <dd className="text-xs text-white flex-1">{r.value}</dd>
            </div>
          ))}
        </dl>

        {tagEntries.length > 0 && (
          <div className="mt-4">
            <p className="text-xs text-muted uppercase tracking-wide mb-2">Tags</p>
            <dl className="space-y-1">
              {tagEntries.map(([k, v]) => (
                <div key={k} className="flex gap-2">
                  <dt className="text-xs text-muted w-28 shrink-0 truncate">{k}</dt>
                  <dd className="text-xs text-white flex-1">{v}</dd>
                </div>
              ))}
            </dl>
          </div>
        )}
      </div>
    </div>
  )
}

// ---- Main component ----------------------------------------------------------

interface ObjectsProps {
  bucket: string
}

export default function Objects({ bucket }: ObjectsProps) {
  const qc = useQueryClient()
  const [prefix, setPrefix] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)
  const [selectedKeys, setSelectedKeys] = useState<Set<string>>(new Set())
  const [detailObj, setDetailObj] = useState<ObjectRecord | null>(null)
  const [showUpload, setShowUpload] = useState(false)

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['objects', bucket, prefix],
    queryFn: () => api.buckets.listObjects(bucket, prefix),
    refetchInterval: 60_000,
  })

  const objects = data?.objects ?? []
  const prefixes = data?.prefixes ?? []

  const breadcrumbs = prefix
    ? prefix.split('/').filter(Boolean)
    : []

  const navigateTo = (seg: string) => {
    setPrefix(seg ? seg + '/' : '')
    setDetailObj(null)
    setSelectedKeys(new Set())
  }

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

  const deleteSingleMutation = useMutation({
    mutationFn: (key: string) => api.buckets.deleteObject(bucket, key),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['objects', bucket] })
      if (detailObj?.key === deleteTarget) setDetailObj(null)
      setDeleteTarget(null)
    },
  })

  const handleDownload = (obj: ObjectRecord) => {
    const url = `/admin/v1/buckets/${bucket}/objects/${encodeURIComponent(obj.key)}`
    window.open(url, '_blank')
  }

  const hasRows = objects.length > 0 || prefixes.length > 0

  return (
    <div className={`flex h-full ${detailObj ? 'divide-x divide-border' : ''}`}>
      {/* Main panel */}
      <div className="flex-1 min-w-0 p-6 overflow-auto">
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
              <Button
                variant="danger"
                size="sm"
                icon={<Trash2 size={14} />}
                isLoading={bulkDeleteMutation.isPending}
                onClick={() => {
                  if (confirm(`Delete ${selectedKeys.size} object(s)?`)) bulkDeleteMutation.mutate()
                }}
              >
                Delete {selectedKeys.size}
              </Button>
            )}
            <Button
              variant="outline"
              size="sm"
              icon={<Upload size={14} />}
              onClick={() => setShowUpload(v => !v)}
            >
              {showUpload ? 'Hide Upload' : 'Upload'}
            </Button>
            <button
              onClick={() => refetch()}
              className="p-2 text-muted hover:text-white rounded transition"
              title="Refresh"
            >
              <RefreshCw size={16} />
            </button>
          </div>
        </div>

        {/* Upload drop zone (toggle) */}
        {showUpload && (
          <DropZone
            bucket={bucket}
            prefix={prefix}
            onUploaded={() => qc.invalidateQueries({ queryKey: ['objects', bucket] })}
          />
        )}

        {isLoading && <p className="text-muted text-sm">Loading…</p>}
        {error && <p className="text-red-400 text-sm">{(error as Error).message}</p>}

        {!isLoading && !hasRows && (
          <EmptyState
            icon={<File size={40} />}
            title="No objects here"
            description="This prefix is empty. Upload files to get started."
            action={
              <Button
                variant="primary"
                icon={<Upload size={14} />}
                onClick={() => setShowUpload(true)}
              >
                Upload files
              </Button>
            }
          />
        )}

        {hasRows && (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-muted text-left">
                  <th className="pb-2 pr-3 w-6">
                    <input
                      type="checkbox"
                      className="accent-accent"
                      onChange={e => {
                        if (e.target.checked) setSelectedKeys(new Set(objects.map(o => o.key)))
                        else setSelectedKeys(new Set())
                      }}
                    />
                  </th>
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
                  const isSelected = selectedKeys.has(obj.key)
                  const isActive = detailObj?.key === obj.key
                  return (
                    <tr
                      key={obj.key}
                      className={`border-b border-border/40 hover:bg-white/[0.02] transition ${isActive ? 'bg-white/[0.04]' : ''}`}
                    >
                      <td className="py-3 pr-3">
                        <input
                          type="checkbox"
                          className="accent-accent"
                          checked={isSelected}
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
                            onClick={() => setDetailObj(isActive ? null : obj)}
                            className={`p-1.5 rounded transition ${isActive ? 'text-accent' : 'text-muted hover:text-accent'}`}
                            title="View details"
                          >
                            <Info size={14} />
                          </button>
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
        )}
      </div>

      {/* Detail sidebar */}
      {detailObj && (
        <div className="w-72 shrink-0 overflow-hidden">
          <DetailSidebar
            obj={detailObj}
            bucket={bucket}
            onClose={() => setDetailObj(null)}
          />
        </div>
      )}

      {/* Delete confirm dialog */}
      {deleteTarget && (
        <ConfirmDialog
          title="Delete Object"
          message={
            <span>
              Type <span className="text-white font-mono">{deleteTarget.split('/').pop() ?? deleteTarget}</span> to confirm deletion.
              This action cannot be undone.
            </span>
          }
          confirmText={deleteTarget.split('/').pop() ?? deleteTarget}
          confirmLabel="Delete"
          onConfirm={() => deleteSingleMutation.mutate(deleteTarget)}
          onClose={() => setDeleteTarget(null)}
          isPending={deleteSingleMutation.isPending}
          danger
        />
      )}
    </div>
  )
}
