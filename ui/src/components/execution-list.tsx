'use client'

import { useState } from 'react'
import Link from 'next/link'
import { Execution } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { SortableTable, Column } from '@/components/common/sortable-table'
import { CardGrid } from '@/components/common/card-grid'
import {
  Clock,
  Calendar,
  Play,
  Pause,
  Square,
  AlertCircle,
  CheckCircle,
  Timer,
  Eye,
  LayoutGrid,
  Table,
  Workflow
} from 'lucide-react'
import { formatDurationHMS, formatDatePairUltraCompact } from '@/lib/utils'
import { useTranslations } from 'next-intl'

interface ExecutionListProps {
  executions: (Execution & { workflow_name?: string })[]
  tenant?: string
  locale?: string
}

function getStatusIcon(status: Execution['status']) {
  switch (status) {
    case 'pending':
      return <Timer className="h-4 w-4" />
    case 'running':
      return <Play className="h-4 w-4" />
    case 'waiting':
      return <Clock className="h-4 w-4" />
    case 'suspended':
      return <Pause className="h-4 w-4" />
    case 'cancelled':
      return <Square className="h-4 w-4" />
    case 'faulted':
      return <AlertCircle className="h-4 w-4" />
    case 'completed':
      return <CheckCircle className="h-4 w-4" />
    default:
      return <Timer className="h-4 w-4" />
  }
}

function getStatusVariant(status: Execution['status']) {
  switch (status) {
    case 'pending':
      return 'pending' as const
    case 'running':
      return 'running' as const
    case 'waiting':
      return 'waiting' as const
    case 'suspended':
      return 'suspended' as const
    case 'cancelled':
      return 'cancelled' as const
    case 'faulted':
      return 'faulted' as const
    case 'completed':
      return 'completed' as const
    default:
      return 'default' as const
  }
}

