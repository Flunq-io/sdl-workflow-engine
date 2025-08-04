# Flunq.io - Serverless Workflow Engine

A modern Temporal.io replacement built on top of the Serverless Workflow Definition Language with pluggable eventing and storage backends.

## 🏗️ Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│       UI        │    │       API       │    │     Events      │
│   (Next.js)     │◄──►│      (Go)       │◄──►│      (Go)       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │     Worker      │    │     Tasks       │
                       │      (Go)       │◄──►│      (Go)       │
                       └─────────────────┘    └─────────────────┘
```

## 🚀 Services

### **API Service** (`/api`)
- **Language**: Go
- **Purpose**: HTTP API server, DSL parsing, workflow orchestration
- **Features**: REST/GraphQL endpoints, workflow validation, execution control

### **Events Service** (`/events`) 
- **Language**: Go
- **Purpose**: Event streaming and storage
- **Features**: Pluggable backends (Redis, Kafka, NATS), event sourcing

### **Worker Service** (`/worker`)
- **Language**: Go  
- **Purpose**: Workflow execution engine
- **Features**: DSL interpretation, state management, task scheduling

### **Tasks Service** (`/tasks`)
- **Language**: Go
- **Purpose**: External API integration
- **Features**: HTTP calls, retries, circuit breakers, rate limiting

### **UI Service** (`/ui`)
- **Language**: Next.js/TypeScript
- **Purpose**: Web dashboard and workflow designer
- **Features**: Visual workflow builder, monitoring, debugging

## 🔧 Technology Stack

- **Backend**: Go 1.21+
- **Frontend**: Next.js 14, TypeScript, Tailwind CSS
- **Events**: Redis Streams (default), pluggable to Kafka/NATS
- **Storage**: Redis (default), pluggable to PostgreSQL/MongoDB
- **Deployment**: Docker, Kubernetes
- **Monitoring**: OpenTelemetry, Prometheus

## 🚀 Quick Start

```bash
# Clone the repository
git clone https://github.com/your-org/flunq.io
cd flunq.io

# Start all services with Docker Compose
docker-compose up -d

# Or start individual services
cd api && go run main.go
cd events && go run main.go  
cd worker && go run main.go
cd tasks && go run main.go
cd ui && npm run dev
```

## 📋 Development

Each service is independently deployable with its own:
- Dockerfile
- CI/CD pipeline
- Documentation
- Tests

See individual service READMEs for detailed setup instructions.

## 🎯 Roadmap

- [ ] Core DSL parser and validator
- [ ] Redis-based event streaming
- [ ] Basic workflow execution engine
- [ ] REST API endpoints
- [ ] Web UI dashboard
- [ ] Pluggable storage backends
- [ ] Advanced monitoring and observability
- [ ] Kubernetes operator

## 📄 License

MIT License - see LICENSE file for details.
