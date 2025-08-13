'use client'

import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from 'lucide-react'

export interface PaginationMeta {
  total: number
  limit: number
  offset: number
  page: number
  size: number
  total_pages: number
  has_next: boolean
  has_previous: boolean
}

export interface PaginationProps {
  pagination: PaginationMeta
  onPageChange: (page: number) => void
  onPageSizeChange: (size: number) => void
  className?: string
}

export function Pagination({ pagination, onPageChange, onPageSizeChange, className }: PaginationProps) {
  const { page, total_pages, has_next, has_previous, size, total } = pagination

  const pageSizeOptions = [10, 20, 50, 100]

  const handlePageChange = (newPage: number) => {
    if (newPage >= 1 && newPage <= total_pages) {
      onPageChange(newPage)
    }
  }

  const getVisiblePages = () => {
    const delta = 2
    const range = []
    const rangeWithDots = []

    for (let i = Math.max(2, page - delta); i <= Math.min(total_pages - 1, page + delta); i++) {
      range.push(i)
    }

    if (page - delta > 2) {
      rangeWithDots.push(1, '...')
    } else {
      rangeWithDots.push(1)
    }

    rangeWithDots.push(...range)

    if (page + delta < total_pages - 1) {
      rangeWithDots.push('...', total_pages)
    } else if (total_pages > 1) {
      rangeWithDots.push(total_pages)
    }

    return rangeWithDots
  }

  if (total === 0) {
    return null
  }

  const startItem = (page - 1) * size + 1
  const endItem = Math.min(page * size, total)

  const buttonBaseClass = "inline-flex items-center justify-center h-8 w-8 text-sm font-medium border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
  const buttonDefaultClass = "text-gray-700 bg-white border-gray-300 hover:bg-gray-50"
  const buttonActiveClass = "text-white bg-blue-600 border-blue-600 hover:bg-blue-700"

  return (
    <div className={`flex items-center justify-between ${className}`}>
      <div className="flex items-center space-x-2">
        <p className="text-sm text-gray-600">
          Showing {startItem} to {endItem} of {total} results
        </p>
      </div>

      <div className="flex items-center space-x-6">
        <div className="flex items-center space-x-2">
          <p className="text-sm font-medium text-gray-700">Rows per page</p>
          <select
            value={size.toString()}
            onChange={(e) => onPageSizeChange(parseInt(e.target.value))}
            className="h-8 w-16 px-2 text-sm border border-gray-300 rounded-md bg-white focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          >
            {pageSizeOptions.map((pageSize) => (
              <option key={pageSize} value={pageSize.toString()}>
                {pageSize}
              </option>
            ))}
          </select>
        </div>

        <div className="flex items-center space-x-2">
          <p className="text-sm font-medium text-gray-700">
            Page {page} of {total_pages}
          </p>
          <div className="flex items-center space-x-1">
            <button
              className={`${buttonBaseClass} ${buttonDefaultClass}`}
              onClick={() => handlePageChange(1)}
              disabled={!has_previous}
              title="Go to first page"
            >
              <ChevronsLeft className="h-4 w-4" />
            </button>
            <button
              className={`${buttonBaseClass} ${buttonDefaultClass}`}
              onClick={() => handlePageChange(page - 1)}
              disabled={!has_previous}
              title="Go to previous page"
            >
              <ChevronLeft className="h-4 w-4" />
            </button>

            {getVisiblePages().map((pageNum, index) => (
              <button
                key={index}
                className={`${buttonBaseClass} ${pageNum === page ? buttonActiveClass : buttonDefaultClass}`}
                onClick={() => typeof pageNum === 'number' && handlePageChange(pageNum)}
                disabled={typeof pageNum !== 'number'}
              >
                {pageNum}
              </button>
            ))}

            <button
              className={`${buttonBaseClass} ${buttonDefaultClass}`}
              onClick={() => handlePageChange(page + 1)}
              disabled={!has_next}
              title="Go to next page"
            >
              <ChevronRight className="h-4 w-4" />
            </button>
            <button
              className={`${buttonBaseClass} ${buttonDefaultClass}`}
              onClick={() => handlePageChange(total_pages)}
              disabled={!has_next}
              title="Go to last page"
            >
              <ChevronsRight className="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
