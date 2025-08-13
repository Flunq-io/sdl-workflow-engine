'use client'

import { useState, useEffect, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams, useSearchParams, useRouter } from 'next/navigation'
import { createTenantApiClient } from '@/lib/api'
import type { PaginatedResponse, Execution } from '@/lib/api'
import { ExecutionList } from '@/components/execution-list'
import { ClientOnly } from '@/components/client-only'
import { Pagination } from '@/components/pagination'
import { Filters, type FilterField, type FilterValue } from '@/components/filters'
import { Loader2, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useTranslations } from 'next-intl'

// Define filter fields for executions
const filterFields: FilterField[] = [
  {
    key: 'search',
    label: 'Search',
    type: 'text',
    placeholder: 'Search executions...'
  },
  {
    key: 'status',
    label: 'Status',
    type: 'select',
    options: [
      { value: 'pending', label: 'Pending' },
      { value: 'running', label: 'Running' },
      { value: 'waiting', label: 'Waiting' },
      { value: 'suspended', label: 'Suspended' },
      { value: 'completed', label: 'Completed' },
      { value: 'failed', label: 'Failed' },
      { value: 'cancelled', label: 'Cancelled' }
    ]
  },
  {
    key: 'workflow_id',
    label: 'Workflow ID',
    type: 'text',
    placeholder: 'Filter by workflow ID...'
  },
  {
    key: 'correlation_id',
    label: 'Correlation ID',
    type: 'text',
    placeholder: 'Filter by correlation ID...'
  },
  {
    key: 'started_at',
    label: 'Started Date',
    type: 'daterange'
  },
  {
    key: 'completed_at',
    label: 'Completed Date',
    type: 'daterange'
  }
]

export default function ExecutionsPage() {
  const t = useTranslations()
  const params = useParams<{ tenant: string; locale: string }>()
  const searchParams = useSearchParams()
  const router = useRouter()
  const tenant = String(params.tenant)
  const locale = String(params.locale)

  // State for pagination and filtering
  const [currentPage, setCurrentPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [filters, setFilters] = useState<FilterValue>({})
  const [sortBy, setSortBy] = useState('started_at')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc')

  // Initialize state from URL params
  useEffect(() => {
    const page = parseInt(searchParams.get('page') || '1')
    const size = parseInt(searchParams.get('size') || '20')
    const sort = searchParams.get('sort_by') || 'started_at'
    const order = (searchParams.get('sort_order') || 'desc') as 'asc' | 'desc'

    setCurrentPage(page)
    setPageSize(size)
    setSortBy(sort)
    setSortOrder(order)

    // Initialize filters from URL
    const urlFilters: FilterValue = {}
    filterFields.forEach(field => {
      const value = searchParams.get(field.key)
      if (value) urlFilters[field.key] = value
    })
    setFilters(urlFilters)
  }, [searchParams])

  // Create tenant-aware API client
  const apiClient = createTenantApiClient(tenant)

  // Update URL when filters or pagination change
  const updateURL = useCallback((newFilters: FilterValue, newPage: number, newSize: number) => {
    const params = new URLSearchParams()

    // Add pagination params
    if (newPage > 1) params.set('page', newPage.toString())
    if (newSize !== 20) params.set('size', newSize.toString())
    if (sortBy !== 'started_at') params.set('sort_by', sortBy)
    if (sortOrder !== 'desc') params.set('sort_order', sortOrder)

    // Add filter params
    Object.entries(newFilters).forEach(([key, value]) => {
      if (value) params.set(key, value)
    })

    const queryString = params.toString()
    const newURL = `/${tenant}/${locale}/executions${queryString ? `?${queryString}` : ''}`
    router.replace(newURL)
  }, [tenant, locale, sortBy, sortOrder, router])

  // Fetch executions with current filters and pagination
  const { data: response, isLoading, error, refetch } = useQuery({
    queryKey: ['executions', tenant, currentPage, pageSize, sortBy, sortOrder, filters],
    queryFn: () => apiClient.getExecutions({
      page: currentPage,
      size: pageSize,
      sort_by: sortBy,
      sort_order: sortOrder,
      ...filters
    }),
    refetchInterval: 30000, // Auto-refresh every 30 seconds (much less aggressive)
  })

  // Handle pagination changes
  const handlePageChange = (page: number) => {
    setCurrentPage(page)
    updateURL(filters, page, pageSize)
  }

  const handlePageSizeChange = (size: number) => {
    setPageSize(size)
    setCurrentPage(1) // Reset to first page
    updateURL(filters, 1, size)
  }

  // Handle filter changes
  const handleFiltersChange = (newFilters: FilterValue) => {
    setFilters(newFilters)
    setCurrentPage(1) // Reset to first page when filters change
    updateURL(newFilters, 1, pageSize)
  }

  const handleClearFilters = () => {
    setFilters({})
    setCurrentPage(1)
    updateURL({}, 1, pageSize)
  }

  if (isLoading && !response) {
    return (
      <div className="flex items-center justify-center h-96">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-96">
        <div className="text-center">
          <h2 className="text-lg font-semibold text-foreground mb-2">{t('executions.noExecutions')}</h2>
          <p className="text-muted-foreground">{error.message}</p>
          <Button onClick={() => refetch()} className="mt-4">
            {t('common.refresh')}
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold text-foreground mb-2">
            {t('executions.title')}
          </h1>
          <p className="text-muted-foreground">
            {t('executions.subtitle')}
          </p>
          {isLoading && (
            <p className="text-xs text-blue-600 mt-1">{t('common.loading')}</p>
          )}
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            onClick={() => refetch()}
            disabled={isLoading}
          >
            <RefreshCw className={`h-4 w-4 mr-2 ${isLoading ? 'animate-spin' : ''}`} />
            {t('common.refresh')}
          </Button>
          <Button
            variant="outline"
            onClick={() => window.location.href = `/${tenant}/${locale}/workflows`}
          >
            {t('navigation.workflows')}
          </Button>
        </div>
      </div>

      {/* Filters */}
      <Filters
        fields={filterFields}
        values={filters}
        onChange={handleFiltersChange}
        onClear={handleClearFilters}
        filterMeta={response?.filters}
      />

      <ClientOnly fallback={
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      }>
        <div className="space-y-4">
          <ExecutionList executions={response?.items || []} tenant={tenant} locale={locale} />

          {/* Pagination */}
          {response?.pagination && (
            <Pagination
              pagination={response.pagination}
              onPageChange={handlePageChange}
              onPageSizeChange={handlePageSizeChange}
            />
          )}
        </div>
      </ClientOnly>
    </div>
  )
}
