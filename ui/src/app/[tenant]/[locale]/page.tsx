'use client'

import { useQuery } from '@tanstack/react-query'
import { createTenantApiClient } from '@/lib/api'
import { ExecutionList } from '@/components/execution-list'
import { ClientOnly } from '@/components/client-only'
import { Loader2 } from 'lucide-react'
import { useParams } from 'next/navigation'

export default function TenantDashboard() {
  const params = useParams<{ tenant: string; locale: string }>()
  const tenant = String(params.tenant)
  const locale = String(params.locale)

  // Create tenant-aware API client
  const apiClient = createTenantApiClient(tenant);

  const { data: executions, isLoading, error } = useQuery({
    queryKey: ['executions', tenant],
    queryFn: () => apiClient.getExecutions({ limit: 50 }),
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-96">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-96">
        <div className="text-center">
          <h2 className="text-lg font-semibold text-foreground mb-2">Failed to load executions</h2>
          <p className="text-muted-foreground">{error.message}</p>
        </div>
      </div>
    )
  }

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-foreground mb-2">
          Workflow Executions - {tenant}
        </h1>
        <p className="text-muted-foreground">
          Monitor and track your workflow execution instances
        </p>
      </div>

      <ClientOnly fallback={
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      }>
        <ExecutionList executions={executions?.items || []} tenant={tenant} locale={locale} />
      </ClientOnly>
    </div>
  )
}
