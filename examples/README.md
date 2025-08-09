# flunq.io Examples

This folder contains examples for testing the **Worker service** with **Serverless Workflow DSL 1.0.0**.

## 📋 Files

- **`simple-workflow.yaml`** - Basic DSL 1.0.0 workflow definition
- **`postman-requests.md`** - Complete Postman testing guide
- **`README.md`** - This file

## 🚀 Quick Start

### 1. Start Services
```bash
# Terminal 1: Redis
redis-server

# Terminal 2: Worker
cd worker
go run cmd/server/main.go
```

### 2. Test with Postman
Follow the instructions in `postman-requests.md` to:
1. Create a workflow with DSL 1.0.0
2. Start workflow execution
3. Monitor the results

### 3. Expected Flow
```
Postman → API → Shared Event Stream (Redis) → Worker → Executor (for call/wait/inject) → back to API
```

## 🎯 What the Worker Does

1. **Parses DSL 1.0.0** - Supports new YAML-based format
2. **Executes Tasks** - `set`, `wait`, `call`, `emit` tasks
3. **Manages State** - Complete event sourcing with Redis
4. **Logs Progress** - Detailed execution logging

## 📊 Monitoring

**Worker Logs:**
```json
{"msg":"Processing workflow event","type":"io.flunq.workflow.created"}
{"msg":"Successfully parsed workflow definition","spec_version":"1.0.0"}
{"msg":"Executing set task","task_name":"initialize"}
```

**Redis Streams:**
```bash
redis-cli XRANGE events:global - +
redis-cli XRANGE events:workflow:simple-test-workflow - +
```

The Worker service now supports **Serverless Workflow DSL 1.0.0** with complete event-driven orchestration! 🎉
