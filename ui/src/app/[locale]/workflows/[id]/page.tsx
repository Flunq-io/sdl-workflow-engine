'use client'

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams } from 'next/navigation'
import { useTranslations, useLocale } from 'next-intl'
import Link from 'next/link'
import { apiClient } from '@/lib/api'
import { Header } from '@/components/header'
import { WorkflowDetail } from '@/components/workflow-detail'
import { EventTimeline } from '@/components/event-timeline'
import { Button } from '@/components/ui/button'
import { ArrowLeft, Loader2, ChevronLeft, ChevronRight } from 'lucide-react'

export default function WorkflowDetailPage() {
  const params = useParams()
  const workflowId = params.id as string
  const t = useTranslations()
  const locale = useLocale()
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)

  const { data: workflow, isLoading: workflowLoading, error: workflowError } = useQuery({
    queryKey: ['workflow', workflowId],
    queryFn: () => apiClient.getWorkflow(workflowId),
    enabled: !!workflowId,
  })

  const { data: events, isLoading: eventsLoading, error: eventsError } = useQuery({
    queryKey: ['workflow-events', workflowId],
    queryFn: () => apiClient.getWorkflowEvents(workflowId, { limit: 100 }),
    enabled: !!workflowId,
    refetchInterval: 5000, // Refresh every 5 seconds for real-time updates
  })



  if (workflowLoading) {
    return (
      <div className="min-h-screen bg-background">
        <Header />
        <div className="flex items-center justify-center h-96">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      </div>
    )
  }

  if (workflowError || !workflow) {
    return (
      <div className="min-h-screen bg-background">
        <Header />
        <div className="container mx-auto px-4 py-8">
          <div className="flex items-center justify-center h-96">
            <div className="text-center">
              <h2 className="text-lg font-semibold text-foreground mb-2">Workflow not found</h2>
              <p className="text-muted-foreground mb-4">
                {workflowError?.message || 'The requested workflow could not be found.'}
              </p>
              <Button asChild>
                <Link href="/">
                  <ArrowLeft className="h-4 w-4 mr-2" />
                  Back to Workflows
                </Link>
              </Button>
            </div>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="container mx-auto px-4 py-8">
        <div className="mb-6">
          <Button variant="ghost" asChild className="mb-4">
            <Link href={`/${locale}`}>
              <ArrowLeft className="h-4 w-4 mr-2" />
              {t('workflowDetail.backToWorkflows')}
            </Link>
          </Button>

          <div className="mb-8">
            <h1 className="text-3xl font-bold text-foreground mb-2">{workflow.name}</h1>
            <p className="text-muted-foreground">
              {workflow.description || t('workflowDetail.noDescription')}
            </p>
          </div>
        </div>

        <div className="flex gap-6">
          {/* Collapsible Sidebar */}
          <div className={`transition-all duration-300 ${sidebarCollapsed ? 'w-16' : 'w-80'} flex-shrink-0`}>
            <div className="sticky top-4">
              {/* Sidebar Content */}
              {!sidebarCollapsed && (
                <div className="space-y-4">
                  {/* Collapse Toggle */}
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setSidebarCollapsed(!sidebarCollapsed)}
                    className="w-full justify-start text-muted-foreground hover:text-foreground"
                  >
                    <ChevronLeft className="h-4 w-4 mr-2" />
                    {t('workflowDetail.collapse')}
                  </Button>

                  <WorkflowDetail workflow={workflow} events={events?.items || []} />
                </div>
              )}

              {/* Collapsed State - Clean Minimal Sidebar */}
              {sidebarCollapsed && (
                <div className="bg-card border rounded-lg p-3 space-y-6">
                  {/* Expand Toggle */}
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setSidebarCollapsed(!sidebarCollapsed)}
                    className="w-full justify-center p-2"
                    title={t('workflowDetail.expand')}
                  >
                    <ChevronRight className="h-4 w-4" />
                  </Button>

                  {/* Status Indicator */}
                  <div className="flex flex-col items-center space-y-3">
                    <div
                      className={`w-4 h-4 rounded-full ${
                        workflow.status === 'completed' ? 'bg-green-500' :
                        workflow.status === 'running' ? 'bg-blue-500' :
                        workflow.status === 'faulted' ? 'bg-red-500' :
                        workflow.status === 'cancelled' ? 'bg-gray-500' :
                        'bg-yellow-500'
                      }`}
                      title={t(`status.${workflow.status}`)}
                    />

                    {/* Task Count */}
                    <div className="text-center">
                      <div className="text-sm font-medium">
                        {workflow.definition?.do?.length || 0}
                      </div>
                      <div className="text-xs text-muted-foreground">
                        tasks
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Event Timeline - Takes remaining space */}
          <div className="flex-1 min-w-0">
            <EventTimeline
              events={events?.items || []}
              isLoading={eventsLoading}
              error={eventsError}
            />
          </div>
        </div>
      </main>
    </div>
  )
}
