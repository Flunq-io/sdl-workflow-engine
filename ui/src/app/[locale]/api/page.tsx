'use client'

import { useState } from 'react'
import { useTranslations } from 'next-intl'
import { Header } from '@/components/header'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ExternalLink, Code, Book, Table, LayoutGrid } from 'lucide-react'

export default function APIDocumentationPage() {
  const t = useTranslations()
  const [viewMode, setViewMode] = useState<'cards' | 'table'>('cards')

  const openSwaggerDocs = () => {
    window.open('http://localhost:8080/docs', '_blank')
  }

  const endpoints = [
    {
      method: 'GET',
      path: '/workflows',
      description: t('api.endpoints.listWorkflows'),
      parameters: 'limit, offset, status',
      response: 'WorkflowListResponse'
    },
    {
      method: 'GET',
      path: '/workflows/{id}',
      description: t('api.endpoints.getWorkflow'),
      parameters: 'id (path)',
      response: 'Workflow'
    },
    {
      method: 'POST',
      path: '/workflows',
      description: t('api.endpoints.createWorkflow'),
      parameters: 'WorkflowRequest (body)',
      response: 'Workflow'
    },
    {
      method: 'DELETE',
      path: '/workflows/{id}',
      description: t('api.endpoints.deleteWorkflow'),
      parameters: 'id (path)',
      response: 'Success'
    },
    {
      method: 'GET',
      path: '/workflows/{id}/events',
      description: t('api.endpoints.getEvents'),
      parameters: 'id (path), since',
      response: 'EventHistoryResponse'
    },
    {
      method: 'POST',
      path: '/workflows/{id}/execute',
      description: t('api.endpoints.executeWorkflow'),
      parameters: 'id (path), input (body)',
      response: 'ExecutionResponse'
    }
  ]

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-foreground mb-2">{t('api.title')}</h1>
          <p className="text-muted-foreground">
            {t('api.subtitle')}
          </p>
        </div>

        {/* Swagger Documentation Card */}
        <Card className="mb-8">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Book className="h-5 w-5" />
              {t('api.swagger.title')}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-muted-foreground">
              {t('api.swagger.description')}
            </p>
            <div className="flex flex-wrap gap-2">
              <Badge variant="secondary">OpenAPI 3.0</Badge>
              <Badge variant="secondary">REST API</Badge>
              <Badge variant="secondary">JSON</Badge>
              <Badge variant="secondary">CloudEvents</Badge>
            </div>
            <Button onClick={openSwaggerDocs} className="w-full">
              <ExternalLink className="h-4 w-4 mr-2" />
              {t('api.swagger.openDocs')}
            </Button>
          </CardContent>
        </Card>

        {/* API Endpoints */}
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle className="flex items-center gap-2">
                <Code className="h-5 w-5" />
                {t('api.endpoints.title')}
              </CardTitle>
              <div className="flex gap-2">
                <Button
                  variant={viewMode === 'cards' ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => setViewMode('cards')}
                >
                  <LayoutGrid className="h-4 w-4 mr-1" />
                  {t('api.view.cards')}
                </Button>
                <Button
                  variant={viewMode === 'table' ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => setViewMode('table')}
                >
                  <Table className="h-4 w-4 mr-1" />
                  {t('api.view.table')}
                </Button>
              </div>
            </div>
          </CardHeader>
          <CardContent>
            {viewMode === 'cards' ? (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {endpoints.map((endpoint, index) => (
                  <div key={index} className="border rounded-lg p-4 space-y-2">
                    <div className="flex items-center justify-between">
                      <code className="font-medium">{endpoint.method} {endpoint.path}</code>
                      <Badge variant={endpoint.method === 'GET' ? 'secondary' : endpoint.method === 'POST' ? 'default' : 'destructive'}>
                        {endpoint.method}
                      </Badge>
                    </div>
                    <p className="text-sm text-muted-foreground">{endpoint.description}</p>
                    <div className="text-xs text-muted-foreground">
                      <div><strong>Parameters:</strong> {endpoint.parameters}</div>
                      <div><strong>Response:</strong> {endpoint.response}</div>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full border-collapse">
                  <thead>
                    <tr className="border-b">
                      <th className="text-left p-2 font-medium">{t('api.table.method')}</th>
                      <th className="text-left p-2 font-medium">{t('api.table.endpoint')}</th>
                      <th className="text-left p-2 font-medium">{t('api.table.description')}</th>
                      <th className="text-left p-2 font-medium">{t('api.table.parameters')}</th>
                      <th className="text-left p-2 font-medium">{t('api.table.response')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {endpoints.map((endpoint, index) => (
                      <tr key={index} className="border-b hover:bg-muted/50">
                        <td className="p-2">
                          <Badge variant={endpoint.method === 'GET' ? 'secondary' : endpoint.method === 'POST' ? 'default' : 'destructive'}>
                            {endpoint.method}
                          </Badge>
                        </td>
                        <td className="p-2">
                          <code className="text-sm">{endpoint.path}</code>
                        </td>
                        <td className="p-2 text-sm">{endpoint.description}</td>
                        <td className="p-2 text-sm text-muted-foreground">{endpoint.parameters}</td>
                        <td className="p-2 text-sm text-muted-foreground">{endpoint.response}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>
      </main>
    </div>
  )
}
