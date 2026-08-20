# 🗄️ UrSchema — Database Management Simplified

> A modern, standalone, web-based MySQL/PostgreSQL/SQLite management tool. Lightweight, fast, and built for server operations.

<p align="center">
  <img src="www/public/icon.png" width="96" alt="UrSchema Icon" />
</p>

<p align="center">
  <strong>UrSchema</strong> is a self-contained database management application, providing a visual database experience similar to Navicat / DBeaver.
</p>

<p align="center">
  <a href="https://urschema.app">urschema.app</a>
</p>

---

## ✨ Features

### Connection Management
- 🔐 AES-256-GCM encrypted credential storage
- 🔌 Multiple concurrent connections with automatic pool recycling
- ✅ Connection testing & one-click save
- 🗂️ Server-side file browser for picking SQLite database files

### Data Operations
- 📊 Paginated data browsing (Grid View)
- 📝 Inline editing — modify data directly in the grid
- 📋 Form View — view and edit individual records vertically
- ➕ Add rows / 🗑️ Delete rows with batch commit
- 🔍 Safe CRUD for tables with and without primary keys

### SQL Query
- ⚡ SQL Editor powered by CodeMirror 6
- 📜 Query history with re-execution support
- 🎯 Multi-statement execution, `Ctrl+Enter` shortcut

### Table Structure Designer
- 🏗️ Visual structure editor (columns, indexes, foreign keys)
- 📝 DDL preview with real-time diff comparison
- 🔄 Auto-generated ALTER TABLE statements

### Import / Export
- 📤 Database export (mysqldump compatible + pure Go fallback)
- 📥 SQL file import with automatic database creation
- 📑 Table-level data export (CSV / JSON / SQL formats)
- 📋 Task management panel with real-time progress tracking
- 🗂️ Server-side file/folder browser for picking import and export paths

### Database Overview
- 📈 Table/view listing with row count, size, and engine info
- 🔎 Object search & filter
- 📄 Quick DDL preview & copy

### Internationalization
- 🌐 UI strings and backend error messages are key-based (vue-i18n on the frontend, an ini-backed catalog on the backend), so adding another language is a matter of adding a translation file — no hardcoded strings.

---

## 🏗️ Architecture

```
┌───────────────────────────────┐
│           Browser             │
│   Vue 3 + Naive UI (SPA)      │
└───────────────┬───────────────┘
                │ HTTP  (POST /api/<method>)
┌───────────────▼───────────────┐
│         Go Backend            │
│  net/http · JSON-RPC dispatch │
│  serves API + static frontend │
└───────────────┬───────────────┘
                │
        ┌───────▼───────┐
        │  MySQL / PG /  │
        │    SQLite      │
        └────────────────┘
```

| Layer | Tech Stack |
|-------|-----------|
| **Frontend** | Vue 3 + Naive UI + CodeMirror 6 + Pinia |
| **Backend** | Go + simplejrpc-go, exposed over a plain HTTP endpoint |
| **Storage** | AES-256-GCM encrypted JSON (connection configs) |

The backend serves both the JSON API (`/api/<method>`) and the built frontend's static assets from a single process — no separate gateway or reverse proxy is required to run UrSchema.

The UI ships with a dark theme by default (no light-mode toggle) and paints its own background, so it renders correctly in any browser tab without relying on a host application.

---

## 📁 Project Structure

```
UrSchema/
├── backend/                 # Go backend
│   ├── handler/rpc/         # JSON-RPC route handlers + HTTP server
│   ├── service/             # Business logic layer
│   ├── pkg/
│   │   ├── config/          # Connection store (encrypted)
│   │   ├── db/              # Database abstraction layer
│   │   ├── files/           # File utilities
│   │   └── rpcutil/         # RPC utilities
│   ├── i18n/                # Backend i18n
│   ├── Makefile             # Build script
│   └── main.go              # Entry point
│
├── www/                     # Vue frontend
│   ├── src/
│   │   ├── components/      # Core components
│   │   ├── views/           # Page layouts
│   │   ├── stores/          # Pinia state management
│   │   ├── styles/          # CSS design system
│   │   ├── i18n/            # Frontend i18n
│   │   └── utils/           # API client & UI helpers (toasts, dialogs)
│   └── vite.config.js
│
├── docs/                    # Architecture docs
└── landing/                 # Marketing site for urschema.app
```

---

## 🚀 Getting Started

### Prerequisites

- **Go** ≥ 1.21
- **Node.js** ≥ 18 + **pnpm**

### Development

The quickest way to get both servers running is `make start`, which installs any missing Go/pnpm dependencies (`go mod tidy` + `pnpm install`), then launches the backend and frontend dev servers together (via `concurrently`) and wires the frontend proxy to the backend port automatically:

```bash
cd backend
make start        # backend on :8080, frontend on http://localhost:5173
```

#### Using a custom port

The backend defaults to port `8080`. If that port is already taken (e.g. by another service), override it with `PORT`:

```bash
PORT=8888 make start   # backend on :8888, frontend proxy follows automatically
```

To run each server manually instead:

```bash
# 1. Backend dev server (listens on :8080 by default; override with PORT=xxxx)
cd backend
go mod tidy
go run main.go

# 2. Frontend dev server (proxies /api to the backend on the same port, see www/vite.config.js)
cd www
pnpm install
BACKEND_PORT=8888 pnpm dev   # http://localhost:5173, proxying to a custom backend port
```

### Build & Package

```bash
cd backend

make help         # Show all build targets
make amd64        # Build linux/amd64
make arm64        # Build linux/arm64
make package      # Build frontend + dual-arch packages (tar.gz)
```

Build output structure:
```
build/linux-amd64/
├── app/
│   ├── bin/      # Backend binary + config + i18n
│   └── www/      # Frontend static files, served by the backend
├── data/         # Runtime data
├── logs/         # Logs
└── tmp/          # Temporary files
```

Running the packaged binary (`app/bin/main`) starts an HTTP server that serves both the API and the frontend from `app/www` — open it directly in a browser, no other services required.

### Access Logging

The backend supports four levels of Apache-style access logging, off by default:

| Flag | Output |
|------|--------|
| `-v` | Apache Common Log Format |
| `-vv` | Apache Combined Log Format |
| `-vvv` | Combined Log Format + request/response headers (plain text, `curl -v` style) |
| `-dump` (or `-dump=text`) | Same as `-vvv`, plus the full request payload and response body — structured JSON by default, or plain text with `-dump=text` |

```bash
./main -v            # PORT=8080 ./main -v
./main -dump          # full request/response dump, JSON
./main -dump=text     # full request/response dump, plain text
```

---

## 📸 Screenshots

> *Coming soon*

---

## 🤝 Contributing

Contributions are welcome! Please follow these guidelines:

1. Fork this repository
2. Create a feature branch: `git checkout -b feature/your-feature`
3. Commit your changes: `git commit -m 'feat: add some feature'`
4. Push to the branch: `git push origin feature/your-feature`
5. Open a Pull Request

### Development Guidelines

- **Backend**: Layered architecture `handler → service → pkg`, i18n error messages
- **Frontend**: Follow design specs under `docs/designs/` (tokens, typography, spacing)
- **Commits**: Use [Conventional Commits](https://www.conventionalcommits.org/)

---

## 📄 License

[MIT](LICENSE)
