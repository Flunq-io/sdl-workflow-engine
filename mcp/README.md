# MCP Server for flunq.io

## ✅ **Status: Working & Tested**

The MCP server is fully functional and tested with Claude Desktop. It successfully:
- ✅ Discovers workflows from flunq.io API
- ✅ Exposes workflows as MCP tools to AI models
- ✅ Executes workflows with proper input validation
- ✅ Handles errors gracefully with meaningful messages
- ✅ Supports cache refresh for immediate workflow discovery

## 🎯 **Overview**

The MCP (Model Context Protocol) server exposes flunq.io workflows as discoverable tools that AI models can execute. This enables AI agents to discover, understand, and execute workflows dynamically.

## 🏗️ **Architecture**

```
AI Model → MCP Client → MCP Server → flunq.io API → Workflow Engine
                                        ↓
                                   Workflow Discovery
                                   Execution Management
                                   I/O Handling
```

## 🚀 **Features**

### **Workflow Discovery**
- **Dynamic Tool Discovery**: Automatically discovers all workflows by tenant
- **ID-based Tool Naming**: Uses workflow IDs for reliable tool identification
- **Schema Generation**: Generates JSON schemas from workflow definitions
- **Metadata Extraction**: Extracts workflow descriptions, parameters, and expected outputs
- **Cache Management**: 30-second cache TTL for fast development iteration

### **Workflow Execution**
- **Tool Execution**: Executes workflows as MCP tools using workflow IDs
- **Input Validation**: Validates input parameters against workflow schemas
- **Output Formatting**: Returns structured workflow outputs
- **Execution Tracking**: Monitors workflow execution status
- **Error Handling**: Provides detailed error messages for debugging

### **Cache Management**
- **Manual Refresh**: `refresh_workflows` tool for immediate cache updates
- **Configurable TTL**: Cache expiry configurable via `FLUNQ_CACHE_TTL` environment variable
- **Automatic Discovery**: New workflows appear after cache expiry or on manual refresh
- **Smart Caching**: Cache is automatically cleared during tool execution to ensure latest workflows

### **Multi-Tenant Support**
- **Tenant Isolation**: Proper tenant scoping for workflow discovery and execution
- **Access Control**: Tenant-based access control for workflow operations

## 📋 **MCP Tool Format**

Each workflow is exposed as an MCP tool using the workflow ID for reliable identification:

```json
{
  "name": "workflow_wf_26e548a7-bd9d-47",
  "description": "Run a sourcing event completely without user interaction for the purchase of goods below 500€",
  "inputSchema": {
    "type": "object",
    "properties": {
      "input": {
        "type": "object",
        "description": "Workflow input data",
        "additionalProperties": true
      }
    },
    "required": [],
    "additionalProperties": false,
    "description": "Workflow input parameters"
  }
}
```

### **Special Tools**

The server also provides utility tools:

#### **Server Status Tool**
```json
{
  "name": "server_status",
  "description": "Server status: X workflows available",
  "inputSchema": {
    "type": "object",
    "properties": {},
    "additionalProperties": false
  }
}
```

#### **Refresh Workflows Tool**
```json
{
  "name": "refresh_workflows",
  "description": "Refresh the workflow cache to discover new workflows",
  "inputSchema": {
    "type": "object",
    "properties": {},
    "additionalProperties": false
  }
}
```

#### **List Workflows Tool**
```json
{
  "name": "list_workflows",
  "description": "List workflows with filtering and pagination options",
  "inputSchema": {
    "type": "object",
    "properties": {
      "page": { "type": "number", "description": "Page number (1-based)" },
      "size": { "type": "number", "description": "Number of items per page" },
      "search": { "type": "string", "description": "Search across workflow name, description, and tags" }
    },
    "additionalProperties": false
  }
}
```

## 🔧 **Implementation**

### **Core Components**

1. **MCP Server**: Implements the Model Context Protocol
2. **Workflow Discovery Service**: Discovers and catalogs workflows
3. **Schema Generator**: Generates JSON schemas from workflow definitions
4. **Execution Manager**: Manages workflow execution and monitoring
5. **API Client**: Interfaces with flunq.io REST API

### **Quick Start**

```bash
# Install dependencies
cd flunq.io/mcp
npm install

# Copy environment configuration
cp .env.example .env

# Edit configuration
vim .env

# Build the project
npm run build

# Start the MCP server
npm start
```

### **Docker Deployment**

