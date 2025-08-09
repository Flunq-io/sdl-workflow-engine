'use client'

import { useQuery } from '@tanstack/react-query'
import { createTenantApiClient } from '@/lib/api'
import { EventTimeline } from '@/components/event-timeline'
import { ClientOnly } from '@/components/client-only'
import { Loader2, ArrowLeft } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import Link from 'next/link'
import { useParams } from 'next/navigation'
import { formatAbsoluteTime, formatAbsoluteTimeCompact, formatRelativeTime } from '@/lib/utils'
import { useTranslations } from 'next-intl'

function getStatusColor(status: string) {
  switch (status.toLowerCase()) {
    case 'completed':
      return 'bg-green-100 text-green-800';
    case 'running':
      return 'bg-blue-100 text-blue-800';
    case 'faulted':
      return 'bg-red-100 text-red-800';
    case 'pending':
      return 'bg-yellow-100 text-yellow-800';
    default:
      return 'bg-gray-100 text-gray-800';
  }
}

export default function ExecutionDetailPage() {
  const t = useTranslations()
  const params = useParams<{ tenant: string; locale: string; id: string }>()
  const tenant = String(params.tenant)
  const locale = String(params.locale)
  const id = String(params.id)

  // Create tenant-aware API client
  const apiClient = createTenantApiClient(tenant);

  const { data: execution, isLoading: executionLoading, error: executionError } = useQuery({
    queryKey: ['execution', tenant, id],
    queryFn: () => apiClient.getExecution(id),
    refetchInterval: 5000, // Auto-refresh every 5 seconds
  })

  const { data: events, isLoading: eventsLoading, error: eventsError } = useQuery({
    queryKey: ['execution-events', tenant, id],
    queryFn: () => apiClient.getExecutionEvents(id),
    refetchInterval: 5000, // Auto-refresh every 5 seconds
  })

  if (executionLoading) {
    return (
      <div className="flex items-center justify-center h-96">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (executionError) {
    return (
      <div className="flex items-center justify-center h-96">
        <div className="text-center">
          <h2 className="text-lg font-semibold text-foreground mb-2">Failed to load execution</h2>
          <p className="text-muted-foreground">{executionError.message}</p>
          <Button asChild className="mt-4">
            <Link href={`/${tenant}/${locale}/executions`}>
              <ArrowLeft className="h-4 w-4 mr-2" />
              Back to Executions
            </Link>
          </Button>
        </div>
      </div>
    )
  }

  if (!execution) {
    return (
      <div className="flex items-center justify-center h-96">
        <div className="text-center">
          <h2 className="text-lg font-semibold text-foreground mb-2">Execution not found</h2>
          <p className="text-muted-foreground">The execution with ID &quot;{id}&quot; could not be found.</p>
          <Button asChild className="mt-4">
            <Link href={`/${tenant}/${locale}/executions`}>
              <ArrowLeft className="h-4 w-4 mr-2" />
              Back to Executions
            </Link>
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-4 mb-2">
            <Button asChild variant="outline" size="sm">
              <Link href={`/${tenant}/${locale}/executions`}>
                <ArrowLeft className="h-4 w-4 mr-2" />
                {t('common.back')}
              </Link>
            </Button>
            <h1 className="text-2xl font-bold text-foreground">
              {t('executions.viewDetails')}
            </h1>
          </div>
          <p className="text-muted-foreground">
            {t('executions.executionId')}: <code className="bg-muted px-2 py-1 rounded text-sm">{execution.id}</code>
          </p>
        </div>
        <Badge className={getStatusColor(execution.status)}>
          {execution.status}
        </Badge>
      </div>

      {/* Execution Info Card */}
      <Card>
        <CardHeader>
          <CardTitle>{t('executions.viewDetails')}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="text-sm font-medium text-muted-foreground">{t('executions.workflowName')}</label>
              <p className="text-sm font-mono bg-muted px-2 py-1 rounded">{execution.workflow_id}</p>
            </div>
            <div>
              <label className="text-sm font-medium text-muted-foreground">{t('common.status')}</label>
              <div className="text-sm">
                <Badge className={getStatusColor(execution.status)}>
                  {execution.status}
                </Badge>
              </div>
            </div>
            <div>
              <label className="text-sm font-medium text-muted-foreground">{t('executions.startedAt')}</label>
              <p className="text-sm">
                {formatAbsoluteTimeCompact(execution.started_at, locale)} ({formatRelativeTime(execution.started_at, locale)})
              </p>
            </div>
            {execution.completed_at && (
              <div>
                <label className="text-sm font-medium text-muted-foreground">{t('common.completed')}</label>
                <p className="text-sm">
                  {formatAbsoluteTime(execution.completed_at, locale)} ({formatRelativeTime(execution.completed_at, locale)})
                </p>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Events Timeline */}
      <ClientOnly fallback={
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      }>
        <EventTimeline
          events={events?.events || []}
          isLoading={eventsLoading}
          error={eventsError}
          locale={locale}
        />
      </ClientOnly>
    </div>
  )
}
