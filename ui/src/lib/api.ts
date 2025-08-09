const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

console.log('🌍 Environment NEXT_PUBLIC_API_URL:', process.env.NEXT_PUBLIC_API_URL)
console.log('🎯 Final API_BASE_URL:', API_BASE_URL)

export interface Workflow {
  id: string
  name: string
  description: string
  definition: Record<string, unknown>
  state: 'active' | 'inactive'
  tags: string[]
  created_at: string
  updated_at: string
  execution_count?: number
  last_execution?: string
}

export interface Execution {
  id: string
  workflow_id: string
  status: 'pending' | 'running' | 'waiting' | 'suspended' | 'cancelled' | 'faulted' | 'completed'
  correlation_id?: string
  input?: Record<string, unknown>
  output?: Record<string, unknown>
  error?: {
    message: string
    code: string
    details?: Record<string, unknown>
  }
  current_state?: string
  started_at: string
  completed_at?: string
  duration_ms?: number
}

export interface WorkflowEvent {
  id: string
  type: string
  source: string
  specversion: string
  time: string
  workflowid?: string
  executionid?: string
  taskid?: string
  data?: Record<string, unknown>
}

export interface ApiResponse<T> {
  data?: T
  error?: {
    code: string
    message: string
    details?: Record<string, unknown>
  }
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
  limit: number
  offset: number
}

class ApiClient {
  private baseUrl: string
  private tenantId?: string

  constructor(baseUrl: string = API_BASE_URL, tenantId?: string) {
    this.baseUrl = baseUrl
    this.tenantId = tenantId
    console.log('🔧 API Client initialized with baseUrl:', this.baseUrl, 'tenantId:', tenantId)
  }

  private async request<T>(endpoint: string, options?: RequestInit): Promise<T> {
    const method = (options?.method || 'GET').toUpperCase()

    // Primary: non-tenant path (matches handlers: GET /api/v1/workflows...)
    const apiPath = `/api/v1${endpoint}`
    let url = `${this.baseUrl}${apiPath}`

    console.log('🔍 API Request:', { url, method })

    let response = await fetch(url, {
      headers: {
        'Content-Type': 'application/json',
        ...(this.tenantId ? { 'X-Tenant-ID': this.tenantId } : {}),
        ...options?.headers,
      },
      ...options,
    })

    console.log('📡 API Response:', response.status, response.statusText)

    // Fallback: if 404 and tenantId is set, retry with tenant-prefixed path
    if (response.status === 404 && this.tenantId && method === 'GET' && !endpoint.startsWith(`/${this.tenantId}/`)) {
      const tenantPath = `/api/v1/${this.tenantId}${endpoint}`
      const tenantUrl = `${this.baseUrl}${tenantPath}`
      console.warn('🔁 Retrying with tenant-prefixed path', { tenantUrl })
      response = await fetch(tenantUrl, {
        headers: {
          'Content-Type': 'application/json',
          ...(this.tenantId ? { 'X-Tenant-ID': this.tenantId } : {}),
          ...options?.headers,
        },
        ...options,
      })
      url = tenantUrl
    }

    if (!response.ok) {
      let errorBody: any = undefined
      try {
        // Try JSON first, else fall back to text
        errorBody = await response.clone().json()
      } catch {
        try {
          errorBody = { message: await response.text() }
        } catch {
          errorBody = { message: 'Unknown error' }
        }
      }
      console.warn('API request failed', {
        url,
        status: response.status,
        statusText: response.statusText,
        error: errorBody
      })
      throw new Error(errorBody?.message || `HTTP ${response.status}`)
    }

    return response.json()
  }

  // Workflow endpoints
  async getWorkflows(params?: { limit?: number; offset?: number }): Promise<PaginatedResponse<Workflow>> {
    const searchParams = new URLSearchParams()
    if (params?.limit) searchParams.set('limit', params.limit.toString())
    if (params?.offset) searchParams.set('offset', params.offset.toString())
    
    const query = searchParams.toString()
    return this.request(`/workflows${query ? `?${query}` : ''}`)
  }

