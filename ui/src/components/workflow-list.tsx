'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useTranslations, useLocale } from 'next-intl'
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
  ArrowRight
} from 'lucide-react'
import { formatRelativeTime, formatAbsoluteTime } from '@/lib/utils'

interface WorkflowListProps {
  workflows: Workflow[]
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

function getTaskSummary(taskNames: string[], status: Workflow['status']): string {
  if (taskNames.length === 0) return 'No tasks defined'

  if (status === 'completed') {
    return `Completed ${taskNames.length} tasks: ${taskNames.slice(0, 3).join(', ')}${taskNames.length > 3 ? '...' : ''}`
  }

  if (status === 'running') {
    return `Running tasks: ${taskNames.slice(0, 3).join(' → ')}${taskNames.length > 3 ? '...' : ''}`
  }

  return `${taskNames.length} tasks: ${taskNames.slice(0, 3).join(', ')}${taskNames.length > 3 ? '...' : ''}`
}

function getStatusIcon(status: Workflow['status']) {
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

function getStatusVariant(status: Workflow['status']) {
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

export function WorkflowList({ workflows }: WorkflowListProps) {
  const [filter, setFilter] = useState<'all' | Workflow['status']>('all')
  const t = useTranslations()
  const locale = useLocale()

  const filteredWorkflows = workflows.filter(workflow =>
    filter === 'all' || workflow.status === filter
  )

  const statusCounts = workflows.reduce((acc, workflow) => {
    acc[workflow.status] = (acc[workflow.status] || 0) + 1
    return acc
  }, {} as Record<string, number>)

  return (
    <div className="space-y-6">
      {/* Status Filter */}
      <div className="flex flex-wrap gap-2">
        <Button
          variant={filter === 'all' ? 'default' : 'outline'}
          size="sm"
          onClick={() => setFilter('all')}
        >
          All ({workflows.length})
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

      {/* Workflow Grid */}
      {filteredWorkflows.length === 0 ? (
        <div className="text-center py-12">
          <h3 className="text-lg font-semibold text-foreground mb-2">{t('workflows.noWorkflows')}</h3>
          <p className="text-muted-foreground">
            {filter === 'all'
              ? t('workflows.noWorkflowsDescription')
              : t('workflows.noWorkflowsFiltered', { status: filter })
            }
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filteredWorkflows.map((workflow) => {
            const taskNames = extractTaskNames(workflow.definition)
            const taskSummary = getTaskSummary(taskNames, workflow.status)

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
                    <Badge variant={getStatusVariant(workflow.status)} className="flex items-center gap-1">
                      {getStatusIcon(workflow.status)}
                      {workflow.status}
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
                    <Link href={`/${locale}/workflows/${workflow.id}`}>
                      <Eye className="h-4 w-4 mr-1" />
                      {t('common.view')}
                    </Link>
                  </Button>
                </div>
              </CardContent>
            </Card>
            )
          })}
        </div>
      )}
    </div>
  )
}
