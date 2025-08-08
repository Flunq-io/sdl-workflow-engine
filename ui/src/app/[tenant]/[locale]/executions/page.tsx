'use client'

import { useQuery } from '@tanstack/react-query'
import { createTenantApiClient } from '@/lib/api'
import { ExecutionList } from '@/components/execution-list'
import { ClientOnly } from '@/components/client-only'
import { Loader2, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useEffect } from 'react'

interface PageProps {
  params: {
    tenant: string;
    locale: string;
  };
}

export default function ExecutionsPage({ params }: PageProps) {
  const { tenant, locale } = params;

  // Create tenant-aware API client
  const apiClient = createTenantApiClient(tenant);
  
  const { data: executions, isLoading, error, refetch } = useQuery({
    queryKey: ['executions', tenant],
    queryFn: () => apiClient.getExecutions({ limit: 50 }),
    refetchInterval: 5000, // Auto-refresh every 5 seconds
  })

  if (isLoading && !executions) {
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
          <Button onClick={() => refetch()} className="mt-4">
            Try Again
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div>
      <div className="mb-8 flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold text-foreground mb-2">
            Executions - {tenant}
          </h1>
          <p className="text-muted-foreground">
            Monitor workflow executions for tenant {tenant}
          </p>
          {isLoading && (
            <p className="text-xs text-blue-600 mt-1">Refreshing...</p>
          )}
        </div>
        <div className="flex gap-2">
          <Button 
            variant="outline" 
            onClick={() => refetch()}
            disabled={isLoading}
          >
            <RefreshCw className={`h-4 w-4 mr-2 ${isLoading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
          <Button 
            variant="outline" 
            onClick={() => window.location.href = `/${tenant}/${locale}/workflows`}
          >
            View Workflows
          </Button>
        </div>
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
