'use client'

import { useQuery } from '@tanstack/react-query'
import { createTenantApiClient } from '@/lib/api'
import { WorkflowList } from '@/components/workflow-list'
import { ClientOnly } from '@/components/client-only'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface PageProps {
  params: {
    tenant: string;
    locale: string;
  };
}

export default function WorkflowsPage({ params }: PageProps) {
  const { tenant, locale } = params;
  
  // Create tenant-aware API client
  const apiClient = createTenantApiClient(tenant);
  
  const { data: workflows, isLoading, error } = useQuery({
    queryKey: ['workflows', tenant],
    queryFn: () => apiClient.getWorkflows({ limit: 50 }),
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
          <h2 className="text-lg font-semibold text-foreground mb-2">Failed to load workflows</h2>
          <p className="text-muted-foreground">{error.message}</p>
        </div>
      </div>
    )
  }

  return (
    <div>
      <div className="mb-8 flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold text-foreground mb-2">
            Workflows - {tenant}
          </h1>
          <p className="text-muted-foreground">
            Manage and execute workflows for tenant {tenant}
          </p>
        </div>
        <div className="flex gap-2">
          <Button 
            variant="outline" 
            onClick={() => window.location.href = `/${tenant}/${locale}/executions`}
          >
            View Executions
          </Button>
        </div>
      </div>

      <ClientOnly fallback={
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      }>
        <WorkflowList workflows={workflows?.items || []} tenant={tenant} locale={locale} />
      </ClientOnly>
    </div>
  )
}
