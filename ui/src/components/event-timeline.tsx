'use client'

import { WorkflowEvent } from '@/lib/api'
import { useLocale } from 'next-intl'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { 
  Activity,
  Clock,
  Loader2,
  AlertCircle,
  CheckCircle,
  Play,
  Square,
  Zap,
  FileText,
  Database
} from 'lucide-react'
import { formatRelativeTime, formatAbsoluteTime } from '@/lib/utils'

interface EventTimelineProps {
  events: WorkflowEvent[]
  isLoading?: boolean
  error?: Error | null
}

function getEventIcon(eventType: string) {
  if (eventType.includes('workflow.created')) return <FileText className="h-4 w-4" />
  if (eventType.includes('workflow.started') || eventType.includes('execution.started')) return <Play className="h-4 w-4" />
  if (eventType.includes('workflow.completed') || eventType.includes('execution.completed')) return <CheckCircle className="h-4 w-4" />
  if (eventType.includes('workflow.failed') || eventType.includes('execution.failed')) return <AlertCircle className="h-4 w-4" />
  if (eventType.includes('workflow.cancelled') || eventType.includes('execution.cancelled')) return <Square className="h-4 w-4" />
  if (eventType.includes('task.requested')) return <Clock className="h-4 w-4" />
  if (eventType.includes('task.completed')) return <CheckCircle className="h-4 w-4" />
  if (eventType.includes('task.failed')) return <AlertCircle className="h-4 w-4" />
  if (eventType.includes('data')) return <Database className="h-4 w-4" />
  return <Activity className="h-4 w-4" />
}

function getEventColor(eventType: string) {
  if (eventType.includes('created')) return 'bg-blue-500'
  if (eventType.includes('started')) return 'bg-green-500'
  if (eventType.includes('completed')) return 'bg-green-600'
  if (eventType.includes('failed') || eventType.includes('faulted')) return 'bg-red-500'
  if (eventType.includes('cancelled')) return 'bg-gray-500'
  if (eventType.includes('requested')) return 'bg-yellow-500'
  if (eventType.includes('waiting')) return 'bg-orange-500'
  return 'bg-blue-400'
}

function isTaskEvent(eventType: string): boolean {
  return eventType.includes('task.')
}

function formatEventType(eventType: string) {
  return eventType
    .replace('io.flunq.', '')
    .replace(/\./g, ' ')
    .replace(/\b\w/g, l => l.toUpperCase())
}

function extractTaskName(event: WorkflowEvent): string | null {
  if (!event.data) return null

  // Try different ways to extract task name from event data
  if (event.data.task_name) return event.data.task_name
  if (event.data.taskName) return event.data.taskName

  // For task events, also check nested data
  if (event.type.includes('task.')) {
    if (event.data.input?.task_name) return event.data.input.task_name
    if (event.data.output?.task_name) return event.data.output.task_name
  }

  return null
}

function getEventTitle(event: WorkflowEvent): { title: string; subtitle?: string } {
  const taskName = extractTaskName(event)
  const baseType = formatEventType(event.type)

  if (taskName) {
    if (event.type.includes('task.requested')) {
      return {
        title: `Task Requested: ${taskName}`,
        subtitle: baseType
      }
    }
    if (event.type.includes('task.completed')) {
      return {
        title: `Task Completed: ${taskName}`,
        subtitle: baseType
      }
    }
    if (event.type.includes('task.failed')) {
      return {
        title: `Task Failed: ${taskName}`,
        subtitle: baseType
      }
    }
  }

  return { title: baseType }
}

