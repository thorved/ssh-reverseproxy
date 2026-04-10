# SSH ReverseProxy

OIDC-secured SSH reverse proxy with a Go backend, Next.js frontend, SQLite storage, and per-user instance assignments.

## Stack

- `backend/`: Go, Gin, GORM, SQLite, OIDC, SSH proxy
- `frontend/`: Next.js App Router, TypeScript, shadcn-style UI primitives
- Auth: OIDC-only with backend-issued HTTP-only session cookies
- Routing: SSH key identifies the user, SSH username selects the assigned instance slug

## Project Structure

```text
ssh-reverseproxy/
├── .dockerignore
├── backend/
│   ├── cmd/server
│   ├── internal/{auth,config,database,handlers,middleware,models,proxy,routes,sshkeys}
│   ├── .air.toml
│   └── .env.example
├── docker-compose.yml
├── Dockerfile
├── frontend/
│   ├── src/app
│   ├── src/components
│   ├── src/contexts
│   ├── src/lib
│   └── .env.example
└── README.md
```

## Local Development

### 1. Backend

```powershell
cd backend
Copy-Item .env.example .env
go mod tidy
go install github.com/air-verse/air@latest
air
```

The backend runs the HTTP API and the SSH proxy in one process:

- HTTP API: `http://localhost:8080`
- SSH proxy: `:2222`

### 2. Frontend

```powershell
cd frontend
Copy-Item .env.example .env.local
npm install
npm run dev
```

The frontend runs on `http://localhost:3000` and talks to the Go backend using `NEXT_PUBLIC_DEV_API_URL`.

## Production Deployment

The production image builds the Next frontend as a static export, copies the exported files into the runtime image, and has the Go server serve both the frontend and `/api/*` from the same origin.

```powershell
docker compose up -d --build
```

Published ports:

- HTTP UI + API: `http://localhost:8080`
- SSH proxy: `:2222`

Persistent data is mounted from `backend/data` to `/app/data` in the container.

## Required Backend Configuration

Set these in `backend/.env`:

- `OIDC_ISSUER_URL`
- `OIDC_CLIENT_ID`
- `OIDC_CLIENT_SECRET`
- `OIDC_REDIRECT_URL`
- `ADMIN_EMAILS`

SQLite data is stored at `DATABASE_PATH`.

## Core Behavior

### Login

- OIDC only
- Matching emails in `ADMIN_EMAILS` become admins on login
- Users can be pre-created by admins or auto-created on first OIDC login

### Admin Features

- Create and update users
- Create and update instances
- Assign one instance to one user at a time

### User Features

- View assigned instances
- Add, update, and delete their own SSH public keys

### SSH Flow

Users connect like this:

```bash
ssh <instance-slug>@<proxy-host> -p 2222
```

- The SSH key maps to a user
- The SSH username maps to an assigned instance slug
- The proxy opens the configured upstream connection for that instance

## Verification

Backend:

```powershell
cd backend
go test ./...
```

Frontend:

```powershell
cd frontend
npm run lint
npm run build
```

Docker:

```powershell
docker compose build
docker compose up
```
