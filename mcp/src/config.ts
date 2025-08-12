import { config } from 'dotenv';

// Load environment variables
config();

export class Config {
  // Flunq.io API configuration
  public readonly flunqApiUrl: string;
  public readonly defaultTenant: string;
  public readonly requestTimeout: number;

  // Execution configuration
  public readonly executionTimeout: number; // seconds
  public readonly pollInterval: number; // seconds

  // Cache configuration
  public readonly cacheEnabled: boolean;
  public readonly cacheTtl: number; // seconds

  // Logging configuration
  public readonly logLevel: string;
  public readonly logFormat: string;

  constructor() {
    // Flunq.io API configuration
    this.flunqApiUrl = process.env.FLUNQ_API_URL || 'http://localhost:8080';
    this.defaultTenant = process.env.FLUNQ_DEFAULT_TENANT || 'default';
    this.requestTimeout = parseInt(process.env.FLUNQ_REQUEST_TIMEOUT || '30000');

    // Execution configuration
    this.executionTimeout = parseInt(process.env.FLUNQ_EXECUTION_TIMEOUT || '300'); // 5 minutes
    this.pollInterval = parseInt(process.env.FLUNQ_POLL_INTERVAL || '2'); // 2 seconds

    // Cache configuration
    this.cacheEnabled = process.env.FLUNQ_CACHE_ENABLED !== 'false';
    this.cacheTtl = parseInt(process.env.FLUNQ_CACHE_TTL || '300'); // 5 minutes

    // Logging configuration
    this.logLevel = process.env.LOG_LEVEL || 'info';
    this.logFormat = process.env.LOG_FORMAT || 'json';

    this.validateConfig();
  }

  private validateConfig(): void {
    const errors: string[] = [];

    if (!this.flunqApiUrl) {
      errors.push('FLUNQ_API_URL is required');
    }

    if (!this.defaultTenant) {
      errors.push('FLUNQ_DEFAULT_TENANT is required');
    }

    if (this.requestTimeout <= 0) {
      errors.push('FLUNQ_REQUEST_TIMEOUT must be positive');
    }

    if (this.executionTimeout <= 0) {
      errors.push('FLUNQ_EXECUTION_TIMEOUT must be positive');
    }

    if (this.pollInterval <= 0) {
      errors.push('FLUNQ_POLL_INTERVAL must be positive');
    }

    if (this.cacheTtl <= 0) {
      errors.push('FLUNQ_CACHE_TTL must be positive');
    }

    if (errors.length > 0) {
      throw new Error(`Configuration validation failed:\n${errors.join('\n')}`);
    }
  }

  public toObject(): Record<string, any> {
    return {
      flunqApiUrl: this.flunqApiUrl,
      defaultTenant: this.defaultTenant,
      requestTimeout: this.requestTimeout,
      executionTimeout: this.executionTimeout,
      pollInterval: this.pollInterval,
      cacheEnabled: this.cacheEnabled,
      cacheTtl: this.cacheTtl,
      logLevel: this.logLevel,
      logFormat: this.logFormat,
    };
  }
}
