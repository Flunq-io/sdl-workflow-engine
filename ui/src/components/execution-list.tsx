'use client'

import { useState } from 'react'
import Link from 'next/link'
import { Execution } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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
import { formatDurationHMS, formatDatePairUltraCompact, isDisplayableDate } from '@/lib/utils'

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
            All ({executions.length})
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
                {status} ({count})
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
            Cards
          </Button>
          <Button
            variant={viewMode === 'table' ? 'default' : 'outline'}
            size="sm"
            onClick={() => setViewMode('table')}
          >
            <Table className="h-4 w-4 mr-1" />
            Table
          </Button>
        </div>
      </div>

      {/* Execution Content */}
      {filteredExecutions.length === 0 ? (
        <div className="text-center py-12">
          <h3 className="text-lg font-semibold text-foreground mb-2">No executions found</h3>
          <p className="text-muted-foreground">
            {filter === 'all'
              ? 'Execute a workflow to see executions here'
              : `No executions found with status: ${filter}`
            }
          </p>
        </div>
      ) : viewMode === 'cards' ? (
        // Cards View
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filteredExecutions.map((execution) => (
            <Card key={execution.id} className="hover:shadow-md transition-shadow">
              <CardHeader className="pb-3">
                <div className="flex items-start justify-between">
                  <div className="space-y-1">
                    <CardTitle className="text-lg">
                      {execution.workflow_name || execution.workflow_id}
                    </CardTitle>
                    <p className="text-sm text-muted-foreground">
                      Execution: {execution.id.slice(0, 8)}...
                    </p>
                  </div>
                  <Badge variant={getStatusVariant(execution.status)} className="flex items-center gap-1">
                    {getStatusIcon(execution.status)}
                    {execution.status}
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
                      Started {formatDatePairUltraCompact(execution.started_at, locale)}
                    </span>
                  </div>
                  {execution.completed_at && (
                    <div className="flex items-center gap-1 text-muted-foreground">
                      <Clock className="h-4 w-4" />
                      <span className="text-muted-foreground" title={new Date(execution.completed_at).toISOString()}>
                        Completed {formatDatePairUltraCompact(execution.completed_at, locale)}
                      </span>
                    </div>
                  )}
                </div>

                {execution.duration_ms && (
                  <div className="text-sm text-muted-foreground">
                    Duration: {formatDuration(execution.duration_ms)}
                  </div>
                )}
                
                {execution.current_state && (
                  <div className="text-sm">
                    <span className="text-muted-foreground">Current step: </span>
                    <span className="font-medium">{execution.current_state}</span>
                  </div>
                )}
                
                <div className="flex gap-2 pt-2">
                  <Button asChild size="sm" variant={'outline'} className="flex-1 text-blue-500">
                    <Link href={`/${tenant}/${locale}/executions/${execution.id}`}>
                      <Eye className="h-4 w-4 mr-1" />
                      View
                    </Link>
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      ) : (
        // Table View
        <div className="border rounded-lg overflow-hidden">
          <table className="w-full">
            <thead className="bg-muted/50">
              <tr>
                <th className="text-left p-4 font-medium">Execution</th>
                <th className="text-left p-4 font-medium">Workflow</th>
                <th className="text-left p-4 font-medium">Status</th>
                <th className="text-left p-4 font-medium">Started</th>
                <th className="text-left p-4 font-medium">Completed</th>
                <th className="text-left p-4 font-medium">Duration</th>
                <th className="text-left p-4 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {filteredExecutions.map((execution) => (
                <tr key={execution.id} className="border-t hover:bg-muted/25">
                  <td className="p-4">
                    <div>
                      <div className="font-medium font-mono text-sm">{execution.id}</div>
                      {execution.current_state && (
                        <div className="text-xs text-muted-foreground">
                          Step: {execution.current_state}
                        </div>
                      )}
                    </div>
                  </td>
                  <td className="p-4">
                    <div>
                      <div className="font-medium">{execution.workflow_name || execution.workflow_id}</div>
                      <div className="text-sm text-muted-foreground font-mono">
                        {execution.workflow_id}
                      </div>
                    </div>
                  </td>
                  <td className="p-4">
                    <Badge variant={getStatusVariant(execution.status)} className="flex items-center gap-1 w-fit">
                      {getStatusIcon(execution.status)}
                      {execution.status}
                    </Badge>
                  </td>
                  <td className="p-4">
                    <span className="text-sm" title={new Date(execution.started_at).toISOString()}>
                      {formatDatePairUltraCompact(execution.started_at, locale)}
                    </span>
                  </td>
                  <td className="p-4">
                    <span className="text-sm">
                      {execution.completed_at ? (
                        <>{formatDatePairUltraCompact(execution.completed_at, locale)}</>
                      ) : (
                        <span className="text-muted-foreground">-</span>
                      )}
                    </span>
                  </td>
                  <td className="p-4">
                    <div className="text-sm">
                      {execution.duration_ms ? (
                        <>
                          {formatDuration(execution.duration_ms)}
                          <span className="text-muted-foreground"> ({formatDurationHMS(execution.duration_ms)})</span>
                        </>
                      ) : execution.status === 'running' ? (
                        <span className="text-muted-foreground">Running...</span>
                      ) : (
                        <span className="text-muted-foreground">-</span>
                      )}
                    </div>
                  </td>
                  <td className="p-4">
                    <Button asChild size="sm" variant="outline">
                      <Link href={`/${tenant}/${locale}/executions/${execution.id}`}>
                        <Eye className="h-4 w-4 mr-1" />
                        View
                      </Link>
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
