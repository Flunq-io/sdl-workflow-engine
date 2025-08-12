# OpenAPI Call Tasks

The Executor service now supports OpenAPI call tasks, enabling workflows to interact with external services described by OpenAPI specifications. This provides a more structured and type-safe way to make API calls compared to raw HTTP requests.

## 🚀 Features

- **OpenAPI 3.0 Support**: Full support for OpenAPI 3.0.x specifications
- **Operation Resolution**: Find operations by `operationId`
- **Parameter Handling**: Automatic handling of path, query, and header parameters
- **Request Body Support**: JSON request body construction
- **Authentication**: Support for Bearer, Basic, and API Key authentication
- **Response Processing**: Flexible response format options
- **Document Caching**: Automatic caching of OpenAPI documents
- **Backward Compatibility**: Existing HTTP calls continue to work unchanged

## 📋 Configuration

### **OpenAPI Call Configuration**

```json
{
  "call_type": "openapi",
  "document": {
    "endpoint": "https://petstore.swagger.io/v2/swagger.json"
  },
  "operation_id": "findPetsByStatus",
  "parameters": {
    "status": "available",
    "limit": 10
  },
  "authentication": {
    "type": "bearer",
    "config": {
      "token": "your-api-token"
    }
  },
  "output": "content"
}
```

### **HTTP Call Configuration (Backward Compatibility)**

```json
{
  "call_type": "http",
  "url": "https://api.example.com/pets",
  "method": "GET",
  "headers": {
    "Authorization": "Bearer your-token"
  },
  "query": {
    "status": "available"
  }
}
```

## 🎯 Usage Examples

### **Example 1: Pet Store API**

```yaml
# Workflow DSL
do:
  - findPets:
      call: openapi
      with:
        document: 
          endpoint: https://petstore.swagger.io/v2/swagger.json
        operationId: findPetsByStatus
        parameters:
          status: available
          limit: 10
```

**Generated Task Configuration:**
```json
{
  "task_type": "call",
  "config": {
    "parameters": {
      "call_type": "openapi",
      "document": {
        "endpoint": "https://petstore.swagger.io/v2/swagger.json"
      },
      "operation_id": "findPetsByStatus",
      "parameters": {
        "status": "available",
        "limit": 10
      },
      "output": "content"
    }
  }
}
```

### **Example 2: Get Specific Pet**

```yaml
# Workflow DSL
do:
  - getPet:
      call: openapi
      with:
        document: 
          endpoint: https://petstore.swagger.io/v2/swagger.json
        operationId: getPetById
        parameters:
          petId: ${ .petId }
        authentication:
          type: apikey
          config:
            key: api_key
            value: ${ .apiKey }
            in: header
```

### **Example 3: Create New Pet**

```yaml
# Workflow DSL
do:
  - createPet:
      call: openapi
      with:
        document: 
          endpoint: https://petstore.swagger.io/v2/swagger.json
        operationId: addPet
        parameters:
          body:
            name: ${ .petName }
            status: available
            category:
              id: 1
              name: Dogs
        authentication:
          type: bearer
          config:
            token: ${ .authToken }
```

## 🔧 Configuration Options

### **Document Reference**

```json
{
  "document": {
    "endpoint": "https://api.example.com/openapi.json"
  }
}
```

**Supported formats:**
- JSON OpenAPI documents
- YAML support (planned for future release)
- HTTP/HTTPS endpoints
- Document caching (5-minute default TTL)

### **Authentication Types**

#### **Bearer Token**
```json
{
  "authentication": {
    "type": "bearer",
    "config": {
      "token": "your-jwt-token"
    }
  }
}
```

#### **Basic Authentication**
```json
{
  "authentication": {
    "type": "basic",
    "config": {
      "username": "your-username",
      "password": "your-password"
    }
  }
}
```

