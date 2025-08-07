'use client'

import { useQuery } from '@tanstack/react-query'
import { useTranslations, useLocale } from 'next-intl'
import { useParams } from 'next/navigation'
import Link from 'next/link'
import { apiClient } from '@/lib/api'
import { Header } from '@/components/header'
import { ClientOnly } from '@/components/client-only'
import { EventTimeline } from '@/components/event-timeline'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import {
  ArrowLeft,
  Calendar,
  Clock,
  Play,
  Pause,
  Square,
  AlertCircle,
  CheckCircle,
  Timer,
  Workflow,
  Loader2,
  Activity,
  FileText,
  Settings
} from 'lucide-react'
import { formatRelativeTime, formatAbsoluteTime } from '@/lib/utils'

function getStatusIcon(status: string) {
  switch (status) {
    case 'pending':
      return <Timer className="h-5 w-5" />
    case 'running':
      return <Play className="h-5 w-5" />
    case 'waiting':
      return <Clock className="h-5 w-5" />
    case 'suspended':
      return <Pause className="h-5 w-5" />
    case 'cancelled':
      return <Square className="h-5 w-5" />
    case 'faulted':
      return <AlertCircle className="h-5 w-5" />
    case 'completed':
      return <CheckCircle className="h-5 w-5" />
    default:
      return <Timer className="h-5 w-5" />
  }
}

