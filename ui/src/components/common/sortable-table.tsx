"use client"

import React from "react"
import clsx from "clsx"
import { ArrowUpDown, ArrowUp, ArrowDown } from "lucide-react"

export type SortDirection = "asc" | "desc"

export type Column<T> = {
  id: string
  header: React.ReactNode
  cell: (row: T) => React.ReactNode
  accessor?: (row: T) => string | number | Date | null | undefined
  className?: string
  sortable?: boolean
}

export type SortState = {
  columnId: string
  direction: SortDirection
}

export function SortableTable<T extends object>({
  columns,
  data,
  initialSort,
  onSortChange,
  tableClassName,
  headerClassName,
  bodyClassName,
}: {
  columns: Column<T>[]
  data: T[]
  initialSort?: SortState
  onSortChange?: (state: SortState) => void
  tableClassName?: string
  headerClassName?: string
  bodyClassName?: string
}) {
  const [sort, setSort] = React.useState<SortState | undefined>(initialSort)

  const sortedData = React.useMemo(() => {
    if (!sort) return data
    const col = columns.find((c) => c.id === sort.columnId)
    if (!col || !col.accessor) return data

    const copy = [...data]
    copy.sort((a, b) => {
      const va = col.accessor!(a)
      const vb = col.accessor!(b)
      const toNum = (v: any): number => {
        if (v == null) return Number.NaN
        if (v instanceof Date) return v.getTime()
        if (typeof v === "string") {
          const d = Date.parse(v)
          if (!Number.isNaN(d)) return d
        }
        return Number(v)
      }

      const na = toNum(va)
      const nb = toNum(vb)

      if (Number.isNaN(na) || Number.isNaN(nb)) {
        // Fallback to string compare
        const sa = (va ?? "").toString()
        const sb = (vb ?? "").toString()
        if (sa < sb) return sort.direction === "asc" ? -1 : 1
        if (sa > sb) return sort.direction === "asc" ? 1 : -1
        return 0
      }

      return sort.direction === "asc" ? na - nb : nb - na
    })
    return copy
  }, [data, sort, columns])

  const toggleSort = (column: Column<T>) => {
    if (!column.sortable) return
    setSort((prev) => {
      if (!prev || prev.columnId !== column.id) {
        const next = { columnId: column.id, direction: "asc" as SortDirection }
        onSortChange?.(next)
        return next
      }
      const nextDir = prev.direction === "asc" ? "desc" : "asc"
      const next = { columnId: column.id, direction: nextDir }
      onSortChange?.(next)
      return next
    })
  }

  const sortIndicator = (columnId: string) => {
    if (!sort || sort.columnId !== columnId) return (
      <ArrowUpDown className="h-3.5 w-3.5 text-muted-foreground" />
    )
    return sort.direction === "asc"
      ? <ArrowUp className="h-3.5 w-3.5 text-foreground" />
      : <ArrowDown className="h-3.5 w-3.5 text-foreground" />
  }

  return (
    <div className="border rounded-lg overflow-hidden">
      <table className={clsx("w-full", tableClassName)}>
        <thead className={clsx("bg-muted/50", headerClassName)}>
          <tr>
            {columns.map((col) => (
              <th
                key={col.id}
                className={clsx("text-left p-4 font-medium select-none", col.className, col.sortable && "cursor-pointer")}
                onClick={() => toggleSort(col)}
              >
                <div className="inline-flex items-center gap-1.5">
                  <span>{col.header}</span>
                  {col.sortable && sortIndicator(col.id)}
                </div>
              </th>
            ))}
          </tr>
        </thead>
        <tbody className={bodyClassName}>
          {sortedData.map((row, idx) => (
            <tr key={idx} className="border-t hover:bg-muted/25">
              {columns.map((col) => (
                <td key={col.id} className={clsx("p-4", col.className)}>
                  {col.cell(row)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