```bash
# Build Docker image
docker build -t flunq-mcp-server .

# Run with environment variables
docker run -d \
  --name flunq-mcp \
  -e FLUNQ_API_URL=http://host.docker.internal:8080 \
  -e FLUNQ_DEFAULT_TENANT=acme-inc \
  flunq-mcp-server
```

### **Workflow Discovery Process**

1. **Scan Workflows**: Use `GET /api/v1/{tenant}/workflows` to discover workflows
2. **Extract Metadata**: Parse workflow definitions for input/output schemas
3. **Generate Tools**: Create MCP tool definitions from workflow metadata
4. **Cache Tools**: Cache tool definitions for performance

### **Execution Process**

1. **Validate Input**: Validate tool input against workflow schema
2. **Execute Workflow**: Call `POST /api/v1/{tenant}/workflows/{id}/execute`
3. **Monitor Execution**: Poll execution status via `GET /api/v1/{tenant}/executions/{id}`
4. **Return Output**: Return workflow output when execution completes

## 📊 **Configuration**

Configuration is managed through environment variables. Copy `.env.example` to `.env` and customize:

```bash
# Flunq.io API Configuration
FLUNQ_API_URL=http://localhost:8080
FLUNQ_DEFAULT_TENANT=acme-inc
FLUNQ_REQUEST_TIMEOUT=30000

# Execution Configuration
FLUNQ_EXECUTION_TIMEOUT=300  # 5 minutes
FLUNQ_POLL_INTERVAL=2        # 2 seconds

# Cache Configuration
FLUNQ_CACHE_ENABLED=true
FLUNQ_CACHE_TTL=300          # 5 minutes

# Logging Configuration
LOG_LEVEL=info               # debug, info, warn, error
LOG_FORMAT=json              # json, simple
```

### **Configuration Options**

| Variable | Default | Description |
|----------|---------|-------------|
| `FLUNQ_API_URL` | `http://localhost:8080` | Base URL for flunq.io API |
| `FLUNQ_DEFAULT_TENANT` | `acme-inc` | Default tenant for workflow operations |
| `FLUNQ_REQUEST_TIMEOUT` | `30000` | HTTP request timeout in milliseconds |
| `FLUNQ_EXECUTION_TIMEOUT` | `300` | Workflow execution timeout in seconds |
| `FLUNQ_POLL_INTERVAL` | `2` | Execution status polling interval in seconds |
| `FLUNQ_CACHE_ENABLED` | `true` | Enable/disable workflow caching |
| `FLUNQ_CACHE_TTL` | `300` | Cache time-to-live in seconds |
| `LOG_LEVEL` | `info` | Logging level (debug, info, warn, error) |
| `LOG_FORMAT` | `json` | Log format (json, simple) |

## 🧪 **Testing**

### **Quick Test**
```bash
# Test the MCP server functionality
npm run test:server
```

This will verify:
- ✅ MCP protocol initialization
- ✅ Tool discovery and listing
- ✅ Server status and configuration
- ✅ Workflow count and availability

### **Manual Testing with Claude Desktop**

