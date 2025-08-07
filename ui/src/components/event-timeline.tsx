'use client'

import { WorkflowEvent } from '@/lib/api'
import { useLocale, useTranslations } from 'next-intl'
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
import { useState } from 'react'

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
    if (event.data.data?.task_name) return event.data.data.task_name
  }

  return null
}

// Check if this event is part of a task pair (requested + completed)
function findTaskPair(event: WorkflowEvent, events: WorkflowEvent[]): WorkflowEvent | null {
  if (!isTaskEvent(event.type)) return null

  const taskName = extractTaskName(event)
  if (!taskName) return null

  if (event.type.includes('requested')) {
    // Find the corresponding completed event
    return events.find(e =>
      e.type.includes('completed') &&
      extractTaskName(e) === taskName
    ) || null
  } else if (event.type.includes('completed')) {
    // Find the corresponding requested event
    return events.find(e =>
      e.type.includes('requested') &&
      extractTaskName(e) === taskName
    ) || null
  }

  return null
}

function extractInputData(event: WorkflowEvent): Record<string, any> | null {
  if (!event.data) return null

  // For workflow execution started events
  if (event.type.includes('execution.started') && event.data.input) {
    return event.data.input
  }

  // For task events - try to get meaningful business data instead of internal metadata
  if (event.type.includes('task.requested')) {
    // For set tasks, the input is usually the workflow context/variables available to the task
    // Look for actual business data in config parameters
    if (event.data.config?.parameters) {
      const params = event.data.config.parameters
      // Filter out internal metadata and show only business data
      const businessData: Record<string, any> = {}

      // For set tasks, show what variables are being set
      if (params.set_data) {
        return { variables_to_set: params.set_data }
      }

      // For other task types, show the parameters
      Object.keys(params).forEach(key => {
        if (!key.includes('_data') && !key.includes('metadata')) {
          businessData[key] = params[key]
        }
      })

      if (Object.keys(businessData).length > 0) {
        return businessData
      }
    }

    // Fallback to original input data
    if (event.data.input_data) return event.data.input_data
    if (event.data.input) return event.data.input
  }

  return null
}

