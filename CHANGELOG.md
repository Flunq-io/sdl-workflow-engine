# Changelog

All notable changes to the flunq.io project will be documented in this file.

## [Unreleased]

### Changed
- **BREAKING**: Renamed `tasks` service to `executor` service
  - Updated service name from "Tasks Service" to "Executor Service"
  - Changed directory from `/tasks` to `/executor`
  - Updated Docker Compose service name from `tasks` to `executor`
  - Updated API endpoints from `/api/v1/tasks/*` to `/api/v1/execute*`
  - Updated Go module from `github.com/flunq-io/tasks` to `github.com/flunq-io/executor`
  - Updated all references in documentation and configuration files

### Technical Details
- Updated `docker-compose.yml` service definition
- Updated `Makefile` targets and commands
- Updated example workflow definitions to use new executor endpoints
- Updated environment variable `TASKS_URL` to `EXECUTOR_URL`
- Updated health check endpoints and monitoring commands

### Migration Guide
If you have existing deployments:

1. **Stop existing services**:
   ```bash
   docker-compose down
   ```

2. **Update your configuration**:
   - Change any `TASKS_URL` environment variables to `EXECUTOR_URL`
   - Update any hardcoded references from `tasks:8083` to `executor:8083`

3. **Rebuild and restart**:
   ```bash
   make build
   make start
   ```

4. **Update workflow definitions**:
   - Change function operations from `http://tasks:8083/api/v1/tasks/execute` 
   - To: `http://executor:8083/api/v1/execute`

### Rationale
The rename from "tasks" to "executor" better reflects the service's purpose:
- **Tasks** implied simple task management
- **Executor** better represents external API execution and integration capabilities
- Aligns with common terminology in workflow engines
- Avoids confusion with workflow "tasks" vs service "tasks"
