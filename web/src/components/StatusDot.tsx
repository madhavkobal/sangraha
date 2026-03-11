interface StatusDotProps {
  status: 'online' | 'offline' | 'warning'
  label?: string
}

const dotClasses: Record<StatusDotProps['status'], string> = {
  online: 'bg-green-400',
  offline: 'bg-red-400',
  warning: 'bg-yellow-400',
}

export default function StatusDot({ status, label }: StatusDotProps) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className={`inline-block w-2 h-2 rounded-full ${dotClasses[status]}`} aria-hidden="true" />
      {label && <span className="text-sm text-muted">{label}</span>}
    </span>
  )
}
