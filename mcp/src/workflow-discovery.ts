import { FlunqApiClient, Workflow, WorkflowListParams, WorkflowListResponse } from './flunq-client.js';
import { SchemaGenerator } from './schema-generator.js';
import { Logger } from './logger.js';

export interface WorkflowTool {
  id: string;
  name: string;
  description: string;
  inputSchema: any;
  outputSchema?: any;
  metadata: {
    tenantId: string;
    version: string;
    tags: string[];
    createdAt: string;
  };
}

export class WorkflowDiscoveryService {
  private flunqClient: FlunqApiClient;
  private schemaGenerator: SchemaGenerator;
  private logger: Logger;
  private cache: Map<string, WorkflowTool[]> = new Map();
  private cacheExpiry: Map<string, number> = new Map();
  private readonly CACHE_TTL = 30 * 1000; // 30 seconds for development

  constructor(
    flunqClient: FlunqApiClient,
    schemaGenerator: SchemaGenerator,
    logger: Logger
  ) {
    this.flunqClient = flunqClient;
    this.schemaGenerator = schemaGenerator;
    this.logger = logger;
  }

  async discoverWorkflows(tenantId?: string): Promise<WorkflowTool[]> {
    const tenant = tenantId || 'default';
    const cacheKey = `workflows:${tenant}`;

    // Check cache first
    if (this.isCacheValid(cacheKey)) {
      this.logger.debug(`Returning cached workflows for tenant: ${tenant}`);
      return this.cache.get(cacheKey) || [];
    }

    try {
      this.logger.info(`Discovering workflows for tenant: ${tenant}`);
      
      // Fetch workflows from flunq.io API
      const workflows = await this.flunqClient.listWorkflows(tenant);
      
      // Convert workflows to MCP tools
      const tools: WorkflowTool[] = [];
      
      for (const workflow of workflows) {
        try {
          const tool = await this.convertWorkflowToTool(workflow);
          tools.push(tool);
        } catch (error) {
          this.logger.warn(`Failed to convert workflow ${workflow.id} to tool:`, error);
        }
      }

      // Cache the results
      this.cache.set(cacheKey, tools);
      this.cacheExpiry.set(cacheKey, Date.now() + this.CACHE_TTL);

      this.logger.info(`Discovered ${tools.length} workflow tools for tenant: ${tenant}`);
      return tools;
    } catch (error) {
      this.logger.error(`Failed to discover workflows for tenant ${tenant}:`, error);
      
      // Return cached results if available, even if expired
      return this.cache.get(cacheKey) || [];
    }
  }

  private async convertWorkflowToTool(workflow: Workflow): Promise<WorkflowTool> {
    this.logger.debug(`Converting workflow to tool: ${workflow.name}`);

    // Generate input schema from workflow definition
    const inputSchema = this.schemaGenerator.generateInputSchema(workflow.definition);
    
    // Generate output schema (optional)
    const outputSchema = this.schemaGenerator.generateOutputSchema(workflow.definition);

    // Extract description from workflow definition or use fallback
    const description = this.extractDescription(workflow);

    return {
      id: workflow.id,
      name: workflow.name,
      description,
      inputSchema,
      outputSchema,
      metadata: {
        tenantId: workflow.tenant_id,
        version: this.extractVersion(workflow.definition),
        tags: workflow.tags,
        createdAt: workflow.created_at,
      },
    };
  }

  private extractDescription(workflow: Workflow): string {
    // Try to extract description from various sources
    if (workflow.description) {
      return workflow.description;
    }

    if (workflow.definition?.document?.description) {
      return workflow.definition.document.description;
    }

    if (workflow.definition?.description) {
      return workflow.definition.description;
    }

    // Fallback description
    return `Execute ${workflow.name} workflow`;
  }

  private extractVersion(definition: any): string {
    return definition?.document?.version || 
           definition?.version || 
           '1.0.0';
  }

  private isCacheValid(cacheKey: string): boolean {
    const expiry = this.cacheExpiry.get(cacheKey);
    return expiry !== undefined && Date.now() < expiry;
  }

  clearCache(tenantId?: string): void {
    if (tenantId) {
      const cacheKey = `workflows:${tenantId}`;
      this.cache.delete(cacheKey);
      this.cacheExpiry.delete(cacheKey);
    } else {
      this.cache.clear();
      this.cacheExpiry.clear();
    }
    this.logger.info('Workflow cache cleared');
  }

  async refreshWorkflows(tenantId?: string): Promise<WorkflowTool[]> {
    this.clearCache(tenantId);
    return this.discoverWorkflows(tenantId);
  }

  async listWorkflowsWithPagination(params?: WorkflowListParams): Promise<WorkflowListResponse> {
    try {
      this.logger.info('Listing workflows with pagination and filtering', params);

      // Use the enhanced API client method
      const response = await this.flunqClient.listWorkflowsWithPagination(undefined, params);

      this.logger.info(`Found ${response.items.length} workflows (${response.pagination.total} total)`);
      return response;
    } catch (error) {
      this.logger.error('Failed to list workflows with pagination:', error);

      // Fallback to basic discovery without pagination
      const workflows = await this.discoverWorkflows();
      return {
        items: workflows.map(tool => ({
          id: tool.id,
          name: tool.name,
          description: tool.description,
          tenant_id: tool.metadata.tenantId,
          definition: {},
          state: 'active',
          tags: tool.metadata.tags,
          created_at: tool.metadata.createdAt,
          updated_at: tool.metadata.createdAt,
        })),
        pagination: {
          total: workflows.length,
          limit: workflows.length,
          offset: 0,
          page: 1,
          size: workflows.length,
          total_pages: 1,
          has_next: false,
          has_previous: false,
        },
        filters: {
          applied: {},
          count: workflows.length,
        },
      };
    }
  }
}
