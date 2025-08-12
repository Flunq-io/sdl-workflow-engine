import { Logger } from './logger.js';

export interface JsonSchema {
  type: string;
  properties?: Record<string, any>;
  required?: string[];
  description?: string;
  additionalProperties?: boolean;
}

export class SchemaGenerator {
  private logger: Logger;

  constructor(logger: Logger) {
    this.logger = logger;
  }

  generateInputSchema(workflowDefinition: any): JsonSchema {
    this.logger.debug('Generating input schema from workflow definition');

    try {
      // For now, return a simple default schema to avoid JSON schema validation issues
      // TODO: Implement proper schema extraction once we have real workflow definitions
      return this.getDefaultInputSchema();
    } catch (error) {
      this.logger.warn('Failed to generate input schema, using default:', error);
      return this.getDefaultInputSchema();
    }
  }

  generateOutputSchema(workflowDefinition: any): JsonSchema | undefined {
    this.logger.debug('Generating output schema from workflow definition');

    try {
      // Try to extract output schema from workflow definition
      return this.extractOutputFromDefinition(workflowDefinition);
    } catch (error) {
      this.logger.warn('Failed to generate output schema:', error);
      return undefined;
    }
  }

  validateInput(input: any, schema: JsonSchema): void {
    this.logger.debug('Validating input against schema');

    // Basic validation - in production, use a proper JSON schema validator like Ajv
    if (schema.required) {
      for (const field of schema.required) {
        if (!(field in input)) {
          throw new Error(`Required field missing: ${field}`);
        }
      }
    }

    if (schema.properties) {
      for (const [key, value] of Object.entries(input)) {
        if (!(key in schema.properties)) {
          this.logger.warn(`Unknown input field: ${key}`);
        }
      }
    }
  }

  private extractInputFromDefinition(definition: any): JsonSchema {
    // Check for explicit input schema in SDL definition
    if (definition.input) {
      return this.convertToJsonSchema(definition.input);
    }

    // Check for input in document section
    if (definition.document?.input) {
      return this.convertToJsonSchema(definition.document.input);
    }

    // Check for dataInputSchema in SDL 1.0.0 format
    if (definition.dataInputSchema) {
      return definition.dataInputSchema;
    }

    return this.getDefaultInputSchema();
  }

  private extractOutputFromDefinition(definition: any): JsonSchema | undefined {
    // Check for explicit output schema
    if (definition.output) {
      return this.convertToJsonSchema(definition.output);
    }

    if (definition.document?.output) {
      return this.convertToJsonSchema(definition.document.output);
    }

    if (definition.dataOutputSchema) {
      return definition.dataOutputSchema;
    }

    return undefined;
  }

  private inferInputFromTasks(definition: any): JsonSchema {
    const properties: Record<string, any> = {};
    const required: string[] = [];

    // Analyze tasks to find variable references
    const tasks = definition.do || definition.states || [];
    
    for (const task of tasks) {
      this.extractVariablesFromTask(task, properties, required);
    }

    return {
      type: 'object',
      properties,
      required,
      additionalProperties: true,
    };
  }

  private extractVariablesFromTask(task: any, properties: Record<string, any>, required: string[]): void {
    // Look for variable references like ${.variableName} or ${ .variableName }
    const taskStr = JSON.stringify(task);
    const variableRegex = /\$\{\s*\.(\w+)\s*\}/g;
    let match;

    while ((match = variableRegex.exec(taskStr)) !== null) {
      const variableName = match[1];
      
      if (!properties[variableName]) {
        properties[variableName] = {
          type: 'string',
          description: `Input parameter: ${variableName}`,
        };
      }
    }
  }

  private convertToJsonSchema(input: any): JsonSchema {
    if (typeof input === 'object' && input.type) {
      // Already a JSON schema
      return input;
    }

    // Convert simple object to JSON schema
    const properties: Record<string, any> = {};
    const required: string[] = [];

    for (const [key, value] of Object.entries(input)) {
      if (typeof value === 'object' && value !== null) {
        properties[key] = value;
      } else {
        properties[key] = {
          type: this.inferType(value),
          description: `Input parameter: ${key}`,
        };
      }
    }

    return {
      type: 'object',
      properties,
      required,
      additionalProperties: true,
    };
  }

  private inferType(value: any): string {
    if (typeof value === 'string') return 'string';
    if (typeof value === 'number') return 'number';
    if (typeof value === 'boolean') return 'boolean';
    if (Array.isArray(value)) return 'array';
    if (typeof value === 'object') return 'object';
    return 'string';
  }

  private getDefaultInputSchema(): JsonSchema {
    return {
      type: 'object',
      properties: {
        input: {
          type: 'object',
          description: 'Workflow input data',
          additionalProperties: true
        }
      },
      required: [],
      additionalProperties: false,
      description: 'Workflow input parameters',
    };
  }
}