1. **Add to Claude Desktop config** (`~/Library/Application Support/Claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "flunq": {
      "command": "node",
      "args": ["/path/to/flunq.io/mcp/dist/index.js"],
      "env": {
        "FLUNQ_API_URL": "http://localhost:8080",
        "FLUNQ_DEFAULT_TENANT": "acme-inc"
      }
    }
  }
}
```

2. **Restart Claude Desktop**

3. **Test workflow discovery**: Ask Claude to list available tools

4. **Test workflow execution**: Ask Claude to execute a specific workflow

## 🔧 **Troubleshooting**

### **Common Issues**

| Issue | Solution |
|-------|----------|
| "Tool not available" error | Run `refresh_workflows` tool or check cache TTL |
| Connection timeout | Verify `FLUNQ_API_URL` and network connectivity |
| No workflows found | Check tenant configuration and API permissions |
| Schema validation errors | Verify workflow input schema matches expected format |

### **Debug Mode**
```bash
# Enable debug logging
export LOG_LEVEL=debug
npm run dev
```

### **Health Check**
```bash
# Check if flunq.io API is accessible
curl http://localhost:8080/health

# Test workflow listing
curl http://localhost:8080/api/v1/acme-inc/workflows
```

## 🎯 **Usage Examples**

### **AI Model Interaction**

```typescript
// AI model discovers available tools
const tools = await mcpClient.listTools();
// Returns: [
//   { name: "workflow_user_onboarding", description: "..." },
//   { name: "workflow_payment_processing", description: "..." },
//   { name: "workflow_order_fulfillment", description: "..." }
// ]

// AI model executes a workflow
const result = await mcpClient.callTool("workflow_user_onboarding", {
  email: "user@example.com",
  firstName: "John",
  lastName: "Doe"
});
// Returns: { success: true, userId: "user_123", accountId: "acc_456" }
```

### **Workflow Definition Example**

```yaml
# user-onboarding.yaml
document:
  dsl: '1.0.0'
  namespace: 'user-management'
  name: 'user-onboarding'
  version: '1.0.0'
  description: 'Complete user onboarding process with email validation and account setup'

# MCP will extract this as input schema
input:
  email: 
    type: string
    format: email
    description: "User email address"
  firstName:
    type: string
    description: "User first name"
  lastName:
    type: string
    description: "User last name"

do:
  - validateEmail:
      call: http
      with:
        method: post
        endpoint: https://api.emailvalidation.com/validate
        body:
          email: "${.email}"
  
  - createAccount:
      call: http
      with:
        method: post
        endpoint: https://api.accounts.com/users
        body:
          email: "${.email}"
          firstName: "${.firstName}"
          lastName: "${.lastName}"
```

## 🛠️ **Development Plan**

### **Phase 1: Core MCP Server**
- [ ] Implement basic MCP server with tool discovery
- [ ] Create flunq.io API client
- [ ] Implement workflow discovery service
- [ ] Basic schema generation from workflow definitions

### **Phase 2: Advanced Features**
- [ ] Sophisticated schema inference from SDL definitions
- [ ] Execution monitoring and status updates
- [ ] Error handling and retry logic
- [ ] Caching and performance optimization

### **Phase 3: Production Features**
- [ ] Multi-tenant support
- [ ] Authentication and authorization
- [ ] Metrics and monitoring
- [ ] Configuration management

## 🔍 **Schema Generation Strategy**

The MCP server will analyze workflow definitions to generate JSON schemas:

### **Input Schema Generation**
1. **Parse SDL Definition**: Extract input parameters from workflow definition
2. **Analyze Task Inputs**: Look for variable references in task configurations
3. **Generate JSON Schema**: Create JSON schema from extracted parameters
4. **Add Descriptions**: Use workflow metadata for parameter descriptions

### **Output Schema Generation**
1. **Analyze Final Tasks**: Look at final task outputs
2. **Extract Return Values**: Identify workflow return values
3. **Generate Output Schema**: Create schema for expected outputs

## 🚀 **Benefits**

### **For AI Models**
- **Dynamic Discovery**: Automatically discover available workflows
- **Type Safety**: JSON schema validation for inputs and outputs
- **Rich Metadata**: Detailed descriptions and parameter information
- **Execution Monitoring**: Real-time execution status and results

### **For Developers**
- **Zero Configuration**: Workflows automatically become AI tools
- **Standard Protocol**: Uses industry-standard MCP protocol
- **Flexible Integration**: Works with any MCP-compatible AI model
- **Observability**: Full execution tracking and monitoring

## 🎯 **Integration with AI Models**

### **Claude Desktop Integration**

Add to your Claude Desktop configuration (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "flunq": {
      "command": "node",
      "args": ["/path/to/flunq.io/mcp/dist/index.js"],
      "env": {
        "FLUNQ_API_URL": "http://localhost:8080",
        "FLUNQ_DEFAULT_TENANT": "acme-inc",
        "FLUNQ_REQUEST_TIMEOUT": "30000",
        "FLUNQ_EXECUTION_TIMEOUT": "300",
        "FLUNQ_POLL_INTERVAL": "2",
        "FLUNQ_CACHE_ENABLED": "true",
        "FLUNQ_CACHE_TTL": "300",
        "LOG_LEVEL": "info",
        "LOG_FORMAT": "json"
      }
    }
  }
}
```

**Note**: Replace `/path/to/flunq.io/mcp/dist/index.js` with the actual path to your compiled MCP server.

### **Example AI Interaction**

```
Human: What workflow tools do you have available?

AI: I have access to the following workflow tools:

1. **workflow_wf_26e548a7-bd9d-47** - "Run a sourcing event completely without user interaction for the purchase of goods below 500€"
2. **workflow_wf_57135251-a1a4-46** - "Testing DSL 1.0.0 with multi-tenant Worker"
3. **refresh_workflows** - "Refresh the workflow cache to discover new workflows"

## 🔧 **Troubleshooting**

