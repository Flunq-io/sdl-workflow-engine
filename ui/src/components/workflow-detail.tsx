'use client'

import { Workflow } from '@/lib/api'
import { useLocale } from 'next-intl'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import {
  Clock,
  Calendar,
  Play,
  Pause,
  Square,
  AlertCircle,
  CheckCircle,
  Timer,
  Tag,
  Hash,
  ListTodo,
  ArrowRight
} from 'lucide-react'
import { formatRelativeTime, formatAbsoluteTime } from '@/lib/utils'

interface WorkflowDetailProps {
  workflow: Workflow
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

export function WorkflowDetail({ workflow }: WorkflowDetailProps) {
  const taskNames = extractTaskNames(workflow.definition)
  const locale = useLocale()

  return (
    <div className="space-y-6">
      {/* Status Card */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Status</CardTitle>
        </CardHeader>
        <CardContent>
          <Badge variant={getStatusVariant(workflow.status)} className="flex items-center gap-2 w-fit">
            {getStatusIcon(workflow.status)}
            <span className="capitalize">{workflow.status}</span>
          </Badge>
        </CardContent>
      </Card>

      {/* Task Progress Card */}
      {taskNames.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg flex items-center gap-2">
              <ListTodo className="h-4 w-4" />
              Tasks ({taskNames.length})
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              {taskNames.map((taskName, index) => (
                <div key={taskName} className="flex items-center gap-2 text-sm">
                  <div className="flex items-center gap-1">
                    <div className={`w-2 h-2 rounded-full ${
                      workflow.status === 'completed' ? 'bg-green-500' :
                      workflow.status === 'running' && index === 0 ? 'bg-blue-500' :
                      'bg-gray-300'
                    }`} />
                    <span className={workflow.status === 'completed' ? 'text-foreground' : 'text-muted-foreground'}>
                      {taskName}
                    </span>
                  </div>
                  {index < taskNames.length - 1 && (
                    <ArrowRight className="h-3 w-3 text-muted-foreground" />
                  )}
                </div>
              ))}
            </div>

            {workflow.status === 'completed' && (
              <div className="mt-3 pt-3 border-t">
                <div className="flex items-center gap-2 text-sm text-green-600">
                  <CheckCircle className="h-4 w-4" />
                  All tasks completed successfully
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Metadata Card */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Metadata</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-2 text-sm">
            <Hash className="h-4 w-4 text-muted-foreground" />
            <span className="text-muted-foreground">ID:</span>
            <code className="bg-muted px-2 py-1 rounded text-xs font-mono">
              {workflow.id}
            </code>
          </div>
          
          <div className="flex items-center gap-2 text-sm">
            <Calendar className="h-4 w-4 text-muted-foreground" />
            <span className="text-muted-foreground">Created:</span>
            <span
              title={formatAbsoluteTime(workflow.created_at, locale)}
              className="cursor-help"
            >
              {formatRelativeTime(workflow.created_at, locale)}
            </span>
          </div>

          <div className="flex items-center gap-2 text-sm">
            <Clock className="h-4 w-4 text-muted-foreground" />
            <span className="text-muted-foreground">Updated:</span>
            <span
              title={formatAbsoluteTime(workflow.updated_at, locale)}
              className="cursor-help"
            >
              {formatRelativeTime(workflow.updated_at, locale)}
            </span>
          </div>

          {typeof workflow.execution_count === 'number' && (
            <div className="flex items-center gap-2 text-sm">
              <Play className="h-4 w-4 text-muted-foreground" />
              <span className="text-muted-foreground">Executions:</span>
              <span>{workflow.execution_count}</span>
            </div>
          )}

          {workflow.last_execution && (
            <div className="flex items-center gap-2 text-sm">
              <Timer className="h-4 w-4 text-muted-foreground" />
              <span className="text-muted-foreground">Last execution:</span>
              <span
                title={formatAbsoluteTime(workflow.last_execution, locale)}
                className="cursor-help"
              >
                {formatRelativeTime(workflow.last_execution, locale)}
              </span>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Tags Card */}
      {workflow.tags.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg flex items-center gap-2">
              <Tag className="h-4 w-4" />
              Tags
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-2">
              {workflow.tags.map((tag) => (
                <Badge key={tag} variant="outline">
                  {tag}
                </Badge>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
