import { useState } from 'react'
import Modal from './Modal'
import Button from './Button'

interface ConfirmDialogProps {
  title: string
  message: React.ReactNode
  /** The user must type this exact string to enable the confirm button. */
  confirmText: string
  /** Label for the confirm button. Defaults to "Confirm". */
  confirmLabel?: string
  onConfirm: () => void
  onClose: () => void
  isPending?: boolean
  /** When true the confirm button renders in red (destructive action). */
  danger?: boolean
}

export default function ConfirmDialog({
  title,
  message,
  confirmText,
  confirmLabel = 'Confirm',
  onConfirm,
  onClose,
  isPending = false,
  danger = false,
}: ConfirmDialogProps) {
  const [input, setInput] = useState('')
  const isMatch = input === confirmText

  return (
    <Modal title={title} onClose={onClose} size="md">
      <div className="space-y-4">
        <div className="text-muted text-sm">{message}</div>

        <div>
          <label className="block text-xs text-muted mb-1">
            Type <span className="text-white font-mono">{confirmText}</span> to confirm
          </label>
          <input
            className="w-full bg-bg border border-border rounded px-3 py-2 text-sm text-white focus:outline-none focus:border-accent"
            value={input}
            onChange={e => setInput(e.target.value)}
            placeholder={confirmText}
            autoFocus
            aria-label="Confirm text"
          />
        </div>

        <div className="flex justify-end gap-2 pt-1">
          <Button variant="ghost" onClick={onClose} disabled={isPending}>
            Cancel
          </Button>
          <Button
            variant={danger ? 'danger' : 'primary'}
            onClick={onConfirm}
            disabled={!isMatch || isPending}
            isLoading={isPending}
          >
            {confirmLabel}
          </Button>
        </div>
      </div>
    </Modal>
  )
}