function extractOutputData(event: WorkflowEvent): Record<string, any> | null {
  if (!event.data) return null

  // For workflow completion events
  if (event.type.includes('workflow.completed') && event.data.output) {
    return event.data.output
  }

  // For task completion events - extract meaningful business data
  if (event.type.includes('task.completed')) {
    // Try to get the actual values that were set/produced by the task

    // Check for set_data in the nested output (this contains the actual values set)
    if (event.data.data?.output?.set_data) {
      return event.data.data.output.set_data
    }

    // Check for set_data in direct output
    if (event.data.output?.set_data) {
      return event.data.output.set_data
    }

    // For wait tasks, show the wait completion data
    if (event.data.output && Object.keys(event.data.output).some(key =>
      key.includes('duration') || key.includes('waited') || key.includes('completed_at')
    )) {
      return event.data.output
    }

    // Final fallbacks
    if (event.data.output_data) return event.data.output_data
    if (event.data.output) return event.data.output
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

interface DataDisplayProps {
  title: string
  data: Record<string, any>
  variant?: 'input' | 'output' | 'default'
}

function DataDisplay({ title, data, variant = 'default' }: DataDisplayProps) {
  const [isExpanded, setIsExpanded] = useState(false)
  const t = useTranslations('eventTimeline')

  const getVariantStyles = () => {
    switch (variant) {
      case 'input':
        return 'border-blue-200 bg-blue-50/50'
      case 'output':
        return 'border-green-200 bg-green-50/50'
      default:
        return 'border-gray-200 bg-gray-50/50'
    }
  }

  const getIconColor = () => {
    switch (variant) {
      case 'input':
        return 'text-blue-600'
      case 'output':
        return 'text-green-600'
      default:
        return 'text-gray-600'
    }
  }

  const fieldCount = Object.keys(data).length
  const fieldsText = fieldCount === 1 ? t('fields') : t('fields_plural')

  return (
    <div className={`mt-2 border rounded-md ${getVariantStyles()}`}>
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className="w-full px-3 py-2 text-left flex items-center justify-between hover:bg-white/50 transition-colors"
      >
        <div className="flex items-center gap-2">
          <Database className={`h-3 w-3 ${getIconColor()}`} />
          <span className="text-xs font-medium">{title}</span>
          <Badge variant="outline" className="text-xs">
            {fieldCount} {fieldsText}
          </Badge>
        </div>
        <div className={`transform transition-transform ${isExpanded ? 'rotate-90' : ''}`}>
          <span className="text-xs">▶</span>
        </div>
      </button>

      {isExpanded && (
        <div className="px-3 pb-3">
          <div className="bg-white rounded border p-2">
            <pre className="text-xs overflow-x-auto whitespace-pre-wrap">
              {JSON.stringify(data, null, 2)}
            </pre>
          </div>
        </div>
      )}
    </div>
  )
}

export function EventTimeline({ events, isLoading, error }: EventTimelineProps) {
  const locale = useLocale()
  const t = useTranslations('eventTimeline')

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-lg flex items-center gap-2">
            <Activity className="h-5 w-5" />
            {t('title')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-center py-8">
            <div className="text-center">
              <AlertCircle className="h-8 w-8 text-muted-foreground mx-auto mb-2" />
              <p className="text-muted-foreground">{t('failedToLoad')}</p>
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
          {t('title')}
          {isLoading && <Loader2 className="h-4 w-4 animate-spin" />}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {events.length === 0 ? (
          <div className="flex items-center justify-center py-8">
            <div className="text-center">
              <Activity className="h-8 w-8 text-muted-foreground mx-auto mb-2" />
              <p className="text-muted-foreground">{t('noEvents')}</p>
              <p className="text-sm text-muted-foreground">{t('noEventsDescription')}</p>
            </div>
          </div>
        ) : (
          <div className="space-y-4">
            {events.map((event, index) => {
              const { title, subtitle } = getEventTitle(event)
              const taskName = extractTaskName(event)
              const inputData = extractInputData(event)
              const outputData = extractOutputData(event)
              const taskPair = findTaskPair(event, events)

              // For task events, check if we should render as a grouped card
              const isTaskEvent = event.type.includes('task.')
              const isRequestedEvent = event.type.includes('requested')
              const shouldRenderAsGroup = isTaskEvent && isRequestedEvent && taskPair

              if (shouldRenderAsGroup) {
                // Render task pair as a grouped card
                const requestedEvent = event
                const completedEvent = taskPair

                return (
                  <div key={`task-pair-${taskName}-${index}`} className="flex gap-4">
                    {/* Timeline line */}
                    <div className="flex flex-col items-center">
                      <div className={`w-8 h-8 rounded-full ${completedEvent ? 'bg-green-600' : 'bg-yellow-500'} flex items-center justify-center text-white`}>
                        {completedEvent ? <CheckCircle className="h-4 w-4" /> : <Clock className="h-4 w-4" />}
                      </div>
                      {index < events.length - 1 && (
                        <div className="w-0.5 h-6 bg-border mt-2" />
                      )}
                    </div>

                    {/* Task card content */}
                    <div className="flex-1 min-w-0 pb-4">
                      <div className="border rounded-lg p-3 bg-blue-50/50 border-blue-200">
                        <div className="space-y-2">
                          <div className="flex items-center gap-2 flex-wrap">
                            <h4 className="font-medium text-sm text-blue-700 flex items-center gap-1">
                              {/* Special icon for wait tasks */}
                              {requestedEvent.data?.task_type === 'wait' && (
                                <Clock className="h-4 w-4 text-orange-500" />
                              )}
                              Task: {taskName}
                            </h4>
                            <Badge
                              variant={completedEvent ? "default" : "secondary"}
                              className={`text-xs ${completedEvent ? 'bg-green-100 text-green-700 hover:bg-green-200' : ''}`}
                            >
                              {completedEvent ? "Completed" : "In Progress"}
                            </Badge>
                            {/* Show wait duration for wait tasks */}
                            {requestedEvent.data?.task_type === 'wait' && requestedEvent.data?.config?.parameters?.duration && (
                              <Badge variant="outline" className="text-xs text-orange-600 border-orange-300">
                                ⏱️ {requestedEvent.data.config.parameters.duration}
                              </Badge>
                            )}
                          </div>

                          {/* Show both events in the card */}
                          <div className="space-y-2">
                            <div className="flex items-center gap-2 text-xs">
                              <Clock className="h-3 w-3 text-yellow-600" />
                              <span className="font-medium">Started:</span>
                              <span className="cursor-help" title={formatAbsoluteTime(requestedEvent.time, locale)}>
                                {formatRelativeTime(requestedEvent.time, locale)}
                              </span>
                            </div>
                            {completedEvent && (
                              <div className="flex items-center gap-2 text-xs">
                                <CheckCircle className="h-3 w-3 text-green-600" />
                                <span className="font-medium">Completed:</span>
                                <span className="cursor-help" title={formatAbsoluteTime(completedEvent.time, locale)}>
                                  {formatRelativeTime(completedEvent.time, locale)}
                                </span>
                              </div>
                            )}
                          </div>

                          {/* Task Input/Output Data */}
                          {(() => {
                            const requestedInputData = extractInputData(requestedEvent)
                            const completedOutputData = completedEvent ? extractOutputData(completedEvent) : null

                            return (
                              <>
                                {requestedInputData && (
                                  <DataDisplay
                                    title={t('taskInput')}
                                    data={requestedInputData}
                                    variant="input"
                                  />
                                )}
                                {completedOutputData && (
                                  <DataDisplay
                                    title={t('taskOutput')}
                                    data={completedOutputData}
                                    variant="output"
                                  />
                                )}
                              </>
                            )
                          })()}
                        </div>
                      </div>
                    </div>
                  </div>
                )
              }

              // Skip completed events that are already shown in pairs
              if (isTaskEvent && event.type.includes('completed') && findTaskPair(event, events)) {
                return null
              }

              // Render individual events (non-task events or single task events)
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
                  <div className="flex-1 min-w-0 pb-4">
                    <div className="flex items-start justify-between">
                      <div className="space-y-1">
                        <div className="flex items-center gap-2 flex-wrap">
                          <h4 className="font-medium text-sm">
                            {title}
                          </h4>
                          {subtitle && (
                            <Badge variant="secondary" className="text-xs">
                              {subtitle}
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
                        </div>
                      </div>
                    </div>

                    {/* Input/Output Data Display */}
                    {inputData && (
                      <DataDisplay
                        title={event.type.includes('execution.started') ? t('workflowInput') : t('taskInput')}
                        data={inputData}
                        variant="input"
                      />
                    )}

                    {outputData && (
                      <DataDisplay
                        title={event.type.includes('workflow.completed') ? t('workflowOutput') : t('taskOutput')}
                        data={outputData}
                        variant="output"
                      />
                    )}

                    {/* Raw event data - only show if no input/output data is displayed */}
                    {event.data && Object.keys(event.data).length > 0 && !inputData && !outputData && (
                      <div className="mt-2">
                        <details className="group">
                          <summary className="cursor-pointer text-xs text-muted-foreground hover:text-foreground">
                            {t('viewEventData')}
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
