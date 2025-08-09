'use client'

import { useState } from 'react'
import Link from 'next/link'
import { Workflow } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { SortableTable, Column } from '@/components/common/sortable-table'
import { CardGrid } from '@/components/common/card-grid'
import {
  Calendar,
  Pause,
  CheckCircle,
  Eye,
  LayoutGrid,
  Table
} from 'lucide-react'
import { formatRelativeTime, formatAbsoluteTime } from '@/lib/utils'
import { useTranslations } from 'next-intl'

interface WorkflowListProps {
  workflows: Workflow[]
  tenant?: string
  locale?: string
}

type DoStep = Record<string, unknown>

function extractTaskNames(definition: { do?: DoStep[] } | null | undefined): string[] {
  if (!definition || !Array.isArray(definition.do)) return []
  return definition.do
    .map((step) => Object.keys(step || {})[0])
    .filter((name): name is string => Boolean(name))
}

function getTaskSummary(taskNames: string[]): string {
  if (taskNames.length === 0) return 'No tasks defined'
  return `${taskNames.length} tasks: ${taskNames.slice(0, 3).join(', ')}${taskNames.length > 3 ? '...' : ''}`
}

function getStateIcon(state: Workflow['state']) {
  switch (state) {
    case 'active':
      return <CheckCircle className="h-4 w-4" />
    case 'inactive':
      return <Pause className="h-4 w-4" />
    default:
      return <CheckCircle className="h-4 w-4" />
  }
}

function getStateVariant(state: Workflow['state']) {
  switch (state) {
    case 'active':
      return 'default' as const
    case 'inactive':
      return 'secondary' as const
    default:
      return 'default' as const
  }
}

