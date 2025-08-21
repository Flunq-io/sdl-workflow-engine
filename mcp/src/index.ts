#!/usr/bin/env node

import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
  Tool,
} from '@modelcontextprotocol/sdk/types.js';
import { FlunqApiClient } from './flunq-client.js';
import { WorkflowDiscoveryService } from './workflow-discovery.js';
import { SchemaGenerator } from './schema-generator.js';
import { ExecutionManager } from './execution-manager.js';
import { Logger } from './logger.js';
import { Config } from './config.js';
import { SERVER_INFO, TOOL_NAMES, MESSAGES } from './constants.js';

class FlunqMcpServer {
  private server: Server;
  private flunqClient: FlunqApiClient;
  private discoveryService: WorkflowDiscoveryService;
  private schemaGenerator: SchemaGenerator;
  private executionManager: ExecutionManager;
  private logger: Logger;
  private config: Config;

  constructor() {
    this.config = new Config();
    this.logger = new Logger(this.config);
    this.flunqClient = new FlunqApiClient(this.config, this.logger);
    this.schemaGenerator = new SchemaGenerator(this.logger);
    this.discoveryService = new WorkflowDiscoveryService(
      this.flunqClient,
      this.schemaGenerator,
      this.logger,
      this.config
    );
    this.executionManager = new ExecutionManager(
      this.flunqClient,
      this.logger,
      this.config
    );

    this.server = new Server(
      {
        name: SERVER_INFO.NAME,
        version: SERVER_INFO.VERSION,
      },
      {
        capabilities: {
          tools: {},
        },
      }
    );

    this.setupHandlers();
  }