function formatDuration(durationMs?: number): string {
  if (!durationMs) return 'N/A'
  
  const seconds = Math.floor(durationMs / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  
  if (hours > 0) {
    return `${hours}h ${minutes % 60}m ${seconds % 60}s`
  } else if (minutes > 0) {
    return `${minutes}m ${seconds % 60}s`
  } else {
    return `${seconds}s`
  }
}

export function ExecutionList({ executions, tenant = '', locale = 'en' }: ExecutionListProps) {
  const t = useTranslations()
  const [filter, setFilter] = useState<'all' | Execution['status']>('all')
  const [viewMode, setViewMode] = useState<'cards' | 'table'>('table')

  const filteredExecutions = executions.filter(execution =>
    filter === 'all' || execution.status === filter
  )

  const statusCounts = executions.reduce((acc, execution) => {
    acc[execution.status] = (acc[execution.status] || 0) + 1
    return acc
  }, {} as Record<string, number>)

  return (
    <div className="space-y-6">
      {/* Header with Filters and View Toggle */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        {/* Status Filter */}
        <div className="flex flex-wrap gap-2">
          <Button
            variant={filter === 'all' ? 'default' : 'outline'}
            size="sm"
            onClick={() => setFilter('all')}
          >
            {t('common.all')} ({executions.length})
          </Button>
          {(['pending', 'running', 'waiting', 'suspended', 'cancelled', 'faulted', 'completed'] as const).map(status => {
            const count = statusCounts[status] || 0
            if (count === 0) return null

            return (
              <Button
                key={status}
                variant={filter === status ? 'default' : 'outline'}
                size="sm"
                onClick={() => setFilter(status)}
                className="capitalize"
              >
                {t(`status.${status}`)} ({count})
              </Button>
            )
          })}
        </div>

        {/* View Toggle */}
        <div className="flex gap-2">
          <Button
            variant={viewMode === 'cards' ? 'default' : 'outline'}
            size="sm"
            onClick={() => setViewMode('cards')}
          >
            <LayoutGrid className="h-4 w-4 mr-1" />
            {t('api.view.cards')}
          </Button>
          <Button
            variant={viewMode === 'table' ? 'default' : 'outline'}
            size="sm"
            onClick={() => setViewMode('table')}
          >
            <Table className="h-4 w-4 mr-1" />
            {t('api.view.table')}
          </Button>
        </div>
      </div>

      {/* Execution Content */}
      {filteredExecutions.length === 0 ? (
        <div className="text-center py-12">
          <h3 className="text-lg font-semibold text-foreground mb-2">{t('executions.noExecutions')}</h3>
          <p className="text-muted-foreground">
            {filter === 'all'
              ? t('executions.noExecutionsDescription')
              : t('executions.noExecutionsFiltered', {status: filter})
            }
          </p>
        </div>
      ) : viewMode === 'cards' ? (
        // Cards View
        <CardGrid>
          {filteredExecutions.map((execution) => (
            <Card key={execution.id} className="hover:shadow-md transition-shadow">
              <CardHeader className="pb-3">
                <div className="flex items-start justify-between">
                  <div className="space-y-1">
                    <CardTitle className="text-lg">
                      {execution.workflow_name || execution.workflow_id}
                    </CardTitle>
                    <p className="text-sm text-muted-foreground">
                      {t('common.execution')}: {execution.id.slice(0, 8)}...
                    </p>
                  </div>
                  <Badge variant={getStatusVariant(execution.status)} className="flex items-center gap-1">
                    {getStatusIcon(execution.status)}
                    {t(`status.${execution.status}`)}
                  </Badge>
                </div>
              </CardHeader>

              <CardContent className="space-y-4">
                {/* Workflow Info */}
                <div className="flex items-center gap-2 text-sm">
                  <Workflow className="h-4 w-4 text-muted-foreground" />
                  <span className="text-muted-foreground">{execution.workflow_id}</span>
                </div>

                {/* Timing Info */}
                <div className="space-y-1 text-sm">
                  <div className="flex items-center gap-1 text-muted-foreground">
                    <Calendar className="h-4 w-4" />
                    <span className="text-muted-foreground" title={new Date(execution.started_at).toISOString()}>
                      {t('executions.startedAt')} {formatDatePairUltraCompact(execution.started_at, locale)}
                    </span>
                  </div>
                  {execution.completed_at && (
                    <div className="flex items-center gap-1 text-muted-foreground">
                      <Clock className="h-4 w-4" />
                      <span className="text-muted-foreground" title={new Date(execution.completed_at).toISOString()}>
                        {t('common.completed')} {formatDatePairUltraCompact(execution.completed_at, locale)}
                      </span>
                    </div>
                  )}
                </div>

                {execution.duration_ms && (
                  <div className="text-sm text-muted-foreground">
                    {t('common.duration')}: {formatDuration(execution.duration_ms)}
                  </div>
                )}

                {execution.current_state && (
                  <div className="text-sm">
                    <span className="text-muted-foreground">{t('executions.currentStep')}: </span>
                    <span className="font-medium">{execution.current_state}</span>
                  </div>
                )}

                <div className="flex gap-2 pt-2">
                  <Button asChild size="sm" variant={'outline'} className="flex-1 text-blue-500">
                    <Link href={`/${tenant}/${locale}/executions/${execution.id}`}>
                      <Eye className="h-4 w-4 mr-1" />
                      {t('common.view')}
                    </Link>
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </CardGrid>
      ) : (
        // Table View
        <SortableTable
          columns={[
            {
              id: 'id',
              header: t('executions.executionId'),
              sortable: true,
              accessor: (e: Execution) => e.id,
              cell: (e: Execution) => (
                <div>
                  <div className="font-medium font-mono text-sm">{e.id}</div>
                  {e.current_state && (
                    <div className="text-xs text-muted-foreground">
                      {t('executions.currentStep')}: {e.current_state}
                    </div>
                  )}
                </div>
              )
            },
            {
              id: 'workflow',
              header: t('executions.workflowName'),
              sortable: true,
              accessor: (e: Execution & { workflow_name?: string }) => e.workflow_name || e.workflow_id,
              cell: (e: Execution & { workflow_name?: string }) => (
                <div>
                  <div className="font-medium">{e.workflow_name || e.workflow_id}</div>
                  <div className="text-sm text-muted-foreground font-mono">{e.workflow_id}</div>
                </div>
              )
            },
            {
              id: 'status',
              header: t('common.status'),
              sortable: true,
              accessor: (e: Execution) => e.status,
              cell: (e: Execution) => (
                <Badge variant={getStatusVariant(e.status)} className="flex items-center gap-1 w-fit">
                  {getStatusIcon(e.status)}
                  {e.status}
                </Badge>
              )
            },
            {
              id: 'started_at',
              header: t('executions.startedAt'),
              sortable: true,
              accessor: (e: Execution) => e.started_at,
              cell: (e: Execution) => (
                <span className="text-sm" title={new Date(e.started_at).toISOString()}>
                  {formatDatePairUltraCompact(e.started_at, locale)}
                </span>
              )
            },
            {
              id: 'completed_at',
              header: t('common.completed'),
              sortable: true,
              accessor: (e: Execution) => e.completed_at ?? '',
              cell: (e: Execution) => (
                <span className="text-sm">
                  {e.completed_at ? (
                    <>{formatDatePairUltraCompact(e.completed_at, locale)}</>
                  ) : (
                    <span className="text-muted-foreground">-</span>
                  )}
                </span>
              )
            },
            {
              id: 'duration',
              header: t('common.duration'),
              sortable: true,
              accessor: (e: Execution) => e.duration_ms ?? 0,
              cell: (e: Execution) => (
                <div className="text-sm">
                  {e.duration_ms ? (
                    <>
                      {formatDuration(e.duration_ms)}
                      <span className="text-muted-foreground"> ({formatDurationHMS(e.duration_ms)})</span>
                    </>
                  ) : e.status === 'running' ? (
                    <span className="text-muted-foreground">Running...</span>
                  ) : (
                    <span className="text-muted-foreground">-</span>
                  )}
                </div>
              )
            },
            {
              id: 'actions',
              header: t('common.actions'),
              cell: (e: Execution) => (
                <Button asChild size="sm" variant="outline">
                  <Link href={`/${tenant}/${locale}/executions/${e.id}`}>
                    <Eye className="h-4 w-4 mr-1" />
                    {t('common.view')}
                  </Link>
                </Button>
              )
            }
          ] as Column<Execution>[]}
          data={filteredExecutions}
          initialSort={{ columnId: 'started_at', direction: 'desc' }}
        />
      )}
    </div>
  )
}