export function EventTimeline({ events, isLoading, error }: EventTimelineProps) {
  const locale = useLocale()

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-lg flex items-center gap-2">
            <Activity className="h-5 w-5" />
            Event Timeline
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-center py-8">
            <div className="text-center">
              <AlertCircle className="h-8 w-8 text-muted-foreground mx-auto mb-2" />
              <p className="text-muted-foreground">Failed to load events</p>
              <p className="text-sm text-muted-foreground">{error.message}</p>
            </div>
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg flex items-center gap-2">
          <Activity className="h-5 w-5" />
          Event Timeline
          {isLoading && <Loader2 className="h-4 w-4 animate-spin" />}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {events.length === 0 ? (
          <div className="flex items-center justify-center py-8">
            <div className="text-center">
              <Activity className="h-8 w-8 text-muted-foreground mx-auto mb-2" />
              <p className="text-muted-foreground">No events yet</p>
              <p className="text-sm text-muted-foreground">Events will appear here as the workflow executes</p>
            </div>
          </div>
        ) : (
          <div className="space-y-4">
            {events.map((event, index) => {
              const { title, subtitle } = getEventTitle(event)
              const taskName = extractTaskName(event)

              return (
                <div key={event.id} className="flex gap-4">
                  {/* Timeline line */}
                  <div className="flex flex-col items-center">
                    <div className={`w-8 h-8 rounded-full ${getEventColor(event.type)} flex items-center justify-center text-white`}>
                      {getEventIcon(event.type)}
                    </div>
                    {index < events.length - 1 && (
                      <div className="w-0.5 h-6 bg-border mt-2" />
                    )}
                  </div>

                  {/* Event content */}
                  <div className={`flex-1 min-w-0 pb-4 ${isTaskEvent(event.type) ? 'pl-3 border-l-2 border-l-blue-200' : ''}`}>
                    <div className="flex items-start justify-between">
                      <div className="space-y-1">
                        <div className="flex items-center gap-2 flex-wrap">
                          <h4 className={`font-medium text-sm ${isTaskEvent(event.type) ? 'text-blue-700' : ''}`}>
                            {title}
                          </h4>
                          {subtitle && (
                            <Badge variant="secondary" className="text-xs">
                              {subtitle}
                            </Badge>
                          )}
                          <Badge variant="outline" className="text-xs">
                            {event.source}
                          </Badge>
                          {isTaskEvent(event.type) && (
                            <Badge variant="default" className="text-xs bg-blue-100 text-blue-700 hover:bg-blue-200">
                              Task Event
                            </Badge>
                          )}
                        </div>

                        <div className="flex items-center gap-4 text-xs text-muted-foreground flex-wrap">
                          <span
                            title={formatAbsoluteTime(event.time, locale)}
                            className="cursor-help"
                          >
                            {formatRelativeTime(event.time, locale)}
                          </span>
                          {event.executionid && (
                            <span>Execution: {event.executionid.slice(0, 8)}...</span>
                          )}
                          {event.taskid && (
                            <span>Task ID: {event.taskid.slice(-8)}</span>
                          )}
                          {taskName && event.type.includes('task.') && (
                            <span className="font-medium text-foreground">→ {taskName}</span>
                          )}
                        </div>
                      </div>
                    </div>

                  {/* Task-specific information */}
                  {taskName && event.type.includes('task.') && (
                    <div className="mt-2 p-2 bg-muted/50 rounded-md">
                      <div className="flex items-center gap-2 text-xs">
                        <span className="font-medium">Task:</span>
                        <span className="font-mono bg-background px-1 rounded">{taskName}</span>
                        {event.data?.task_type && (
                          <>
                            <span className="text-muted-foreground">•</span>
                            <span>Type: {event.data.task_type}</span>
                          </>
                        )}
                        {event.data?.duration_ms && (
                          <>
                            <span className="text-muted-foreground">•</span>
                            <span>Duration: {event.data.duration_ms}ms</span>
                          </>
                        )}
                        {event.data?.success !== undefined && (
                          <>
                            <span className="text-muted-foreground">•</span>
                            <span className={event.data.success ? 'text-green-600' : 'text-red-600'}>
                              {event.data.success ? 'Success' : 'Failed'}
                            </span>
                          </>
                        )}
                      </div>
                    </div>
                  )}

                  {/* Event data */}
                  {event.data && Object.keys(event.data).length > 0 && (
                    <div className="mt-2">
                      <details className="group">
                        <summary className="cursor-pointer text-xs text-muted-foreground hover:text-foreground">
                          View full event data
                        </summary>
                        <div className="mt-2 p-3 bg-muted rounded-md">
                          <pre className="text-xs overflow-x-auto">
                            {JSON.stringify(event.data, null, 2)}
                          </pre>
                        </div>
                      </details>
                    </div>
                  )}
                </div>
              </div>
              )
            })}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