#### **API Key**
```json
{
  "authentication": {
    "type": "apikey",
    "config": {
      "key": "X-API-Key",
      "value": "your-api-key",
      "in": "header"
    }
  }
}
```

### **Output Formats**

#### **Content (Default)**
Returns the parsed response content:
```json
{
  "status_code": 200,
  "headers": {...},
  "data": {
    "id": 123,
    "name": "Fluffy",
    "status": "available"
  },
  "request": {...}
}
```

#### **Response**
Returns the full HTTP response:
```json
{
  "status_code": 200,
  "headers": {...},
  "body": "{\"id\":123,\"name\":\"Fluffy\"}",
  "request": {...}
}
```

#### **Raw**
Returns raw response data:
```json
{
  "status_code": 200,
  "headers": {...},
  "body": [bytes],
  "request": {...}
}
```

## 🔄 Parameter Mapping

### **Path Parameters**
Automatically substituted in the URL path:
```json
{
  "operation_id": "getPetById",
  "parameters": {
    "petId": "123"
  }
}
```
URL: `/pets/{petId}` → `/pets/123`

### **Query Parameters**
Added to the URL query string:
```json
{
  "operation_id": "findPets",
  "parameters": {
    "status": "available",
    "limit": 10
  }
}
```
URL: `/pets` → `/pets?status=available&limit=10`

### **Header Parameters**
Added to HTTP request headers:
```json
{
  "operation_id": "getPets",
  "parameters": {
    "X-Request-ID": "req-123"
  }
}
```

### **Request Body**
JSON request body construction:
```json
{
  "operation_id": "createPet",
  "parameters": {
    "body": {
      "name": "Fluffy",
      "status": "available"
    }
  }
}
```

## 🛡️ Error Handling

### **Document Loading Errors**
- Invalid OpenAPI document URL
- Network connectivity issues
- Document parsing failures
- Unsupported OpenAPI versions

### **Operation Resolution Errors**
- Operation ID not found
- Invalid operation definition
- Missing required parameters

### **Request Execution Errors**
- HTTP request failures
- Authentication errors
- Parameter validation errors
- Response parsing errors

### **Example Error Response**
```json
{
  "task_id": "call-task-123",
  "success": false,
  "error": "OpenAPI operation failed: operation with id 'invalidOp' not found",
  "duration": "150ms"
}
```

## 📊 Monitoring & Debugging

### **Logging**
The executor provides detailed logging for OpenAPI operations:

```json
{
  "level": "info",
  "msg": "Executing OpenAPI operation",
  "task_id": "call-task-123",
  "operation_id": "getPetById",
  "document": "https://petstore.swagger.io/v2/swagger.json",
  "method": "GET",
  "url": "https://petstore.swagger.io/v2/pet/123",
  "status_code": 200,
  "duration": "245ms"
}
```

### **Request/Response Tracking**
Each response includes request metadata:
```json
{
  "request": {
    "method": "GET",
    "url": "https://petstore.swagger.io/v2/pet/123",
    "headers": {
      "Authorization": "Bearer ***",
      "Content-Type": "application/json"
    },
    "operation_id": "getPetById"
  }
}
```

## 🚀 Performance

### **Document Caching**
- OpenAPI documents are cached for 5 minutes by default
- Reduces network overhead for repeated operations
- Automatic cache cleanup for expired entries

### **HTTP Client Optimization**
- 30-second request timeout
- Connection reuse for multiple requests
- Proper header management

### **Memory Usage**
- Efficient JSON parsing and processing
- Minimal memory footprint for cached documents
- Garbage collection friendly design

## 🔗 Related Documentation

- [Executor Service README](../README.md) - Complete Executor service documentation
- [Event Processing Quick Reference](../../docs/EVENT_PROCESSING_QUICK_REFERENCE.md) - Event processing patterns
- [Serverless Workflow DSL Reference](https://github.com/serverlessworkflow/specification/blob/main/dsl-reference.md) - Official DSL documentation
