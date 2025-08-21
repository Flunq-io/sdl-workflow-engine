# Changelog

## [0.1.0] - 2025-08-21

### 🎉 Initial Release - Clean & Production Ready

This release represents a comprehensive cleanup and enhancement of the Flunq MCP Server, making it production-ready with proper configuration management, error handling, and documentation.

### ✨ Features

#### **Core Functionality**
- ✅ **Dynamic Workflow Discovery**: Automatically discovers workflows from flunq.io API
- ✅ **MCP Tool Generation**: Converts workflows to MCP tools with proper schemas
- ✅ **Workflow Execution**: Executes workflows with input validation and output handling
- ✅ **Multi-tenant Support**: Proper tenant isolation and scoping

#### **Utility Tools**
- ✅ **Server Status Tool**: `server_status` - Shows server configuration and workflow count
- ✅ **Cache Refresh Tool**: `refresh_workflows` - Manually refresh workflow cache
- ✅ **Workflow Listing Tool**: `list_workflows` - List workflows with filtering and pagination

#### **Configuration Management**
- ✅ **Environment-based Configuration**: All settings configurable via environment variables
- ✅ **Configuration Validation**: Validates all configuration values on startup
- ✅ **Flexible Defaults**: Sensible defaults for all configuration options

### 🔧 Technical Improvements

#### **Code Quality**
- ✅ **Removed Hardcoded Values**: All constants extracted to dedicated constants file
- ✅ **Consistent Configuration**: Logger and all components use centralized Config class
- ✅ **Type Safety**: Full TypeScript implementation with proper type definitions
- ✅ **Error Handling**: Comprehensive error handling with meaningful messages

#### **Cache Management**
- ✅ **Configurable Cache TTL**: Cache timeout configurable via `FLUNQ_CACHE_TTL`
- ✅ **Smart Cache Clearing**: Cache automatically cleared during tool execution
- ✅ **Manual Cache Control**: `refresh_workflows` tool for immediate cache updates

#### **Logging & Debugging**
- ✅ **Structured Logging**: JSON-formatted logs with configurable levels
- ✅ **Debug Support**: Debug mode with detailed execution logging
- ✅ **Request Tracing**: Full request/response logging for API calls

### 📚 Documentation

#### **Comprehensive README**
- ✅ **Setup Instructions**: Complete setup and configuration guide
- ✅ **Configuration Reference**: Detailed environment variable documentation
- ✅ **Testing Guide**: Instructions for testing with Claude Desktop
- ✅ **Troubleshooting**: Common issues and solutions

#### **Development Tools**
- ✅ **Test Script**: Automated test script to verify server functionality
- ✅ **Environment Template**: `.env.example` with all configuration options
- ✅ **Docker Support**: Dockerfile and Docker deployment instructions

### 🛠️ Infrastructure

#### **Build & Development**
- ✅ **TypeScript Build**: Clean TypeScript compilation with proper module resolution
- ✅ **Development Mode**: Hot-reload development server with `tsx`
- ✅ **Testing**: Automated test script for CI/CD integration

#### **Deployment**
- ✅ **Docker Ready**: Complete Dockerfile for containerized deployment
- ✅ **Environment Isolation**: Proper environment variable handling
- ✅ **Health Checks**: Built-in health check endpoints

### 🔒 Security & Best Practices

#### **Configuration Security**
- ✅ **No Hardcoded Secrets**: All sensitive values via environment variables
- ✅ **Gitignore Protection**: Comprehensive `.gitignore` to prevent secret commits
- ✅ **Environment Template**: Safe `.env.example` for documentation

#### **Error Handling**
- ✅ **Graceful Degradation**: Server continues operating even with API issues
- ✅ **Detailed Error Messages**: Clear error messages for debugging
- ✅ **Timeout Handling**: Proper timeout handling for all operations

### 📊 Configuration Options

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

### 🧪 Testing

The release includes comprehensive testing:
- ✅ **Automated Test Script**: `npm run test:server`
- ✅ **MCP Protocol Validation**: Tests initialization, tool listing, and execution
- ✅ **Configuration Validation**: Verifies all configuration options work correctly
- ✅ **Error Handling Tests**: Validates graceful error handling

### 🚀 Getting Started

```bash
# Clone and setup
cd flunq.io/mcp
npm install

# Configure environment
cp .env.example .env
# Edit .env with your settings

# Build and run
npm run build
npm start

# Or run in development mode
npm run dev

# Test the server
npm run test:server
```

### 🔄 Migration Notes

This is the initial production-ready release. Previous development versions should be replaced entirely with this version.

### 🎯 Next Steps

- [ ] Add comprehensive unit tests with Jest
- [ ] Implement workflow schema caching
- [ ] Add metrics and monitoring endpoints
- [ ] Support for workflow versioning
- [ ] Enhanced error recovery mechanisms
