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
      // Extract input schema from SDL definition
      return this.extractInputFromDefinition(workflowDefinition);
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
    this.logger.debug('Extracting input schema from SDL definition', { definition });

    // Check for explicit input schema in SDL definition (top-level)
    if (definition.input) {
      this.logger.debug('Found input schema at top level');
      return this.convertToJsonSchema(definition.input);
    }

    // Check for input in document section (SDL 1.0.0 format)
    if (definition.document?.input) {
      this.logger.debug('Found input schema in document section');
      return this.convertToJsonSchema(definition.document.input);
    }

    // Check for dataInputSchema in legacy format
    if (definition.dataInputSchema) {
      this.logger.debug('Found dataInputSchema in legacy format');
      return definition.dataInputSchema;
    }

    // Try to infer input schema from task variable references
    this.logger.debug('No explicit input schema found, inferring from tasks');
    return this.inferInputFromTasks(definition);
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
    this.logger.debug('Converting to JSON Schema', { input });

    if (typeof input === 'object' && input.type) {
      // Already a JSON schema
      this.logger.debug('Input is already in JSON Schema format');
      return input;
    }

    // Convert SDL input schema format to JSON schema
    const properties: Record<string, any> = {};
    const required: string[] = [];

    for (const [key, value] of Object.entries(input)) {
      if (typeof value === 'object' && value !== null) {
        // Handle SDL format: { fieldName: { type: "string", description: "..." } }
        const valueObj = value as any;
        if (valueObj.type) {
          properties[key] = {
            type: valueObj.type,
            description: valueObj.description || `${key} parameter`,
            format: valueObj.format,
            ...(valueObj.enum && { enum: valueObj.enum }),
            ...(valueObj.minimum !== undefined && { minimum: valueObj.minimum }),
            ...(valueObj.maximum !== undefined && { maximum: valueObj.maximum }),
          };

          // Remove undefined properties
          Object.keys(properties[key]).forEach(prop => {
            if (properties[key][prop] === undefined) {
              delete properties[key][prop];
            }
          });
        } else {
          // Nested object
          properties[key] = value;
        }
      } else {
        // Simple value, infer type
        properties[key] = {
          type: this.inferType(value),
          description: `Input parameter: ${key}`,
        };
      }
    }

    const schema = {
      type: 'object' as const,
      properties,
      required,
      additionalProperties: true,
    };

    this.logger.debug('Generated JSON Schema', { schema });
    return schema;
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