  async getWorkflow(id: string): Promise<Workflow> {
    return this.request(`/workflows/${id}`)
  }

  async executeWorkflow(id: string, input: Record<string, unknown>, correlationId?: string): Promise<Execution> {
    if (!this.tenantId) {
      throw new Error('tenant_id is required to execute a workflow')
    }
    return this.request(`/${this.tenantId}/workflows/${id}/execute`, {
      method: 'POST',
      body: JSON.stringify({
        input,
        correlation_id: correlationId,
      }),
    })
  }

  async getWorkflowEvents(id: string, params?: { limit?: number; offset?: number }): Promise<PaginatedResponse<WorkflowEvent>> {
    const searchParams = new URLSearchParams()
    if (params?.limit) searchParams.set('limit', params.limit.toString())
    if (params?.offset) searchParams.set('offset', params.offset.toString())
    
    const query = searchParams.toString()
    return this.request(`/workflows/${id}/events${query ? `?${query}` : ''}`)
  }

  // Execution endpoints
  async getExecutions(params?: { limit?: number; offset?: number; workflow_id?: string }): Promise<PaginatedResponse<Execution>> {
    const searchParams = new URLSearchParams()
    if (params?.limit) searchParams.set('limit', params.limit.toString())
    if (params?.offset) searchParams.set('offset', params.offset.toString())
    if (params?.workflow_id) searchParams.set('workflow_id', params.workflow_id)
    
    const query = searchParams.toString()
    return this.request(`/executions${query ? `?${query}` : ''}`)
  }

  async getExecution(id: string): Promise<Execution> {
    return this.request(`/executions/${id}`)
  }

  // Execution events endpoints (using API service with event sourcing)
  async getExecutionEvents(executionId: string): Promise<{ events: WorkflowEvent[]; count: number }> {
    return this.request(`/executions/${executionId}/events`)
  }

  // Get all executions for a workflow (using API service with event sourcing)
  async getWorkflowExecutions(workflowId: string): Promise<{ execution_ids: string[]; count: number }> {
    return this.request(`/workflows/${workflowId}/executions`)
  }

  // Get all executions across all workflows (combined from multiple sources)
  async getAllExecutions(params?: { limit?: number; offset?: number }): Promise<PaginatedResponse<Execution & { workflow_name?: string }>> {
    // First get all workflows to get execution IDs
    const workflows = await this.getWorkflows({ limit: 100 })

    const allExecutions: (Execution & { workflow_name?: string })[] = []

    // For each workflow, get its executions
    for (const workflow of workflows.items) {
      try {
        const workflowExecutions = await this.getWorkflowExecutions(workflow.id)

        // For each execution ID, try to get execution details from API service
        for (const executionId of workflowExecutions.execution_ids) {
          try {
            const execution = await this.getExecution(executionId)
            allExecutions.push({
              ...execution,
              workflow_name: workflow.name
            })
          } catch {
            // If execution not found in API service, create a minimal execution object
            allExecutions.push({
              id: executionId,
              workflow_id: workflow.id,
              workflow_name: workflow.name,
              status: 'pending' as const,
              started_at: new Date().toISOString(),
            })
          }
        }
      } catch (err) {
        console.warn(`Failed to get executions for workflow ${workflow.id}:`, err)
      }
    }

    // Sort by started_at descending (newest first)
    allExecutions.sort((a, b) => new Date(b.started_at).getTime() - new Date(a.started_at).getTime())

    // Apply pagination
    const offset = params?.offset || 0
    const limit = params?.limit || 50
    const paginatedExecutions = allExecutions.slice(offset, offset + limit)

    return {
      items: paginatedExecutions,
      total: allExecutions.length,
      limit,
      offset
    }
  }
}

export const apiClient = new ApiClient()

// Helper function to create tenant-aware API client
export function createTenantApiClient(tenantId: string): ApiClient {
  return new ApiClient(API_BASE_URL, tenantId)
}
