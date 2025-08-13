'use client'

import { useState } from 'react'
import { WorkflowEvent } from '@/lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Activity,
  Clock,
  Loader2,
  AlertCircle,
  CheckCircle,
  Play,
  Square,
  FileText,
  Database,
  RotateCcw,
  Shield,
  XCircle,
  Copy,
  Check
} from 'lucide-react'
import { formatDatePairCompact } from '@/lib/utils'

// Copy button component
function CopyButton({ data, label = "Copy JSON" }: { data: any, label?: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(JSON.stringify(data, null, 2))
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch (err) {
      console.error('Failed to copy:', err)
    }
  }

  return (
    <Button
      variant="outline"
      size="sm"
      onClick={handleCopy}
      className="h-6 px-2 text-xs"
    >
      {copied ? (
        <>
          <Check className="h-3 w-3 mr-1" />
          Copied!
        </>
      ) : (
        <>
          <Copy className="h-3 w-3 mr-1" />
          {label}
        </>
      )}
    </Button>
  )
}

import { useTranslations } from 'next-intl'

interface EventTimelineProps {
  events: WorkflowEvent[]
  isLoading?: boolean
  error?: Error | null
  locale?: string
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
  const d = event.data as Record<string, unknown> | undefined
  if (!d) return null

  // Try different ways to extract task name from event data
  const tn = (d as Record<string, unknown>)['task_name']
  if (typeof tn === 'string') return tn
  const tName = (d as Record<string, unknown>)['taskName']
  if (typeof tName === 'string') return tName

