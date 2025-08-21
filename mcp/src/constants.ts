/**
 * Constants for the Flunq MCP Server
 */

// Read package.json to get name and version
import { readFileSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

let packageInfo: { name: string; version: string };

try {
  const packagePath = join(__dirname, '..', 'package.json');
  const packageContent = readFileSync(packagePath, 'utf-8');
  packageInfo = JSON.parse(packageContent);
} catch (error) {
  // Fallback values if package.json can't be read
  packageInfo = {
    name: '@flunq/mcp-server',
    version: '0.1.0'
  };
}

export const SERVER_INFO = {
  NAME: 'flunq-mcp-server',
  VERSION: packageInfo.version,
  FULL_NAME: packageInfo.name,
} as const;

export const TOOL_NAMES = {
  SERVER_STATUS: 'server_status',
  REFRESH_WORKFLOWS: 'refresh_workflows',
  LIST_WORKFLOWS: 'list_workflows',
  WORKFLOW_PREFIX: 'workflow_',
} as const;

export const MESSAGES = {
  SERVER_STARTED: 'Flunq MCP Server started successfully',
  DISCOVERING_WORKFLOWS: 'Discovering workflows...',
  WORKFLOW_CACHE_CLEARED: 'Workflow cache cleared',
  WORKFLOW_CACHE_REFRESHED: 'Workflow cache refreshed',
} as const;
