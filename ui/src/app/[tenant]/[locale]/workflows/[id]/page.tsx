'use client'

import React from 'react'
import {useQuery} from '@tanstack/react-query'
import {createTenantApiClient} from '@/lib/api'
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card'
import {Button} from '@/components/ui/button'
import {Loader2, ArrowLeft, Copy, Check} from 'lucide-react'
import Link from 'next/link'
import {useParams} from 'next/navigation'

function CopyButton({ json }: { json: string }) {
  const [copied, setCopied] = React.useState(false)
  return (
    <Button
      variant="outline"
      size="sm"
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(json)
          setCopied(true)
          setTimeout(() => setCopied(false), 2000)
        } catch (e) {
          console.warn('Failed to copy JSON to clipboard', e)
        }
      }}
      className="h-8 px-2"
    >
      {copied ? (
        <><Check className="h-4 w-4 mr-2" />Copied</>
      ) : (
        <><Copy className="h-4 w-4 mr-2" />Copy</>
      )}
    </Button>
  )
}

export default function WorkflowDetailPage() {
  const params = useParams<{tenant: string; locale: string; id: string}>()
  const tenant = String(params.tenant)
  const locale = String(params.locale)
  const id = String(params.id)

  const api = createTenantApiClient(tenant)

  const {data: workflow, isLoading, error, refetch} = useQuery({
    queryKey: ['workflow', tenant, id],
    queryFn: () => api.getWorkflow(id)
  })

  if (isLoading && !workflow) {
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
          <h2 className="text-lg font-semibold text-foreground mb-2">Failed to load workflow</h2>
          <p className="text-muted-foreground">{(error as Error).message}</p>
          <Button onClick={() => refetch()} className="mt-4">Refresh</Button>
        </div>
      </div>
    )
  }

  if (!workflow) return null

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Button asChild variant="outline" size="sm">
          <Link href={`/${tenant}/${locale}/workflows`}>
            <ArrowLeft className="h-4 w-4 mr-2" /> Back
          </Link>
        </Button>
        <h1 className="text-2xl font-bold text-foreground">{workflow.name}</h1>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-lg">Workflow DSL (JSON)</CardTitle>
          <CopyButton json={JSON.stringify(workflow.definition ?? {}, null, 2)} />
        </CardHeader>
        <CardContent>
          {workflow.description && (
            <p className="text-sm text-muted-foreground mb-3">{workflow.description}</p>
          )}
          <div className="rounded border bg-muted p-3">
            <pre className="text-xs overflow-x-auto whitespace-pre-wrap">
{JSON.stringify(workflow.definition ?? {}, null, 2)}
            </pre>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

