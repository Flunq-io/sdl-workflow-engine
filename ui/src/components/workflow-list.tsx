'use client'

import { useState } from 'react'
import Link from 'next/link'
import { Workflow } from '@/lib/api'
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
  Zap,
  ArrowRight,
  LayoutGrid,
  Table
} from 'lucide-react'
import { formatRelativeTime, formatAbsoluteTime } from '@/lib/utils'

interface WorkflowListProps {
  workflows: Workflow[]
  tenant?: string
  locale?: string
}

function extractTaskNames(definition: Record<string, any>): string[] {
  if (!definition || !definition.do || !Array.isArray(definition.do)) {
    return []
  }

  return definition.do.map((step: any) => {
    const stepName = Object.keys(step)[0]
    return stepName
  }).filter(Boolean)
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

export function WorkflowList({ workflows, tenant = 'acme-inc', locale = 'en' }: WorkflowListProps) {
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
            All ({workflows.length})
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
                {state} ({count})
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

      {/* Workflow Content */}
      {filteredWorkflows.length === 0 ? (
        <div className="text-center py-12">
          <h3 className="text-lg font-semibold text-foreground mb-2">No workflows found</h3>
          <p className="text-muted-foreground">
            {filter === 'all'
              ? 'Create your first workflow to get started'
              : `No workflows found with status: ${filter}`
            }
          </p>
        </div>
      ) : viewMode === 'cards' ? (
        // Cards View
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
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
                        {workflow.description || 'No description'}
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
                  <Zap className="h-4 w-4 text-muted-foreground" />
                  <span className="text-muted-foreground">{taskSummary}</span>
                </div>

                <div className="flex items-center justify-between text-sm">
                  <div className="flex items-center gap-1 text-muted-foreground">
                    <Calendar className="h-4 w-4" />
                    <span
                      title={formatAbsoluteTime(workflow.created_at, locale)}
                      className="cursor-help"
                    >
                      Created {formatRelativeTime(workflow.created_at, locale)}
                    </span>
                  </div>
                </div>
                
                {workflow.execution_count !== undefined && (
                  <div className="text-sm text-muted-foreground">
                    {workflow.execution_count} execution{workflow.execution_count !== 1 ? 's' : ''}
                    {workflow.last_execution && (
                      <span
                        className="ml-2 cursor-help"
                        title={formatAbsoluteTime(workflow.last_execution, locale)}
                      >
                        • Last: {formatRelativeTime(workflow.last_execution, locale)}
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
                      View
                    </Link>
                  </Button>
                </div>
              </CardContent>
            </Card>
            )
          })}
        </div>
      ) : (
        // Table View
        <div className="border rounded-lg overflow-hidden">
          <table className="w-full">
            <thead className="bg-muted/50">
              <tr>
                <th className="text-left p-4 font-medium">Name</th>
                <th className="text-left p-4 font-medium">Status</th>
                <th className="text-left p-4 font-medium">Tasks</th>
                <th className="text-left p-4 font-medium">Created</th>
                <th className="text-left p-4 font-medium">Executions</th>
                <th className="text-left p-4 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {filteredWorkflows.map((workflow) => {
                const taskNames = extractTaskNames(workflow.definition)

                return (
                  <tr key={workflow.id} className="border-t hover:bg-muted/25">
                    <td className="p-4">
                      <div>
                        <div className="font-medium">{workflow.name}</div>
                        <div className="text-sm text-muted-foreground line-clamp-1">
                          {workflow.description || 'No description'}
                        </div>
                      </div>
                    </td>
                    <td className="p-4">
                      <Badge variant={getStateVariant(workflow.state)} className="flex items-center gap-1 w-fit">
                        {getStateIcon(workflow.state)}
                        {workflow.state}
                      </Badge>
                    </td>
                    <td className="p-4">
                      <div className="text-sm">
                        {taskNames.length} task{taskNames.length !== 1 ? 's' : ''}
                      </div>
                    </td>
                    <td className="p-4">
                      <span
                        title={formatAbsoluteTime(workflow.created_at, locale)}
                        className="cursor-help text-sm"
                      >
                        {formatRelativeTime(workflow.created_at, locale)}
                      </span>
                    </td>
                    <td className="p-4">
                      <div className="text-sm">
                        {workflow.execution_count !== undefined ? (
                          <span>
                            {workflow.execution_count} execution{workflow.execution_count !== 1 ? 's' : ''}
                            {workflow.last_execution && (
                              <div className="text-xs text-muted-foreground">
                                Last: {formatRelativeTime(workflow.last_execution, locale)}
                              </div>
                            )}
                          </span>
                        ) : (
                          <span className="text-muted-foreground">-</span>
                        )}
                      </div>
                    </td>
                    <td className="p-4">
                      <Button asChild size="sm" variant="outline">
                        <Link href={`/${tenant}/${locale}/workflows/${workflow.id}`}>
                          <Eye className="h-4 w-4 mr-1" />
                          View
                        </Link>
                      </Button>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