export function WorkflowList({ workflows, tenant = '', locale = 'en' }: WorkflowListProps) {
  const t = useTranslations()
  const [filter, setFilter] = useState<'all' | Workflow['state']>('all')
  const [viewMode, setViewMode] = useState<'cards' | 'table'>('table')

  const filteredWorkflows = workflows.filter(workflow =>
    filter === 'all' || workflow.state === filter
  )

  const stateCounts = workflows.reduce((acc, workflow) => {
    acc[workflow.state] = (acc[workflow.state] || 0) + 1
    return acc
  }, {} as Record<string, number>)

  return (
    <div className="space-y-6">
      {/* Header with Filters and View Toggle */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        {/* State Filter */}
        <div className="flex flex-wrap gap-2">
          <Button
            variant={filter === 'all' ? 'default' : 'outline'}
            size="sm"
            onClick={() => setFilter('all')}
          >
            {t('common.all')} ({workflows.length})
          </Button>
          {(['active', 'inactive'] as const).map(state => {
            const count = stateCounts[state] || 0
            if (count === 0) return null

            return (
              <Button
                key={state}
                variant={filter === state ? 'default' : 'outline'}
                size="sm"
                onClick={() => setFilter(state)}
                className="capitalize"
              >
                {t(`status.${state}`)} ({count})
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

      {/* Workflow Content */}
      {filteredWorkflows.length === 0 ? (
        <div className="text-center py-12">
          <h3 className="text-lg font-semibold text-foreground mb-2">{t('workflows.noWorkflows')}</h3>
          <p className="text-muted-foreground">
            {filter === 'all'
              ? t('workflows.noWorkflowsDescription')
              : t('workflows.noWorkflowsFiltered', {status: filter})
            }
          </p>
        </div>
      ) : viewMode === 'cards' ? (
        // Cards View
        <CardGrid>
          {filteredWorkflows.map((workflow) => {
            const taskNames = extractTaskNames(workflow.definition)
            const taskSummary = getTaskSummary(taskNames)

            return (
              <Card key={workflow.id} className="hover:shadow-md transition-shadow">
                <CardHeader className="pb-3">
                  <div className="flex items-start justify-between">
                    <div className="space-y-1">
                      <CardTitle className="text-lg">{workflow.name}</CardTitle>
                      <p className="text-sm text-muted-foreground line-clamp-2">
                        {workflow.description || t('workflowDetail.noDescription')}
                      </p>
                    </div>
                    <Badge variant={getStateVariant(workflow.state)} className="flex items-center gap-1">
                      {getStateIcon(workflow.state)}
                      {workflow.state}
                    </Badge>
                  </div>
                </CardHeader>

              <CardContent className="space-y-4">
                {/* Task Summary */}
                <div className="flex items-center gap-2 text-sm">
                  <LayoutGrid className="h-4 w-4 text-muted-foreground" />
                  <span className="text-muted-foreground">{taskSummary}</span>
                </div>

                <div className="flex items-center justify-between text-sm">
                  <div className="flex items-center gap-1 text-muted-foreground">
                    <Calendar className="h-4 w-4" />
                    <span
                      title={formatAbsoluteTime(workflow.created_at, locale)}
                      className="cursor-help"
                    >
                      {t('common.created')} {formatRelativeTime(workflow.created_at, locale)}
                    </span>
                  </div>
                </div>

                {workflow.execution_count !== undefined && (
                  <div className="text-sm text-muted-foreground">
                    {t('workflows.executionCount', {count: workflow.execution_count ?? 0})}
                    {workflow.last_execution && (
                      <span
                        className="ml-2 cursor-help"
                        title={formatAbsoluteTime(workflow.last_execution, locale)}
                      >
                        • {t('workflows.lastExecution', {time: formatRelativeTime(workflow.last_execution, locale)})}
                      </span>
                    )}
                  </div>
                )}

                {workflow.tags.length > 0 && (
                  <div className="flex flex-wrap gap-1">
                    {workflow.tags.slice(0, 3).map((tag) => (
                      <Badge key={tag} variant="outline" className="text-xs">
                        {tag}
                      </Badge>
                    ))}
                    {workflow.tags.length > 3 && (
                      <Badge variant="outline" className="text-xs">
                        +{workflow.tags.length - 3}
                      </Badge>
                    )}
                  </div>
                )}

                <div className="flex gap-2 pt-2">
                  <Button asChild size="sm" variant={'outline'} className="flex-1 text-blue-500">
                    <Link href={`/${tenant}/${locale}/workflows/${workflow.id}`}>
                      <Eye className="h-4 w-4 mr-1" />
                      {t('common.view')}
                    </Link>
                  </Button>
                </div>
              </CardContent>
            </Card>
            )
          })}
        </CardGrid>
      ) : (
        // Table View
        <SortableTable
          columns={[
            {
              id: 'name',
              header: 'Name',
              sortable: true,
              accessor: (w: Workflow) => w.name,
              cell: (w: Workflow) => (
                <div>
                  <div className="font-medium">{w.name}</div>
                  <div className="text-sm text-muted-foreground line-clamp-1">
                    {w.description || 'No description'}
                  </div>
                </div>
              )
            },
            {
              id: 'status',
              header: 'Status',
              sortable: true,
              accessor: (w: Workflow) => w.state,
              cell: (w: Workflow) => (
                <Badge variant={getStateVariant(w.state)} className="flex items-center gap-1 w-fit">
                  {getStateIcon(w.state)}
                  {w.state}
                </Badge>
              )
            },
            {
              id: 'tasks',
              header: 'Tasks',
              accessor: (w: Workflow) => extractTaskNames(w.definition).length,
              cell: (w: Workflow) => {
                const taskNames = extractTaskNames(w.definition)
                return (
                  <div className="text-sm">
                    {t('workflows.taskSummary.default', {count: taskNames.length, tasks: taskNames.slice(0, 3).join(', ')})}
                  </div>
                )
              }
            },
            {
              id: 'created',
              header: 'Created',
              sortable: true,
              accessor: (w: Workflow) => w.created_at,
              cell: (w: Workflow) => (
                <span title={formatAbsoluteTime(w.created_at, locale)} className="cursor-help text-sm">
                  {formatRelativeTime(w.created_at, locale)}
                </span>
              )
            },
            {
              id: 'executions',
              header: 'Executions',
              accessor: (w: Workflow) => w.execution_count ?? 0,
              cell: (w: Workflow) => (
                <div className="text-sm">
                  {w.execution_count !== undefined ? (
                    <span>
                      {t('workflows.executionCount', {count: w.execution_count ?? 0})}
                      {w.last_execution && (
                        <div className="text-xs text-muted-foreground">
                          {t('workflows.lastExecution', {time: formatRelativeTime(w.last_execution, locale)})}
                        </div>
                      )}
                    </span>
                  ) : (
                    <span className="text-muted-foreground">-</span>
                  )}
                </div>
              )
            },
            {
              id: 'actions',
              header: 'Actions',
              cell: (w: Workflow) => (
                <Button asChild size="sm" variant="outline">
                  <Link href={`/${tenant}/${locale}/workflows/${w.id}`}>
                    <Eye className="h-4 w-4 mr-1" />
                    {t('common.view')}
                  </Link>
                </Button>
              )
            }
          ] as Column<Workflow>[]}
          data={filteredWorkflows}
          initialSort={{ columnId: 'created', direction: 'desc' }}
        />
      )}
    </div>
  )
}
