import { FlunqApiClient, Execution } from './flunq-client.js';
import { Logger } from './logger.js';
import { Config } from './config.js';

export interface ExecutionResult {
  success: boolean;
  executionId: string;
  output?: any;
  error?: string;
  duration?: number;
  status: string;
}

export class ExecutionManager {
  private flunqClient: FlunqApiClient;
  private logger: Logger;
  private config: Config;

  constructor(flunqClient: FlunqApiClient, logger: Logger, config: Config) {
    this.flunqClient = flunqClient;
    this.logger = logger;
    this.config = config;
  }

  async executeWorkflow(
    workflowId: string,
    input: any,
    tenantId?: string
  ): Promise<ExecutionResult> {
    const tenant = tenantId || this.config.defaultTenant;
    
    this.logger.info(`Starting workflow execution: ${workflowId}`, {
      tenantId: tenant,
      input,
    });

    try {
      // Start workflow execution
      const execution = await this.flunqClient.executeWorkflow(workflowId, {
        tenant_id: tenant,
        input,
        correlation_id: this.generateCorrelationId(),
      });

      this.logger.info(`Workflow execution started: ${execution.id}`);

      // Monitor execution until completion
      const result = await this.monitorExecution(execution.id, tenant);

      this.logger.info(`Workflow execution completed: ${execution.id}`, {
        success: result.success,
        status: result.status,
      });

      return result;
    } catch (error) {
      this.logger.error(`Workflow execution failed: ${workflowId}`, error);
      
      return {
        success: false,
        executionId: '',
        error: error instanceof Error ? error.message : 'Unknown error',
        status: 'failed',
      };
    }
  }

  private async monitorExecution(
    executionId: string,
    tenantId: string
  ): Promise<ExecutionResult> {
    const startTime = Date.now();
    const timeout = this.config.executionTimeout * 1000; // Convert to milliseconds
    const pollInterval = this.config.pollInterval * 1000; // Convert to milliseconds

    while (Date.now() - startTime < timeout) {
      try {
        const execution = await this.flunqClient.getExecution(executionId, tenantId);
        
        this.logger.debug(`Execution status: ${execution.status}`, {
          executionId,
          status: execution.status,
        });

        // Check if execution is complete
        if (this.isTerminalStatus(execution.status)) {
          return this.createExecutionResult(execution, startTime);
        }

        // Wait before next poll
        await this.sleep(pollInterval);
      } catch (error) {
        this.logger.error(`Failed to poll execution status: ${executionId}`, error);
        
        // Continue polling unless it's a critical error
        if (this.isCriticalError(error)) {
          return {
            success: false,
            executionId,
            error: `Monitoring failed: ${error instanceof Error ? error.message : 'Unknown error'}`,
            status: 'unknown',
          };
        }
      }
    }

    // Execution timed out
    this.logger.warn(`Execution monitoring timed out: ${executionId}`);
    
    return {
      success: false,
      executionId,
      error: `Execution monitoring timed out after ${this.config.executionTimeout} seconds`,
      status: 'timeout',
    };
  }

  private createExecutionResult(execution: Execution, startTime: number): ExecutionResult {
    const duration = Date.now() - startTime;
    const success = execution.status === 'completed';

    return {
      success,
      executionId: execution.id,
      output: execution.output,
      error: execution.error?.message || (success ? undefined : 'Execution failed'),
      duration,
      status: execution.status,
    };
  }

  private isTerminalStatus(status: string): boolean {
    const terminalStatuses = ['completed', 'failed', 'cancelled', 'error'];
    return terminalStatuses.includes(status.toLowerCase());
  }

  private isCriticalError(error: any): boolean {
    // Consider 404 (execution not found) as critical
    if (error.response?.status === 404) {
      return true;
    }

    // Consider authentication errors as critical
    if (error.response?.status === 401 || error.response?.status === 403) {
      return true;
    }

    return false;
  }

  private generateCorrelationId(): string {
    return `mcp-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
  }

  private sleep(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  async getExecutionStatus(executionId: string, tenantId?: string): Promise<Execution> {
    const tenant = tenantId || this.config.defaultTenant;
    return this.flunqClient.getExecution(executionId, tenant);
  }

  async getExecutionEvents(executionId: string, tenantId?: string): Promise<any[]> {
    const tenant = tenantId || this.config.defaultTenant;
    return this.flunqClient.getExecutionEvents(executionId, tenant);
  }
}