  // For task events, also check nested data
  if (event.type.includes('task.')) {
    const input = d['input'] as Record<string, unknown> | undefined
    const output = d['output'] as Record<string, unknown> | undefined
    const data = d['data'] as Record<string, unknown> | undefined
    const inName = input && typeof input['task_name'] === 'string' ? (input['task_name'] as string) : null
    if (inName) return inName
    const outName = output && typeof output['task_name'] === 'string' ? (output['task_name'] as string) : null
    if (outName) return outName
    const dataName = data && typeof data['task_name'] === 'string' ? (data['task_name'] as string) : null
    if (dataName) return dataName
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

// Helper function to get all retry attempts for a task
function getTaskRetryAttempts(taskName: string, events: WorkflowEvent[]): WorkflowEvent[] {
  return events.filter(e =>
    e.type.includes('task.completed') &&
    extractTaskName(e) === taskName
  ).sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime())
}

// Helper function to extract task success status and error message
function getTaskCompletionInfo(event: WorkflowEvent): { success: boolean; error?: string; retryCount?: number } {
  const data = event.data as Record<string, unknown> | undefined
  const success = data?.success !== false // Default to true for backward compatibility
  const error = typeof data?.error === 'string' ? data.error : undefined

  // Try to extract retry count from error context or metadata
  const retryCount = typeof data?.retry_count === 'number' ? data.retry_count : undefined

  return { success, error, retryCount }
}

// Helper function to extract retry policy from try task configuration
function extractRetryPolicy(event: WorkflowEvent): { maxAttempts?: number; backoffType?: string } | null {
  const data = event.data as Record<string, unknown> | undefined
  const config = data?.config as Record<string, unknown> | undefined
  const parameters = config?.parameters as Record<string, unknown> | undefined
  const catchBlock = parameters?.catch as Record<string, unknown> | undefined
  const retry = catchBlock?.retry as Record<string, unknown> | undefined
  const limit = retry?.limit as Record<string, unknown> | undefined
  const attempt = limit?.attempt as Record<string, unknown> | undefined
  const maxAttempts = typeof attempt?.count === 'number' ? attempt.count : undefined

  // Extract backoff type
  const backoff = retry?.backoff as Record<string, unknown> | undefined
  let backoffType = 'constant'
  if (backoff?.exponential) backoffType = 'exponential'
  else if (backoff?.linear) backoffType = 'linear'

  return maxAttempts ? { maxAttempts, backoffType } : null
}

function extractInputData(event: WorkflowEvent): Record<string, JsonValue> | null {
  if (!event.data) return null

  // For workflow execution started events
  if (event.type.includes('execution.started') && event.data.input) {
    return event.data.input as Record<string, JsonValue>
  }

  // For task events - try to get meaningful business data instead of internal metadata
  if (event.type.includes('task.requested')) {
    // For set tasks, the input is usually the workflow context/variables available to the task
    // Look for actual business data in config parameters
    const cfg = (event.data as Record<string, unknown>)['config'] as Record<string, unknown> | undefined
    const params = cfg?.['parameters'] as Record<string, JsonValue> | undefined
    if (params) {
      // Filter out internal metadata and show only business data
      const businessData: Record<string, JsonValue> = {}

      // For set tasks, show what variables are being set
      if (Object.prototype.hasOwnProperty.call(params, 'set_data')) {
        const v = (params as Record<string, JsonValue>)['set_data']
        return { variables_to_set: v as JsonValue }
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
    if (Object.prototype.hasOwnProperty.call(event.data as Record<string, unknown>, 'input_data')) return (event.data as Record<string, JsonValue>)['input_data'] as Record<string, JsonValue>
    if (event.data.input) return event.data.input as Record<string, JsonValue>
  }

  return null
}

function extractOutputData(event: WorkflowEvent): Record<string, JsonValue> | null {
  if (!event.data) return null

  // For workflow completion events
  if (event.type.includes('workflow.completed') && event.data.output) {
    const output = event.data.output as Record<string, JsonValue>

    // If there's a transformed 'result' field, show only that
    if (output.result !== undefined) {
      return { result: output.result }
    }

    // Otherwise, return the full output
    return output
  }

  // For task completion events - extract meaningful business data
  if (event.type.includes('task.completed')) {
    // Try to get the actual values that were set/produced by the task

    // Check for set_data in the nested output (this contains the actual values set)
    if (Object.prototype.hasOwnProperty.call((event.data as Record<string, unknown>)?.['data'] as Record<string, unknown> || {}, 'output')) {
      const out = ((event.data as Record<string, unknown>)['data'] as Record<string, unknown>)['output'] as Record<string, unknown>
      if (Object.prototype.hasOwnProperty.call(out || {}, 'set_data')) {
        return (out as Record<string, JsonValue>)['set_data'] as Record<string, JsonValue>
      }
    }

    // Check for set_data in direct output
    const out2 = (event.data as Record<string, unknown>)['output'] as Record<string, unknown> | undefined
    if (out2 && Object.prototype.hasOwnProperty.call(out2, 'set_data')) {
      return (out2 as Record<string, JsonValue>)['set_data'] as Record<string, JsonValue>
    }

    // For wait tasks, show the wait completion data
    if (event.data.output && Object.keys(event.data.output).some(key =>
      key.includes('duration') || key.includes('waited') || key.includes('completed_at')
    )) {
      return event.data.output as Record<string, JsonValue>
    }

    // Final fallbacks
    if (Object.prototype.hasOwnProperty.call(event.data as Record<string, unknown>, 'output_data')) return (event.data as Record<string, JsonValue>)['output_data'] as Record<string, JsonValue>
    if (event.data.output) {
      const output = event.data.output as Record<string, JsonValue>

      // If there's a transformed 'result' field, show only that
      if (output.result !== undefined) {
        return { result: output.result }
      }

      // Otherwise, return the full output
      return output
    }
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

type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue }

interface DataDisplayProps {
  title: string
  data: Record<string, JsonValue>
  variant?: 'input' | 'output' | 'default'
}

function DataDisplay({ title, data, variant = 'default' }: DataDisplayProps) {
  const [isExpanded, setIsExpanded] = useState(false)

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
  const fieldsText = fieldCount === 1 ? 'field' : 'fields'

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
            <div className="flex justify-between items-start mb-2">
              <span className="text-xs font-medium text-gray-600">JSON Data</span>
              <CopyButton data={data} />
            </div>
            <pre className="text-xs overflow-x-auto whitespace-pre-wrap">
              {JSON.stringify(data, null, 2)}
            </pre>
          </div>
        </div>
      )}
    </div>
  )
}

export function EventTimeline({ events, isLoading, error, locale = 'en' }: EventTimelineProps) {
  const t = useTranslations()

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-lg flex items-center gap-2">
            <Activity className="h-5 w-5" />
            {t('eventTimeline.title')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-center py-8">
            <div className="text-center">
              <AlertCircle className="h-8 w-8 text-muted-foreground mx-auto mb-2" />
              <p className="text-muted-foreground">{t('eventTimeline.failedToLoad')}</p>
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
          {t('eventTimeline.title')}
          {isLoading && <Loader2 className="h-4 w-4 animate-spin" />}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {events.length === 0 ? (
          <div className="flex items-center justify-center py-8">
            <div className="text-center">
              <Activity className="h-8 w-8 text-muted-foreground mx-auto mb-2" />
              <p className="text-muted-foreground">{t('eventTimeline.noEvents')}</p>
              <p className="text-sm text-muted-foreground">{t('eventTimeline.noEventsDescription')}</p>
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
                      {(() => {
                        if (!completedEvent) {
                          return (
                            <div className="w-8 h-8 rounded-full bg-yellow-500 flex items-center justify-center text-white">
                              <Clock className="h-4 w-4" />
                            </div>
                          )
                        }

                        const completionInfo = getTaskCompletionInfo(completedEvent)
                        const bgColor = completionInfo.success ? 'bg-green-600' : 'bg-red-600'
                        const icon = completionInfo.success ?
                          <CheckCircle className="h-4 w-4" /> :
                          <XCircle className="h-4 w-4" />

                        return (
                          <div className={`w-8 h-8 rounded-full ${bgColor} flex items-center justify-center text-white`}>
                            {icon}
                          </div>
                        )
                      })()}
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
                              {/* Special icon for different task types */}
                              {requestedEvent.data?.task_type === 'wait' && (
                                <Clock className="h-4 w-4 text-orange-500" />
                              )}
                              {requestedEvent.data?.task_type === 'try' && (
                                <Shield className="h-4 w-4 text-blue-500" />
                              )}
                              {t('eventTimeline.task') } {taskName}
                            </h4>

                            {/* Task status badge with success/failure indication */}
                            {(() => {
                              if (!completedEvent) {
                                return (
                                  <Badge variant="secondary" className="text-xs">
                                    {t('status.running')}
                                  </Badge>
                                )
                              }

                              const completionInfo = getTaskCompletionInfo(completedEvent)
                              const retryAttempts = taskName ? getTaskRetryAttempts(taskName, events) : []
                              const currentAttempt = retryAttempts.length
                              const retryPolicy = extractRetryPolicy(requestedEvent)

                              return (
                                <>
                                  <Badge
                                    variant={completionInfo.success ? "default" : "destructive"}
                                    className={`text-xs ${
                                      completionInfo.success
                                        ? 'bg-green-100 text-green-700 hover:bg-green-200'
                                        : 'bg-red-100 text-red-700 hover:bg-red-200'
                                    }`}
                                  >
                                    {completionInfo.success ? (
                                      <>
                                        <CheckCircle className="h-3 w-3 mr-1" />
                                        {t('eventTimeline.success')}
                                      </>
                                    ) : (
                                      <>
                                        <XCircle className="h-3 w-3 mr-1" />
                                        {t('eventTimeline.failed')}
                                      </>
                                    )}
                                  </Badge>

                                  {/* Show retry count if there were multiple attempts */}
                                  {currentAttempt > 1 && (
                                    <Badge variant="outline" className="text-xs text-orange-600 border-orange-300">
                                      <RotateCcw className="h-3 w-3 mr-1" />
                                      Attempt {currentAttempt}
                                      {retryPolicy?.maxAttempts && ` / ${retryPolicy.maxAttempts}`}
                                    </Badge>
                                  )}

                                  {/* Show retry policy info for try tasks */}
                                  {requestedEvent.data?.task_type === 'try' && retryPolicy && (
                                    <Badge variant="outline" className="text-xs text-blue-600 border-blue-300">
                                      Max: {retryPolicy.maxAttempts} ({retryPolicy.backoffType})
                                    </Badge>
                                  )}
                                </>
                              )
                            })()}
                            {/* Show wait duration for wait tasks */}
                            {(() => {
                              const rd = requestedEvent.data as Record<string, unknown> | undefined
                              const cfg = rd?.['config'] as Record<string, unknown> | undefined
                              const dur = (cfg?.['parameters'] as Record<string, unknown> | undefined)?.['duration']
                              if (rd?.['task_type'] === 'wait' && typeof dur !== 'undefined') {
                                return (
                                  <Badge variant="outline" className="text-xs text-orange-600 border-orange-300">
                                    ⏱️ {String(dur)}
                                  </Badge>
                                )
                              }
                              return null
                            })()}
                          </div>

                          {/* Show both events in the card */}
                          <div className="space-y-2">
                            <div className="flex items-center gap-2 text-xs">
                              <Clock className="h-3 w-3 text-yellow-600" />
                              <span className="font-medium">{t('executions.startedAt')}:</span>
                              <span className="cursor-help">
                                {formatDatePairCompact(requestedEvent.time, locale)}
                              </span>
                            </div>
                            {completedEvent && (
                              <div className="flex items-center gap-2 text-xs">
                                <CheckCircle className="h-3 w-3 text-green-600" />
                                <span className="font-medium">{t('common.completed')}:</span>
                                <span className="cursor-help">
                                  {formatDatePairCompact(completedEvent.time, locale)}
                                </span>
                              </div>
                            )}
                          </div>

                          {/* Error message display for failed tasks */}
                          {(() => {
                            if (!completedEvent) return null

                            const completionInfo = getTaskCompletionInfo(completedEvent)
                            if (completionInfo.success || !completionInfo.error) return null

                            return (
                              <div className="mt-2 p-2 bg-red-50 border border-red-200 rounded-md">
                                <div className="flex items-start gap-2">
                                  <AlertCircle className="h-4 w-4 text-red-500 mt-0.5 flex-shrink-0" />
                                  <div className="flex-1 min-w-0">
                                    <div className="text-xs font-medium text-red-700 mb-1">
                                      Task Failed
                                    </div>
                                    <div className="text-xs text-red-600 break-words">
                                      {completionInfo.error}
                                    </div>
                                  </div>
                                </div>
                              </div>
                            )
                          })()}

                          {/* Task Input/Output Data */}
                          {(() => {
                            const requestedInputData = extractInputData(requestedEvent)
                            const completedOutputData = completedEvent ? extractOutputData(completedEvent) : null

                            return (
                              <>
                                {requestedInputData && (
                                  <DataDisplay
                                    title={t('eventTimeline.taskInput')}
                                    data={requestedInputData}
                                    variant="input"
                                  />
                                )}
                                {completedOutputData && (
                                  <DataDisplay
                                    title={t('eventTimeline.taskOutput')}
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
                            className="cursor-help"
                          >
                            {formatDatePairCompact(event.time, locale)}
                          </span>
                          {event.executionid && (
                            <span>{t('executions.executionId')}: {event.executionid.slice(0, 8)}...</span>
                          )}
                        </div>
                      </div>
                    </div>

                    {/* Input/Output Data Display */}
                    {inputData && (
                      <DataDisplay
                        title={event.type.includes('execution.started') ? t('eventTimeline.workflowInput') : t('eventTimeline.taskInput')}
                        data={inputData}
                        variant="input"
                      />
                    )}

                    {outputData && (
                      <DataDisplay
                        title={event.type.includes('workflow.completed') ? t('eventTimeline.workflowOutput') : t('eventTimeline.taskOutput')}
                        data={outputData}
                        variant="output"
                      />
                    )}

                    {/* Raw event data - only show if no input/output data is displayed */}
                    {event.data && Object.keys(event.data).length > 0 && !inputData && !outputData && (
                      <div className="mt-2">
                        <details className="group">
                          <summary className="cursor-pointer text-xs text-muted-foreground hover:text-foreground">
                            {t('eventTimeline.viewEventData')}
                          </summary>
                          <div className="mt-2 p-3 bg-muted rounded-md">
                            <div className="flex justify-between items-start mb-2">
                              <span className="text-xs font-medium text-muted-foreground">Event Data</span>
                              <CopyButton data={event.data} label="Copy" />
                            </div>
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
