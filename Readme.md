![alt text](image.png)
# ResQio

ResQio is a verified disaster resource portal for reporting hazards, finding emergency resources, requesting assistance, and coordinating providers.

## Project Blueprint

The complete repository audit and implementation blueprint is in [PROJECT_BLUEPRINT.md](PROJECT_BLUEPRINT.md). It covers:

- frontend, backend, database, and optional ML service architecture
- local setup and verification commands
- authentication and authorization flows
- API routes and data flows
- database migrations and repository file map
- current limitations, security findings, and recommended next work

## Quick Start

```bash
docker compose up -d --wait db
cd backend && make migrate-up && make run
```

In another terminal:

```bash
cd frontend
npm install
npm run dev
```

Open `http://localhost:3000`. The Go API runs on `http://localhost:8080`.