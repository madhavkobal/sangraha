import { useState } from 'react'
import { ChevronUp, ChevronDown } from 'lucide-react'

export interface Column<T> {
  key: keyof T | string
  header: string
  render?: (row: T) => React.ReactNode
  sortable?: boolean
}

interface TableProps<T> {
  columns: Column<T>[]
  rows: T[]
  keyExtractor: (row: T) => string
  isLoading?: boolean
  emptyMessage?: string
}

type SortDir = 'asc' | 'desc'

export default function Table<T extends Record<string, unknown>>({
  columns,
  rows,
  keyExtractor,
  isLoading = false,
  emptyMessage = 'No data.',
}: TableProps<T>) {
  const [sortKey, setSortKey] = useState<string | null>(null)
  const [sortDir, setSortDir] = useState<SortDir>('asc')

  const handleSort = (key: string) => {
    if (sortKey === key) {
      setSortDir(d => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortKey(key)
      setSortDir('asc')
    }
  }

  const sorted = sortKey
    ? [...rows].sort((a, b) => {
        const av = a[sortKey]
        const bv = b[sortKey]
        if (av == null && bv == null) return 0
        if (av == null) return 1
        if (bv == null) return -1
        const cmp = av < bv ? -1 : av > bv ? 1 : 0
        return sortDir === 'asc' ? cmp : -cmp
      })
    : rows

  return (
    <div className="bg-surface border border-border rounded-lg overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border">
              {columns.map(col => (
                <th
                  key={String(col.key)}
                  className={[
                    'px-4 py-2.5 text-left text-xs text-muted uppercase tracking-wide select-none',
                    col.sortable ? 'cursor-pointer hover:text-white transition' : '',
                  ].join(' ')}
                  onClick={col.sortable ? () => handleSort(String(col.key)) : undefined}
                >
                  <span className="inline-flex items-center gap-1">
                    {col.header}
                    {col.sortable && sortKey === String(col.key) && (
                      sortDir === 'asc' ? <ChevronUp size={12} /> : <ChevronDown size={12} />
                    )}
                  </span>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr>
                <td colSpan={columns.length} className="px-4 py-8 text-center text-muted">
                  Loading…
                </td>
              </tr>
            ) : sorted.length === 0 ? (
              <tr>
                <td colSpan={columns.length} className="px-4 py-8 text-center text-muted">
                  {emptyMessage}
                </td>
              </tr>
            ) : (
              sorted.map(row => (
                <tr
                  key={keyExtractor(row)}
                  className="border-b border-border last:border-0 hover:bg-white/[0.02] transition"
                >
                  {columns.map(col => (
                    <td key={String(col.key)} className="px-4 py-2.5">
                      {col.render
                        ? col.render(row)
                        : String(row[col.key as string] ?? '—')}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
