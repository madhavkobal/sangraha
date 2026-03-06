import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, User } from '../api/client'
import { Trash2, RotateCcw, Plus } from 'lucide-react'

export default function Users() {
  const qc = useQueryClient()
  const [newOwner, setNewOwner] = useState('')
  const [flashMsg, setFlashMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [createdUser, setCreatedUser] = useState<User | null>(null)
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)

  const { data: users, isLoading, error } = useQuery({
    queryKey: ['users'],
    queryFn: api.users.list,
  })

  const createMut = useMutation({
    mutationFn: (owner: string) => api.users.create({ owner }),
    onSuccess: (user) => {
      qc.invalidateQueries({ queryKey: ['users'] })
      setNewOwner('')
      setCreatedUser(user)
      setFlashMsg(null)
    },
    onError: (e: Error) => setFlashMsg({ type: 'error', text: e.message }),
  })

  const deleteMut = useMutation({
    mutationFn: (ak: string) => api.users.delete(ak),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['users'] })
      setConfirmDelete(null)
      setFlashMsg({ type: 'success', text: 'User deleted.' })
    },
    onError: (e: Error) => setFlashMsg({ type: 'error', text: e.message }),
  })

  const rotateMut = useMutation({
    mutationFn: (ak: string) => api.users.rotateKey(ak),
    onSuccess: (user) => {
      qc.invalidateQueries({ queryKey: ['users'] })
      setCreatedUser(user)
      setFlashMsg(null)
    },
    onError: (e: Error) => setFlashMsg({ type: 'error', text: e.message }),
  })

  return (
    <div className="p-6 max-w-5xl mx-auto">
      <div className="flex items-center gap-3 mb-6">
        <h1 className="text-xl font-semibold flex-1">Users</h1>
        <input
          type="text"
          placeholder="Owner name…"
          value={newOwner}
          onChange={e => setNewOwner(e.target.value)}
          className="bg-surface border border-border rounded px-3 py-1.5 text-sm text-white focus:outline-none focus:border-accent w-48"
        />
        <button
          onClick={() => newOwner && createMut.mutate(newOwner)}
          disabled={!newOwner || createMut.isPending}
          className="flex items-center gap-1.5 bg-accent text-bg text-sm font-semibold px-3 py-1.5 rounded hover:opacity-90 disabled:opacity-50"
        >
          <Plus size={14} /> Create User
        </button>
      </div>

      {flashMsg && (
        <div className={`rounded p-3 mb-4 text-sm ${flashMsg.type === 'success' ? 'bg-green-900/20 border border-success text-success' : 'bg-red-900/20 border border-danger text-danger'}`}>
          {flashMsg.text}
        </div>
      )}

      {createdUser && (
        <div className="bg-green-900/20 border border-success text-success rounded p-4 mb-4 text-sm">
          <div className="font-semibold mb-1">{createdUser.secret_key ? 'User created!' : 'Key rotated!'}</div>
          <div>Access Key: <code className="bg-black/30 px-1 rounded">{createdUser.access_key}</code></div>
          {createdUser.secret_key && (
            <div className="mt-1">Secret Key: <code className="bg-black/30 px-1 rounded">{createdUser.secret_key}</code></div>
          )}
          <div className="text-warn mt-2 text-xs">Save the secret key — it will not be shown again.</div>
          <button onClick={() => setCreatedUser(null)} className="mt-2 text-xs underline text-muted">Dismiss</button>
        </div>
      )}

      {isLoading ? (
        <div className="text-muted text-sm">Loading…</div>
      ) : error ? (
        <div className="text-danger text-sm">{(error as Error).message}</div>
      ) : (
        <div className="bg-surface border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border">
                <th className="px-4 py-2.5 text-left text-xs text-muted uppercase tracking-wide">Access Key</th>
                <th className="px-4 py-2.5 text-left text-xs text-muted uppercase tracking-wide">Owner</th>
                <th className="px-4 py-2.5 text-left text-xs text-muted uppercase tracking-wide">Role</th>
                <th className="px-4 py-2.5 text-right text-xs text-muted uppercase tracking-wide">Actions</th>
              </tr>
            </thead>
            <tbody>
              {!users?.length ? (
                <tr><td colSpan={4} className="px-4 py-8 text-center text-muted">No users found</td></tr>
              ) : (
                users.map(u => (
                  <tr key={u.access_key} className="border-b border-border last:border-0 hover:bg-white/[0.02]">
                    <td className="px-4 py-2.5 font-mono text-xs">{u.access_key}</td>
                    <td className="px-4 py-2.5">{u.owner}</td>
                    <td className="px-4 py-2.5">
                      <span className={`text-xs px-2 py-0.5 rounded font-semibold ${u.is_root ? 'bg-accent/10 text-accent' : 'bg-muted/10 text-muted'}`}>
                        {u.is_root ? 'root' : 'user'}
                      </span>
                    </td>
                    <td className="px-4 py-2.5 text-right space-x-2">
                      <button
                        onClick={() => rotateMut.mutate(u.access_key)}
                        className="text-xs px-2 py-1 rounded border border-border text-muted hover:text-white transition inline-flex items-center gap-1"
                      >
                        <RotateCcw size={11} /> Rotate Key
                      </button>
                      <button
                        onClick={() => setConfirmDelete(u.access_key)}
                        className="text-xs px-2 py-1 rounded border border-danger/40 text-danger hover:bg-danger/10 transition inline-flex items-center gap-1"
                      >
                        <Trash2 size={11} /> Delete
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* Confirmation modal */}
      {confirmDelete && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-surface border border-border rounded-lg p-6 w-80">
            <h2 className="font-semibold mb-2">Delete user?</h2>
            <p className="text-muted text-sm mb-4">Access key <code className="text-white">{confirmDelete}</code> will be permanently removed.</p>
            <div className="flex gap-2">
              <button
                onClick={() => deleteMut.mutate(confirmDelete)}
                className="flex-1 bg-danger/10 border border-danger text-danger rounded px-3 py-1.5 text-sm hover:bg-danger/20"
              >
                Delete
              </button>
              <button
                onClick={() => setConfirmDelete(null)}
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
