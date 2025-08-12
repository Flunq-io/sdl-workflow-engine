import winston from 'winston';

export class Logger {
  private logger: winston.Logger;

  constructor() {
    const logLevel = process.env.LOG_LEVEL || 'info';
    const logFormat = process.env.LOG_FORMAT || 'json';

    this.logger = winston.createLogger({
      level: logLevel,
      format: this.getFormat(logFormat),
      transports: [
        new winston.transports.Console({
          handleExceptions: true,
          handleRejections: true,
          stderrLevels: ['error', 'warn', 'info', 'debug'], // Force all logs to stderr
        }),
      ],
      exitOnError: false,
    });
  }

  private getFormat(format: string): winston.Logform.Format {
    const timestamp = winston.format.timestamp();
    const errors = winston.format.errors({ stack: true });

    switch (format.toLowerCase()) {
      case 'simple':
        return winston.format.combine(
          timestamp,
          errors,
          winston.format.simple()
        );
      
      case 'json':
      default:
        return winston.format.combine(
          timestamp,
          errors,
          winston.format.json()
        );
    }
  }

  debug(message: string, meta?: any): void {
    this.logger.debug(message, meta);
  }

  info(message: string, meta?: any): void {
    this.logger.info(message, meta);
  }

  warn(message: string, meta?: any): void {
    this.logger.warn(message, meta);
  }

  error(message: string, error?: any): void {
    if (error instanceof Error) {
      this.logger.error(message, {
        error: {
          message: error.message,
          stack: error.stack,
          name: error.name,
        },
      });
    } else if (error) {
      this.logger.error(message, { error });
    } else {
      this.logger.error(message);
    }
  }

  setLevel(level: string): void {
    this.logger.level = level;
  }
}
