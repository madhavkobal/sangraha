interface EmptyStateProps {
  icon?: React.ReactNode
  title: string
  description?: string
  action?: React.ReactNode
}

export default function EmptyState({ icon, title, description, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center gap-3">
      {icon && (
        <div className="text-muted/40 mb-1">
          {icon}
        </div>
      )}
      <p className="text-base font-medium text-white">{title}</p>
      {description && (
        <p className="text-sm text-muted max-w-sm">{description}</p>
      )}
      {action && <div className="mt-2">{action}</div>}
    </div>
  )
}