### **Workflows Not Appearing**
1. **Check API Connection**: Ensure flunq.io API is running on the configured URL
2. **Verify Tenant**: Make sure `FLUNQ_DEFAULT_TENANT` matches your workflow tenant
3. **Refresh Cache**: Use the `refresh_workflows` tool to update the cache
4. **Check Logs**: Look at stderr output for error messages

### **Workflow Execution Fails**
1. **Verify Workflow ID**: Ensure the workflow exists in the specified tenant
2. **Check Input Format**: Validate input parameters match the expected schema
3. **API Connectivity**: Confirm the flunq.io API is accessible and responsive

### **MCP Server Not Starting**
1. **Build First**: Run `npm run build` to compile TypeScript
2. **Check Dependencies**: Ensure all npm packages are installed
3. **Environment Variables**: Verify all required environment variables are set

## Development

### Prerequisites

- Node.js 18+
- TypeScript
- Access to flunq.io platform

### Quick Start

1. **Install dependencies**:
   ```bash
   npm install
   ```

2. **Build the project**:
   ```bash
   npm run build
   ```

3. **Start flunq.io API** (in another terminal):
   ```bash
   cd ../api && go run cmd/server/main.go
   ```

4. **Configure Claude Desktop** with the path to `dist/index.js`

5. **Restart Claude Desktop** to load the MCP server

6. **Test the integration**:
   - Ask Claude: "What workflow tools do you have available?"
   - Use: "Use the refresh_workflows tool to update the cache"

### Installation

```bash
npm install
```

### Building

```bash
npm run build
```

### Running

```bash
npm start
```

### Testing

```bash
npm test
```

## 🗺️ **Roadmap**

### **🔐 Security & Authentication**
- [ ] **API Key Authentication** - Secure API access with key-based auth
- [ ] **JWT Token Support** - Integration with flunq.io's authentication system
- [ ] **Role-Based Access Control (RBAC)** - Workflow access based on user roles
- [ ] **Tenant Isolation** - Strict tenant-based workflow access controls
- [ ] **Audit Logging** - Track all workflow executions and access attempts

### **🚀 Performance & Scalability**
- [ ] **Connection Pooling** - Efficient API connection management
- [ ] **Request Rate Limiting** - Prevent API abuse and ensure fair usage
- [ ] **Caching Improvements** - Redis-based caching for workflow metadata
- [ ] **Batch Operations** - Execute multiple workflows efficiently
- [ ] **Streaming Results** - Real-time workflow execution updates

### **🔧 Developer Experience**
- [ ] **Workflow Validation** - Pre-execution input validation and schema checking
- [ ] **Interactive Debugging** - Step-through workflow execution for development
- [ ] **Workflow Templates** - Pre-built workflow templates for common use cases
- [ ] **Auto-completion** - Smart input suggestions based on workflow schemas
- [ ] **Workflow Documentation** - Auto-generated docs from workflow definitions

### **📊 Monitoring & Observability**
- [ ] **Metrics Collection** - Prometheus/OpenTelemetry integration
- [ ] **Health Checks** - Comprehensive health monitoring for all components
- [ ] **Performance Monitoring** - Track workflow execution times and success rates
- [ ] **Error Tracking** - Detailed error reporting and alerting
- [ ] **Usage Analytics** - Workflow usage patterns and optimization insights

### **🔌 Integration & Extensibility**
- [ ] **Webhook Support** - Trigger workflows via HTTP webhooks
- [ ] **Event-Driven Execution** - React to external events and triggers
- [ ] **Custom Executors** - Plugin system for custom workflow steps
- [ ] **Multi-Tenant API Keys** - Separate API keys per tenant/organization
- [ ] **Workflow Marketplace** - Share and discover workflow templates

### **🛡️ Enterprise Features**
- [ ] **SSO Integration** - SAML/OIDC authentication support
- [ ] **Compliance Logging** - SOX/GDPR compliant audit trails
- [ ] **Data Encryption** - End-to-end encryption for sensitive workflow data
- [ ] **Backup & Recovery** - Automated backup of workflow definitions and executions
- [ ] **High Availability** - Multi-region deployment support

### **📱 User Interface**
- [ ] **Web Dashboard** - Browser-based workflow management interface
- [ ] **Mobile App** - iOS/Android app for workflow monitoring
- [ ] **VS Code Extension** - IDE integration for workflow development
- [ ] **CLI Tool** - Command-line interface for workflow operations
- [ ] **Slack/Teams Integration** - Chat-based workflow execution and monitoring

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - see LICENSE file for details.
