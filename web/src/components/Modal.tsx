import { useEffect, useRef } from 'react'
import { X } from 'lucide-react'

interface ModalProps {
  title: string
  onClose: () => void
  children: React.ReactNode
  size?: 'sm' | 'md' | 'lg'
}

const sizeClasses: Record<NonNullable<ModalProps['size']>, string> = {
  sm: 'max-w-sm',
  md: 'max-w-md',
  lg: 'max-w-lg',
}

export default function Modal({ title, onClose, children, size = 'md' }: ModalProps) {
  const panelRef = useRef<HTMLDivElement>(null)

  // Close on Escape key
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onClose])

  // Focus trap: move focus into the panel on mount
  useEffect(() => {
    const panel = panelRef.current
    if (!panel) return

    const focusableSelectors =
      'a[href], button:not([disabled]), textarea, input, select, [tabindex]:not([tabindex="-1"])'
    const focusable = panel.querySelectorAll<HTMLElement>(focusableSelectors)
    const first = focusable[0]
    const last = focusable[focusable.length - 1]

    first?.focus()

    const trapTab = (e: KeyboardEvent) => {
      if (e.key !== 'Tab') return
      if (focusable.length === 0) { e.preventDefault(); return }
      if (e.shiftKey) {
        if (document.activeElement === first) { e.preventDefault(); last?.focus() }
      } else {
        if (document.activeElement === last) { e.preventDefault(); first?.focus() }
      }
    }

    panel.addEventListener('keydown', trapTab)
    return () => panel.removeEventListener('keydown', trapTab)
  }, [])

  return (
    <div
      className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4"
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
    >
      <div
        ref={panelRef}
        className={`bg-surface border border-border rounded-lg w-full ${sizeClasses[size]} shadow-xl`}
        role="dialog"
        aria-modal="true"
        aria-labelledby="modal-title"
      >
        <div className="flex items-center justify-between px-6 py-4 border-b border-border">
          <h2 id="modal-title" className="text-base font-semibold">{title}</h2>
          <button
            onClick={onClose}
            className="p-1 text-muted hover:text-white rounded transition"
            aria-label="Close"
          >
            <X size={16} />
          </button>
        </div>
        <div className="px-6 py-4">
          {children}
        </div>
      </div>
    </div>
  )
}
