'use client'

import { Workflow, WorkflowEvent } from '@/lib/api'
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
  events?: WorkflowEvent[]
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

// Helper function to determine task status from events
function getTaskStatus(taskName: string, events: WorkflowEvent[] = []) {
  // Filter events that match this exact task name
  const taskEvents = events.filter(event => {
    // Handle different event data structures
    let eventTaskName = null

    if (event.type === 'io.flunq.task.requested') {
      eventTaskName = event.data?.task_name
    } else if (event.type === 'io.flunq.task.completed') {
      eventTaskName = event.data?.data?.task_name || event.data?.task_name
    }

    return eventTaskName === taskName
  })

  // Check for task completion
  const hasCompleted = taskEvents.some(event =>
    event.type === 'io.flunq.task.completed'
  )

  // Check for task start/request
  const hasStarted = taskEvents.some(event =>
    event.type === 'io.flunq.task.requested' ||
    event.type === 'io.flunq.task.started'
  )

  if (hasCompleted) return 'completed'
  if (hasStarted) return 'active'
  return 'pending'
}

// Helper function to find the current active task
function getCurrentTaskIndex(tasks: any[], events: WorkflowEvent[] = []) {
  // Find the task that was requested but not yet completed (active task)
  for (let i = 0; i < tasks.length; i++) {
    const taskName = tasks[i].name || Object.keys(tasks[i])[0]
    const status = getTaskStatus(taskName, events)

    if (status === 'active') {
      return i
    }
  }

  // If no active task found, find the first pending task
  for (let i = 0; i < tasks.length; i++) {
    const taskName = tasks[i].name || Object.keys(tasks[i])[0]
    const status = getTaskStatus(taskName, events)

    if (status === 'pending') {
      return i
    }
  }

  // If all tasks are completed, highlight the last task
  return Math.max(0, tasks.length - 1)
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

export function WorkflowDetail({ workflow, events = [] }: WorkflowDetailProps) {
  const taskNames = extractTaskNames(workflow.definition)
  const locale = useLocale()
  const currentTaskIndex = getCurrentTaskIndex(workflow.definition?.do || [], events)

  // Debug task statuses
  console.log('=== Task Status Debug ===')
  taskNames.forEach((taskName, index) => {
    const status = getTaskStatus(taskName, events, index)
    console.log(`Task ${index}: ${taskName} = ${status}`)
  })
  console.log(`Current task index: ${currentTaskIndex}`)
  console.log('=========================')

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
              {taskNames.map((taskName, index) => {
                const taskStatus = getTaskStatus(taskName, events)
                const isCurrentTask = index === currentTaskIndex
                return (
                  <div key={taskName} className="flex items-center gap-2 text-sm">
                    <div className="flex items-center gap-1">
                      <div className={`w-2 h-2 rounded-full ${
                        taskStatus === 'completed' ? 'bg-green-500' :
                        taskStatus === 'active' ? 'bg-blue-500' :
                        isCurrentTask && workflow.status === 'running' ? 'bg-blue-500' :
                        'bg-gray-300'
                      }`} />
                      <span className={`${
                        taskStatus === 'completed' ? 'text-foreground font-medium' :
                        taskStatus === 'active' ? 'text-foreground font-medium' :
                        isCurrentTask && workflow.status === 'running' ? 'text-foreground font-medium' :
                        'text-muted-foreground'
                      }`}>
                        {taskName}
                      </span>
                    </div>
                    {index < taskNames.length - 1 && (
                      <ArrowRight className="h-3 w-3 text-muted-foreground" />
                    )}
                  </div>
                )
              })}
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
