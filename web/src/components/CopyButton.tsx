import { useState } from 'react'
import { Copy, Check } from 'lucide-react'

interface CopyButtonProps {
  value: string
  label?: string
}

export default function CopyButton({ value, label }: CopyButtonProps) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // clipboard API unavailable — silently ignore
    }
  }

  return (
    <button
      onClick={handleCopy}
      title={copied ? 'Copied!' : `Copy${label ? ` ${label}` : ''}`}
      className="inline-flex items-center gap-1 text-xs text-muted hover:text-white rounded px-1.5 py-0.5 transition"
      aria-label={copied ? 'Copied' : 'Copy to clipboard'}
    >
      {copied ? <Check size={12} className="text-green-400" /> : <Copy size={12} />}
      {label && <span>{copied ? 'Copied' : label}</span>}
    </button>
  )
}
