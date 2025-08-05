# Flunq.io UI

Modern, minimalistic web dashboard for the Flunq.io workflow engine. Built with Next.js 14, React Query, and Tailwind CSS.

## ✨ Features

- **Workflow Management** - View and monitor all workflows with real-time status updates
- **Event Timeline** - Detailed event history with interactive timeline visualization
- **SDL Compliant** - Full support for Serverless Workflow DSL 1.0.0 status types
- **Real-time Updates** - Automatic refresh of workflow status and events
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
- Execution details
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