function getStatusVariant(status: string) {
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

export default function ExecutionDetailPage() {
  const params = useParams()
  const executionId = params.id as string
  const t = useTranslations()
  const locale = useLocale()

  // Get execution details
  const { data: execution, isLoading: executionLoading, error: executionError } = useQuery({
    queryKey: ['execution', executionId],
    queryFn: () => apiClient.getExecution(executionId),
    retry: 1
  })

  // Get execution events
  const { data: eventsData, isLoading: eventsLoading, error: eventsError } = useQuery({
    queryKey: ['execution-events', executionId],
    queryFn: () => apiClient.getExecutionEvents(executionId),
    retry: 1,
    refetchInterval: execution?.status === 'running' ? 2000 : false // Auto-refresh for running executions
  })

  if (executionLoading) {
    return (
      <div className="min-h-screen bg-background">
        <Header />
        <main className="container mx-auto px-4 py-8">
          <div className="flex items-center justify-center h-96">
            <div className="text-center">
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground mx-auto mb-4" />
              <p className="text-muted-foreground">{t('common.loading')}</p>
            </div>
          </div>
        </main>
      </div>
    )
  }

  if (executionError) {
    return (
      <div className="min-h-screen bg-background">
        <Header />
        <main className="container mx-auto px-4 py-8">
          <div className="flex items-center justify-center h-96">
            <div className="text-center">
              <h2 className="text-lg font-semibold text-foreground mb-2">Execution not found</h2>
              <p className="text-muted-foreground mb-4">The requested execution could not be found.</p>
              <Button asChild>
                <Link href={`/${locale}`}>
                  <ArrowLeft className="h-4 w-4 mr-2" />
                  Back to Executions
                </Link>
              </Button>
            </div>
          </div>
        </main>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="container mx-auto px-4 py-8">
        {/* Header */}
        <div className="mb-8">
          <div className="flex items-center gap-4 mb-4">
            <Button variant="outline" size="sm" asChild>
              <Link href={`/${locale}`}>
                <ArrowLeft className="h-4 w-4 mr-2" />
                Back to Executions
              </Link>
            </Button>
          </div>
          
          <div className="flex items-start justify-between">
            <div>
              <h1 className="text-3xl font-bold text-foreground mb-2">
                Execution Details
              </h1>
              <p className="text-muted-foreground font-mono text-sm">
                {executionId}
              </p>
            </div>
            
            {execution && (
              <Badge variant={getStatusVariant(execution.status)} className="flex items-center gap-2 text-sm px-3 py-1">
                {getStatusIcon(execution.status)}
                {execution.status}
              </Badge>
            )}
          </div>
        </div>

        {execution && (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
            {/* Main Content */}
            <div className="lg:col-span-2 space-y-6">
              {/* Execution Overview */}
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Activity className="h-5 w-5" />
                    Execution Overview
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div>
                      <label className="text-sm font-medium text-muted-foreground">Workflow</label>
                      <div className="flex items-center gap-2 mt-1">
                        <Workflow className="h-4 w-4 text-muted-foreground" />
                        <span className="font-mono text-sm">{execution.workflow_id}</span>
                      </div>
                    </div>
                    
                    <div>
                      <label className="text-sm font-medium text-muted-foreground">Started</label>
                      <div className="flex items-center gap-2 mt-1">
                        <Calendar className="h-4 w-4 text-muted-foreground" />
                        <span 
                          className="text-sm cursor-help"
                          title={formatAbsoluteTime(execution.started_at, locale)}
                        >
                          {formatRelativeTime(execution.started_at, locale)}
                        </span>
                      </div>
                    </div>
                    
                    {execution.duration_ms && (
                      <div>
                        <label className="text-sm font-medium text-muted-foreground">Duration</label>
                        <div className="flex items-center gap-2 mt-1">
                          <Clock className="h-4 w-4 text-muted-foreground" />
                          <span className="text-sm">{formatDuration(execution.duration_ms)}</span>
                        </div>
                      </div>
                    )}
                    
                    {execution.current_state && (
                      <div>
                        <label className="text-sm font-medium text-muted-foreground">Current Step</label>
                        <div className="flex items-center gap-2 mt-1">
                          <Settings className="h-4 w-4 text-muted-foreground" />
                          <span className="text-sm font-medium">{execution.current_state}</span>
                        </div>
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>

              {/* Event Timeline */}
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <FileText className="h-5 w-5" />
                    Event Timeline
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <ClientOnly fallback={
                    <div className="flex items-center justify-center py-12">
                      <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                    </div>
                  }>
                    {eventsLoading ? (
                      <div className="flex items-center justify-center py-12">
                        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                      </div>
                    ) : eventsError ? (
                      <div className="text-center py-12">
                        <p className="text-muted-foreground">Failed to load events</p>
                      </div>
                    ) : (
                      <EventTimeline events={eventsData?.events || []} />
                    )}
                  </ClientOnly>
                </CardContent>
              </Card>
            </div>

            {/* Sidebar */}
            <div className="space-y-6">
              {/* Execution Metadata */}
              <Card>
                <CardHeader>
                  <CardTitle className="text-lg">Metadata</CardTitle>
                </CardHeader>
                <CardContent className="space-y-3">
                  <div>
                    <label className="text-sm font-medium text-muted-foreground">Execution ID</label>
                    <p className="font-mono text-sm mt-1 break-all">{execution.id}</p>
                  </div>
                  
                  <Separator />
                  
                  <div>
                    <label className="text-sm font-medium text-muted-foreground">Workflow ID</label>
                    <p className="font-mono text-sm mt-1 break-all">{execution.workflow_id}</p>
                  </div>
                  
                  <Separator />
                  
                  <div>
                    <label className="text-sm font-medium text-muted-foreground">Status</label>
                    <div className="mt-1">
                      <Badge variant={getStatusVariant(execution.status)} className="flex items-center gap-1 w-fit">
                        {getStatusIcon(execution.status)}
                        {execution.status}
                      </Badge>
                    </div>
                  </div>
                  
                  {execution.current_state && (
                    <>
                      <Separator />
                      <div>
                        <label className="text-sm font-medium text-muted-foreground">Current State</label>
                        <p className="text-sm mt-1 font-medium">{execution.current_state}</p>
                      </div>
                    </>
                  )}
                </CardContent>
              </Card>

              {/* Event Summary */}
              {eventsData && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg">Event Summary</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="text-center">
                      <div className="text-2xl font-bold text-foreground">{eventsData.count}</div>
                      <div className="text-sm text-muted-foreground">Total Events</div>
                    </div>
                  </CardContent>
                </Card>
              )}
            </div>
          </div>
        )}
      </main>
    </div>
  )
}
