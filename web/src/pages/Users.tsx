import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, User } from '../api/client'
import { RotateCcw, Plus } from 'lucide-react'
import { Badge, Button, ConfirmDialog, CopyButton, Table } from '../components'
import type { Column } from '../components'

export default function Users() {
  const qc = useQueryClient()
  const [newOwner, setNewOwner] = useState('')
  const [flashMsg, setFlashMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [createdUser, setCreatedUser] = useState<User | null>(null)
  const [lastOp, setLastOp] = useState<'create' | 'rotate'>('create')
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
      setLastOp('create')
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
      setLastOp('rotate')
      setCreatedUser(user)
      setFlashMsg(null)
    },
    onError: (e: Error) => setFlashMsg({ type: 'error', text: e.message }),
  })

  const columns: Column<User>[] = [
    {
      key: 'access_key',
      header: 'Access Key',
      sortable: true,
      render: (u) => (
        <span className="flex items-center gap-1">
          <code className="font-mono text-xs">{u.access_key}</code>
          <CopyButton value={u.access_key} />
        </span>
      ),
    },
    {
      key: 'owner',
      header: 'Owner',
      sortable: true,
      render: (u) => <span>{u.owner}</span>,
    },
    {
      key: 'is_root',
      header: 'Role',
      render: (u) => (
        <Badge variant={u.is_root ? 'info' : 'muted'}>
          {u.is_root ? 'root' : 'user'}
        </Badge>
      ),
    },
    {
      key: 'actions',
      header: 'Actions',
      render: (u) => (
        <div className="flex items-center gap-1.5 justify-end">
          <Button
            variant="outline"
            size="sm"
            icon={<RotateCcw size={11} />}
            isLoading={rotateMut.isPending && rotateMut.variables === u.access_key}
            onClick={() => rotateMut.mutate(u.access_key)}
          >
            Rotate Key
          </Button>
          <Button
            variant="danger"
            size="sm"
            onClick={() => setConfirmDelete(u.access_key)}
          >
            Delete
          </Button>
        </div>
      ),
    },
  ]

  return (
    <div className="p-6 max-w-5xl mx-auto">
      <div className="flex items-center gap-3 mb-6">
        <h1 className="text-xl font-semibold flex-1">Users</h1>
        <input
          type="text"
          placeholder="Owner name…"
          value={newOwner}
          onChange={e => setNewOwner(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter' && newOwner) createMut.mutate(newOwner) }}
          className="bg-surface border border-border rounded px-3 py-1.5 text-sm text-white focus:outline-none focus:border-accent w-48"
        />
        <Button
          onClick={() => newOwner && createMut.mutate(newOwner)}
          disabled={!newOwner}
          isLoading={createMut.isPending}
          icon={<Plus size={14} />}
        >
          Create User
        </Button>
      </div>

      {flashMsg && (
        <div className={`rounded p-3 mb-4 text-sm ${flashMsg.type === 'success' ? 'bg-green-900/20 border border-success text-success' : 'bg-red-900/20 border border-danger text-danger'}`}>
          {flashMsg.text}
        </div>
      )}

      {createdUser && (
        <div className="bg-green-900/20 border border-success text-success rounded p-4 mb-4 text-sm">
          <div className="font-semibold mb-1">{lastOp === 'rotate' ? 'Key rotated!' : 'User created!'}</div>
          <div className="flex items-center gap-2">
            Access Key: <code className="bg-black/30 px-1 rounded">{createdUser.access_key}</code>
            <CopyButton value={createdUser.access_key} label="access key" />
          </div>
          {createdUser.secret_key && (
            <div className="mt-1 flex items-center gap-2">
              Secret Key: <code className="bg-black/30 px-1 rounded">{createdUser.secret_key}</code>
              <CopyButton value={createdUser.secret_key} label="secret key" />
            </div>
          )}
          <div className="text-warn mt-2 text-xs">Save the secret key — it will not be shown again.</div>
          <button onClick={() => setCreatedUser(null)} className="mt-2 text-xs underline text-muted">Dismiss</button>
        </div>
      )}

      {error && (
        <div className="text-danger text-sm mb-4">{(error as Error).message}</div>
      )}

      <Table<User>
        columns={columns}
        rows={users ?? []}
        keyExtractor={u => u.access_key}
        isLoading={isLoading}
        emptyMessage="No users found"
      />

      {confirmDelete && (
        <ConfirmDialog
          title="Delete User"
          message={
            <span>
              Access key <code className="text-white font-mono">{confirmDelete}</code> will be permanently removed.
              This action cannot be undone.
            </span>
          }
          confirmText={confirmDelete}
          confirmLabel="Delete"
          onConfirm={() => deleteMut.mutate(confirmDelete)}
          onClose={() => setConfirmDelete(null)}
          isPending={deleteMut.isPending}
          danger
        />
      )}
    </div>
  )
}
