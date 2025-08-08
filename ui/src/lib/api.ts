const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

console.log('🌍 Environment NEXT_PUBLIC_API_URL:', process.env.NEXT_PUBLIC_API_URL)
console.log('🎯 Final API_BASE_URL:', API_BASE_URL)

export interface Workflow {
  id: string
  name: string
  description: string
  definition: Record<string, any>
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
  input?: Record<string, any>
  output?: Record<string, any>
  error?: {
    message: string
    code: string
    details?: Record<string, any>
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
  data?: Record<string, any>
}

export interface ApiResponse<T> {
  data?: T
  error?: {
    code: string
    message: string
    details?: Record<string, any>
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
    // Build URL with tenant context if available
    const apiPath = this.tenantId ? `/api/v1/${this.tenantId}${endpoint}` : `/api/v1${endpoint}`
    const url = `${this.baseUrl}${apiPath}`

    console.log('🔍 API Request:', url)

    const response = await fetch(url, {
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
      ...options,
    })

    console.log('📡 API Response:', response.status, response.statusText)

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Unknown error' }))
      console.error('❌ API Error:', error)
      throw new Error(error.message || `HTTP ${response.status}`)
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

  async executeWorkflow(id: string, input: any, correlationId?: string): Promise<Execution> {
    return this.request(`/workflows/${id}/execute`, {
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
          } catch (error) {
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
      } catch (error) {
        console.warn(`Failed to get executions for workflow ${workflow.id}:`, error)
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
