# Flunq.io UI

Modern, minimalistic web dashboard for the Flunq.io workflow engine. Built with Next.js 14, React Query, and Tailwind CSS.

## ✨ Features

- **Workflow Management** - View and monitor all workflows with real-time status updates
- **Enhanced Event Timeline** - Interactive timeline with complete I/O data visualization
- **I/O Data Display** - Collapsible workflow and task input/output data with color coding
- **SDL Compliant** - Full support for Serverless Workflow DSL 1.0.0 status types
- **Real-time Updates** - Automatic refresh of workflow status and events
- **Internationalization** - Support for 6 languages (EN, FR, DE, ES, ZH, NL)
- **Dark/Light Mode** - Beautiful theme switching with system preference detection
- **Responsive Design** - Works perfectly on desktop, tablet, and mobile
- **Temporal-like Interface** - Familiar, professional workflow monitoring experience

## 🏗️ Architecture

```
┌─────────────────┐
│   Next.js App   │
│   (React 18)    │
└─────────┬───────┘
          │
┌─────────▼───────┐
│  Workflow       │
│  Designer       │
└─────────┬───────┘
          │
┌─────────▼───────┐
│  Dashboard      │
│  (Monitoring)   │
└─────────┬───────┘
          │
┌─────────▼───────┐
│  API Client     │
│  (REST/GraphQL) │
└─────────────────┘
```

## 🚀 Features

- **Workflow Designer**: Visual drag-and-drop workflow builder
- **Dashboard**: Real-time monitoring and metrics
- **Execution Viewer**: Detailed execution logs and state
- **Schema Editor**: DSL schema validation and editing
- **User Management**: Authentication and authorization
- **Dark/Light Theme**: Customizable UI themes

## 📁 Structure

```
ui/
├── app/
│   ├── (dashboard)/
│   ├── workflows/
│   ├── executions/
│   └── settings/
├── components/
│   ├── ui/
│   ├── workflow/
│   └── dashboard/
├── lib/
│   ├── api/
│   ├── auth/
│   └── utils/
├── public/
├── styles/
├── types/
├── package.json
├── next.config.js
├── tailwind.config.js
└── README.md
```

## 🔧 Configuration

Environment variables:
- `NEXT_PUBLIC_API_URL`: API service URL
- `NEXT_PUBLIC_WS_URL`: WebSocket URL for real-time updates
- `NEXTAUTH_SECRET`: NextAuth.js secret
- `NEXTAUTH_URL`: NextAuth.js URL

## 🚀 Quick Start

```bash
# Install dependencies
npm install

# Run development server
npm run dev

# Build for production
npm run build
npm start

# Run with Docker
docker build -t flunq-ui .
docker run -p 3000:3000 flunq-ui
```

## 📱 Pages

### Dashboard (`/`)
- Workflow execution metrics
- System health status
- Recent activity feed
- Quick actions

### Workflows (`/workflows`)
- List all workflows
- Create new workflow
- Edit workflow definition
- Delete workflow

### Workflow Designer (`/workflows/[id]/design`)
- Visual workflow builder
- Drag-and-drop interface
- State configuration
- Connection management

### Executions (`/executions`)
- List all executions
- Filter by status/workflow
- Execution details with enhanced event timeline
- Complete I/O data visualization
- Logs and traces

### Settings (`/settings`)
- User preferences
- System configuration
- API keys management
- Theme settings

## 🎨 Components

### Workflow Designer
- **Canvas**: Main design area
- **Palette**: Available states/actions
- **Properties Panel**: Configure selected elements
- **Minimap**: Navigate large workflows

### Enhanced Event Timeline
- **I/O Data Visualization**: Color-coded input (blue) and output (green) data sections
- **Collapsible Data Display**: Click to expand/collapse JSON data with field counts
- **Smart Data Extraction**: Automatically detects protobuf vs legacy JSON fields
- **Internationalization**: Translated labels for all UI elements
- **Progressive Disclosure**: Raw event data hidden when I/O data is available
- **Real-time Updates**: Live event streaming with automatic timeline updates

### Dashboard Widgets
- **Metrics Cards**: Key performance indicators
- **Charts**: Execution trends and statistics
- **Activity Feed**: Recent events and actions
- **Status Grid**: Service health overview

## 🔌 Integrations

- **API Client**: REST and GraphQL clients
- **WebSocket**: Real-time updates
- **Authentication**: NextAuth.js with multiple providers
- **State Management**: Zustand for client state
- **Forms**: React Hook Form with Zod validation
