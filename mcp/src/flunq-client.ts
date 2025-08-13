import axios, { AxiosInstance } from 'axios';
import { Config } from './config.js';
import { Logger } from './logger.js';

export interface Workflow {
  id: string;
  name: string;
  description: string;
  tenant_id: string;
  definition: any;
  state: string;
  tags: string[];
  created_at: string;
  updated_at: string;
}

export interface PaginationMeta {
  total: number;
  limit: number;
  offset: number;
  page: number;
  size: number;
  total_pages: number;
  has_next: boolean;
  has_previous: boolean;
}

export interface FilterMeta {
  applied: { [key: string]: any };
  count: number;
}

export interface SortMeta {
  field: string;
  order: string;
}

export interface WorkflowListResponse {
  items: Workflow[];
  pagination: PaginationMeta;
  filters: FilterMeta;
  sort?: SortMeta;
}

export interface WorkflowListParams {
  page?: number;
  size?: number;
  sort_by?: string;
  sort_order?: 'asc' | 'desc';
  status?: string;
  name?: string;
  description?: string;
  tags?: string;
  search?: string;
  created_at?: string;
  updated_at?: string;
}

export interface Execution {
  id: string;
  workflow_id: string;
  tenant_id: string;
  status: string;
  correlation_id?: string;
  input?: any;
  output?: any;
  error?: any;
  started_at: string;
  completed_at?: string;
}

export interface ExecuteWorkflowRequest {
  tenant_id: string;
  input?: any;
  correlation_id?: string;
}

export class FlunqApiClient {
  private client: AxiosInstance;
  private config: Config;
  private logger: Logger;

  constructor(config: Config, logger: Logger) {
    this.config = config;
    this.logger = logger;
    
    this.client = axios.create({
      baseURL: config.flunqApiUrl,
      timeout: config.requestTimeout,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Add request/response interceptors for logging
    this.client.interceptors.request.use(
      (config) => {
        this.logger.debug('API Request:', {
          method: config.method,
          url: config.url,
          data: config.data,
        });
        return config;
      },
      (error) => {
        this.logger.error('API Request Error:', error);
        return Promise.reject(error);
      }
    );

    this.client.interceptors.response.use(
      (response) => {
        this.logger.debug('API Response:', {
          status: response.status,
          data: response.data,
        });
        return response;
      },
      (error) => {
        this.logger.error('API Response Error:', {
          status: error.response?.status,
          data: error.response?.data,
          message: error.message,
        });
        return Promise.reject(error);
      }
    );
  }

  async listWorkflows(tenantId?: string, params?: WorkflowListParams): Promise<Workflow[]> {
    const tenant = tenantId || this.config.defaultTenant;

    // Build query parameters
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.set('page', params.page.toString());
      if (params.size) queryParams.set('size', params.size.toString());
      if (params.sort_by) queryParams.set('sort_by', params.sort_by);
      if (params.sort_order) queryParams.set('sort_order', params.sort_order);
      if (params.status) queryParams.set('status', params.status);
      if (params.name) queryParams.set('name', params.name);
      if (params.description) queryParams.set('description', params.description);
      if (params.tags) queryParams.set('tags', params.tags);
      if (params.search) queryParams.set('search', params.search);
      if (params.created_at) queryParams.set('created_at', params.created_at);
      if (params.updated_at) queryParams.set('updated_at', params.updated_at);
    }

    const url = `/api/v1/${tenant}/workflows${queryParams.toString() ? `?${queryParams.toString()}` : ''}`;
    const response = await this.client.get(url);

    // Handle both old and new API response formats
    if (response.data.items) {
      return response.data.items;
    }

    // Fallback for legacy format
    return response.data || [];
  }

  async listWorkflowsWithPagination(tenantId?: string, params?: WorkflowListParams): Promise<WorkflowListResponse> {
    const tenant = tenantId || this.config.defaultTenant;

    // Build query parameters
    const queryParams = new URLSearchParams();
    if (params) {
      if (params.page) queryParams.set('page', params.page.toString());
      if (params.size) queryParams.set('size', params.size.toString());
      if (params.sort_by) queryParams.set('sort_by', params.sort_by);
      if (params.sort_order) queryParams.set('sort_order', params.sort_order);
      if (params.status) queryParams.set('status', params.status);
      if (params.name) queryParams.set('name', params.name);
      if (params.description) queryParams.set('description', params.description);
      if (params.tags) queryParams.set('tags', params.tags);
      if (params.search) queryParams.set('search', params.search);
      if (params.created_at) queryParams.set('created_at', params.created_at);
      if (params.updated_at) queryParams.set('updated_at', params.updated_at);
    }

    const url = `/api/v1/${tenant}/workflows${queryParams.toString() ? `?${queryParams.toString()}` : ''}`;
    const response = await this.client.get(url);

    return response.data;
  }

  async getWorkflow(workflowId: string, tenantId?: string): Promise<Workflow> {
    const tenant = tenantId || this.config.defaultTenant;
    const response = await this.client.get(`/api/v1/${tenant}/workflows/${workflowId}`);
    return response.data;
  }

  async executeWorkflow(
    workflowId: string,
    request: ExecuteWorkflowRequest
  ): Promise<Execution> {
    const tenant = request.tenant_id || this.config.defaultTenant;
    const response = await this.client.post(
      `/api/v1/${tenant}/workflows/${workflowId}/execute`,
      {
        tenant_id: tenant,
        input: request.input,
        correlation_id: request.correlation_id,
      }
    );
    return response.data;
  }

  async getExecution(executionId: string, tenantId?: string): Promise<Execution> {
    const tenant = tenantId || this.config.defaultTenant;
    const response = await this.client.get(`/api/v1/${tenant}/executions/${executionId}`);
    return response.data;
  }

  async getExecutionEvents(executionId: string, tenantId?: string): Promise<any[]> {
    const tenant = tenantId || this.config.defaultTenant;
    const response = await this.client.get(`/api/v1/${tenant}/executions/${executionId}/events`);
    return response.data.events || [];
  }

  async healthCheck(): Promise<boolean> {
    try {
      const response = await this.client.get('/health');
      return response.status === 200;
    } catch (error) {
      this.logger.error('Health check failed:', error);
      return false;
    }
  }
}
