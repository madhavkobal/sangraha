interface BadgeProps {
  variant: 'success' | 'warning' | 'error' | 'muted' | 'info'
  children: React.ReactNode
}

const variantClasses: Record<BadgeProps['variant'], string> = {
  success: 'bg-green-900/40 text-green-400',
  warning: 'bg-yellow-900/40 text-yellow-400',
  error: 'bg-red-900/40 text-red-400',
  muted: 'bg-white/5 text-muted',
  info: 'bg-accent/10 text-accent',
}

export default function Badge({ variant, children }: BadgeProps) {
  return (
    <span className={`inline-flex items-center text-xs px-2 py-0.5 rounded-full font-medium ${variantClasses[variant]}`}>
      {children}
    </span>
  )
}