  private setupHandlers(): void {
    // List available workflow tools
    this.server.setRequestHandler(ListToolsRequestSchema, async () => {
      try {
        this.logger.info(MESSAGES.DISCOVERING_WORKFLOWS);

        // Use discoverWorkflows with caching for consistency
        const workflowTools = await this.discoveryService.discoverWorkflows();
        this.logger.info(`Discovered ${workflowTools.length} workflow tools for listing`);

        const tools: Tool[] = workflowTools.map(workflow => ({
          name: `${TOOL_NAMES.WORKFLOW_PREFIX}${workflow.id}`,
          description: workflow.description || `Execute ${workflow.name} workflow`,
          inputSchema: workflow.inputSchema || {
            type: 'object',
            properties: {},
            additionalProperties: true,
          },
        }));

        // Add server status tool
        tools.push({
          name: TOOL_NAMES.SERVER_STATUS,
          description: `Server status: ${workflowTools.length} workflows available`,
          inputSchema: {
            type: 'object',
            properties: {},
            additionalProperties: false,
          },
        });

        // Add utility tools
        tools.push({
          name: TOOL_NAMES.REFRESH_WORKFLOWS,
          description: 'Refresh the workflow cache to discover new workflows',
          inputSchema: {
            type: 'object',
            properties: {},
            additionalProperties: false,
          },
        });



        tools.push({
          name: TOOL_NAMES.LIST_WORKFLOWS,
          description: 'List workflows with filtering and pagination options',
          inputSchema: {
            type: 'object',
            properties: {
              page: {
                type: 'number',
                description: 'Page number (1-based)',
                minimum: 1,
              },
              size: {
                type: 'number',
                description: 'Number of items per page',
                minimum: 1,
                maximum: 100,
              },
              sort_by: {
                type: 'string',
                description: 'Field to sort by',
                enum: ['name', 'created_at', 'updated_at', 'state'],
              },
              sort_order: {
                type: 'string',
                description: 'Sort order',
                enum: ['asc', 'desc'],
              },
              status: {
                type: 'string',
                description: 'Filter by workflow status',
                enum: ['active', 'inactive', 'created'],
              },
              name: {
                type: 'string',
                description: 'Filter by workflow name (partial match)',
              },
              description: {
                type: 'string',
                description: 'Filter by workflow description (partial match)',
              },
              tags: {
                type: 'string',
                description: 'Filter by tags (comma-separated)',
              },
              search: {
                type: 'string',
                description: 'Search across workflow name, description, and tags',
              },
              created_at: {
                type: 'string',
                description: 'Filter by creation date range (format: "2024-01-01,2024-12-31")',
              },
              updated_at: {
                type: 'string',
                description: 'Filter by update date range (format: "2024-01-01,2024-12-31")',
              },
            },
            additionalProperties: false,
          },
        });

        this.logger.info(`Discovered ${tools.length} workflow tools`);
        return { tools };
      } catch (error) {
        this.logger.error('Failed to discover workflows:', error);
        return { tools: [] };
      }
    });

    // Execute workflow tool
    this.server.setRequestHandler(CallToolRequestSchema, async (request) => {
      const { name, arguments: args } = request.params;

      try {
        this.logger.info(`Executing tool: ${name}`, { args });
        this.logger.info(`Tool name received: "${name}" (length: ${name.length})`);

        // Handle server status tool
        if (name === TOOL_NAMES.SERVER_STATUS) {
          const workflows = await this.discoveryService.discoverWorkflows();
          const status = {
            server: SERVER_INFO.NAME,
            version: SERVER_INFO.VERSION,
            timestamp: new Date().toISOString(),
            config: this.config.toObject(),
            workflows: {
              count: workflows.length,
              tenant: this.config.defaultTenant,
            },
          };
          return {
            content: [
              {
                type: 'text',
                text: JSON.stringify(status, null, 2),
              },
            ],
          };
        }

        // Handle refresh tool
        if (name === TOOL_NAMES.REFRESH_WORKFLOWS) {
          const workflows = await this.discoveryService.forceRefreshWorkflows();
          return {
            content: [
              {
                type: 'text',
                text: `${MESSAGES.WORKFLOW_CACHE_REFRESHED}. Found ${workflows.length} workflows at ${new Date().toISOString()}.`,
              },
            ],
          };
        }

        // Handle list workflows tool
        if (name === TOOL_NAMES.LIST_WORKFLOWS) {
          const response = await this.discoveryService.listWorkflowsWithPagination(args);
          return {
            content: [
              {
                type: 'text',
                text: JSON.stringify(response, null, 2),
              },
            ],
          };
        }



        // Extract workflow ID from tool name
        const workflowId = name.replace(new RegExp(`^${TOOL_NAMES.WORKFLOW_PREFIX}`), '');
        this.logger.info(`Extracted workflow ID: "${workflowId}" from tool name: "${name}"`);

        // Use smart refresh to balance performance and freshness
        const workflows = await this.discoveryService.smartRefreshIfNeeded();

        this.logger.info(`Available workflows: ${workflows.map(w => w.id).join(', ')}`);

        const workflow = workflows.find(w => w.id === workflowId);

        if (!workflow) {
          this.logger.warn(`Workflow not found in cache: ${workflowId}. Attempting force refresh...`);

          // Try force refresh as fallback
          const freshWorkflows = await this.discoveryService.forceRefreshWorkflows();
          const freshWorkflow = freshWorkflows.find(w => w.id === workflowId);

          if (!freshWorkflow) {
            this.logger.error(`Workflow not found even after refresh: ${workflowId}. Available workflows: ${freshWorkflows.map(w => w.id).join(', ')}`);
            throw new Error(`Workflow not found: ${workflowId}. Available workflows: ${freshWorkflows.map(w => w.id).join(', ')}`);
          }

          this.logger.info(`Found workflow after refresh: ${freshWorkflow.name} (${freshWorkflow.id})`);

          // Validate input against schema
          if (freshWorkflow.inputSchema) {
            this.schemaGenerator.validateInput(args || {}, freshWorkflow.inputSchema);
          }

          // Execute workflow
          const result = await this.executionManager.executeWorkflow(
            freshWorkflow.id,
            args || {}
          );

          this.logger.info(`Tool execution completed: ${name}`, { result });

          return {
            content: [
              {
                type: 'text',
                text: JSON.stringify(result, null, 2),
              },
            ],
          };
        }

        this.logger.info(`Found workflow: ${workflow.name} (${workflow.id})`);

        // Validate input against schema
        if (workflow.inputSchema) {
          this.schemaGenerator.validateInput(args || {}, workflow.inputSchema);
        }

        // Execute workflow
        const result = await this.executionManager.executeWorkflow(
          workflow.id,
          args || {}
        );

        this.logger.info(`Tool execution completed: ${name}`, { result });

        return {
          content: [
            {
              type: 'text',
              text: JSON.stringify(result, null, 2),
            },
          ],
        };
      } catch (error) {
        this.logger.error(`Tool execution failed: ${name}`, error);
        
        return {
          content: [
            {
              type: 'text',
              text: `Error executing workflow: ${error instanceof Error ? error.message : 'Unknown error'}`,
            },
          ],
          isError: true,
        };
      }
    });
  }

  async start(): Promise<void> {
    const transport = new StdioServerTransport();
    await this.server.connect(transport);
    this.logger.info(MESSAGES.SERVER_STARTED);
  }
}

// Start the server
const server = new FlunqMcpServer();
server.start().catch((error) => {
  console.error('Failed to start MCP server:', error);
  process.exit(1);
});
