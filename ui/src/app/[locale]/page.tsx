'use client'

import { useQuery } from '@tanstack/react-query'
import { useTranslations } from 'next-intl'
import { apiClient } from '@/lib/api'
import { ExecutionList } from '@/components/execution-list'
import { Header } from '@/components/header'
import { ClientOnly } from '@/components/client-only'
import { Loader2 } from 'lucide-react'

export default function HomePage() {
  const t = useTranslations()
  const { data: executions, isLoading, error } = useQuery({
    queryKey: ['executions'],
    queryFn: () => apiClient.getAllExecutions({ limit: 50 }),
  })

  if (isLoading) {
    return (
      <div className="min-h-screen bg-background">
        <Header />
        <div className="flex items-center justify-center h-96">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="min-h-screen bg-background">
        <Header />
        <div className="flex items-center justify-center h-96">
          <div className="text-center">
            <h2 className="text-lg font-semibold text-foreground mb-2">Failed to load executions</h2>
            <p className="text-muted-foreground">{error.message}</p>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-foreground mb-2">{t('executions.title')}</h1>
          <p className="text-muted-foreground">
            {t('executions.subtitle')}
          </p>
        </div>

        <ClientOnly fallback={
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        }>
          <ExecutionList executions={executions?.items || []} />
        </ClientOnly>
      </main>
    </div>
  )
}
