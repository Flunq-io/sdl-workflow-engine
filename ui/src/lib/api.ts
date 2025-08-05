const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080/api/v1'

console.log('🌍 Environment NEXT_PUBLIC_API_URL:', process.env.NEXT_PUBLIC_API_URL)
console.log('🎯 Final API_BASE_URL:', API_BASE_URL)

export interface Workflow {
  id: string
  name: string
  description: string
  definition: Record<string, any>
  status: 'pending' | 'running' | 'waiting' | 'suspended' | 'cancelled' | 'faulted' | 'completed'
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

  constructor(baseUrl: string = API_BASE_URL) {
    this.baseUrl = baseUrl
    console.log('🔧 API Client initialized with baseUrl:', this.baseUrl)
  }

  private async request<T>(endpoint: string, options?: RequestInit): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`

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
}

export const apiClient = new ApiClient()
