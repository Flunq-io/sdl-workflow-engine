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
    this.logger = new Logger();
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
        name: 'flunq-mcp-server',
        version: '0.1.0',
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
        this.logger.info('Discovering workflows...');

        // Use the working listWorkflowsWithPagination method directly
        const response = await this.discoveryService.listWorkflowsWithPagination();
        const workflows = response.items || [];

        const tools: Tool[] = workflows.map(workflow => ({
          name: `workflow_${workflow.id}`,
          description: workflow.description || `Execute ${workflow.name} workflow`,
          inputSchema: workflow.inputSchema || {
            type: 'object',
            properties: {},
            additionalProperties: true,
          },
        }));

        // Debug: Add a tool showing how many workflows were found
        tools.push({
          name: 'debug_workflow_count',
          description: `Found ${workflows.length} workflows for execution`,
          inputSchema: {
            type: 'object',
            properties: {},
            additionalProperties: false,
          },
        });

        // Add utility tools
        tools.push({
          name: 'refresh_workflows',
          description: 'Refresh the workflow cache to discover new workflows',
          inputSchema: {
            type: 'object',
            properties: {},
            additionalProperties: false,
          },
        });



        tools.push({
          name: 'list_workflows',
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

        // Handle refresh tool
        if (name === 'refresh_workflows') {
          this.discoveryService.clearCache();
          const workflows = await this.discoveryService.discoverWorkflows();
          return {
            content: [
              {
                type: 'text',
                text: `UNIQUE TEST 12345 - Found ${workflows.length} workflows at ${new Date().toISOString()}.`,
              },
            ],
          };
        }

        // Handle list workflows tool
        if (name === 'list_workflows') {
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

        // Handle test execution tool
        if (name === 'execute_purchase_requisition') {
          const result = await this.executionManager.executeWorkflow(
            'wf_28c0d892-aa31-47',
            args || {}
          );
          return {
            content: [
              {
                type: 'text',
                text: JSON.stringify(result, null, 2),
              },
            ],
          };
        }

        // Extract workflow ID from tool name
        const workflowId = name.replace(/^workflow_/, '');

        // Find the workflow by ID
        const workflows = await this.discoveryService.discoverWorkflows();
        const workflow = workflows.find(w => w.id === workflowId);

        if (!workflow) {
          throw new Error(`Workflow not found: ${workflowId}`);
        }

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
    this.logger.info('Flunq MCP Server started - DEBUG VERSION WITH LOGGING');
  }
}

// Start the server
const server = new FlunqMcpServer();
server.start().catch((error) => {
  console.error('Failed to start MCP server:', error);
  process.exit(1);
});
