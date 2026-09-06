# ResQio Project Blueprint

ResQio is a verified disaster resource portal for the `web-01` problem statement: collect emergency information about shelters, food, medicine, volunteers, assistance requests, and blocked roads; show verification state; and help people request and track assistance.

This document describes the implementation that exists in this repository as of 2026-09-06. It is an engineering blueprint and audit record, not a statement that every planned feature is complete.

## 1. Product Scope

### Implemented product areas

- Public landing page with hazard reporting, assistance request, tracking, camps, and feed experiences.
- User registration and phone/password login.
- Provider registration and email/phone/password login.
- JWT-based authentication with user, provider, victim, and admin guards.
- Road hazard reporting with optional image classification, priority scoring, location, clustering, and verification state.
- Assistance request submission with category inference, priority scoring, tracking codes, and status tracking.
- Provider resource inventory with capacity, location, category, and status.
- Provider distribution camps with map-selected coordinates and active/inactive state.
- Community mutual-aid offers and claims.
- Provider dispatch pings, acceptance/rejection, handshake codes, capacity deduction, and timeout cascading.
- Admin overview, audit feed, provider/user controls, hazard verification, cluster rebuild, resource/camp/request views, and manual assignment.
- Optional sentence-embedding and Pinecone integration for semantic matching.
- Optional YOLO image classification for submitted road-hazard images.

### Partially surfaced or backend-only areas

- Disaster reports and their embedding payload are implemented in the Go API and client library, but there is no dedicated visible frontend workflow.
- Mutual-aid offer and claim operations exist in the API client/backend, but the primary portal does not expose the complete community workflow.
- Provider dispatch is polling every four seconds in the dashboard, not SSE or WebSockets.
- Public verification is represented by fields and admin actions; there is no external source registry or evidence-review workflow.
- Footer policy/help links are placeholders.

## 2. System Architecture

```mermaid
flowchart LR
    Browser["Next.js browser :3000"]
    Upload["Next upload route /api/upload"]
    API["Go REST API :8080"]
    DB["PostgreSQL + PostGIS :5433"]
    Embed["Embedding service :8085"]
    YOLO["YOLO service :8086"]
    Pinecone["Optional Pinecone"]
    Files["frontend/public/uploads"]

    Browser -->|JSON + Bearer JWT| API
    Browser -->|multipart image| Upload
    Upload --> Files
    Browser -->|relative image URL| API
    API --> DB
    API -->|embed, match, upsert| Embed
    API -->|classify image| YOLO
    Embed -. optional .-> Pinecone
    YOLO --> Files
```

### Runtime responsibilities

| Process | Location | Default address | Responsibility |
|---|---|---:|---|
| PostgreSQL/PostGIS | `docker-compose.yml` | `localhost:5433` | Persistent relational, geographic, and vector-capable data store |
| Go API | `backend/main.go` | `localhost:8080` | Authentication, domain rules, HTTP routes, dispatch worker, database access |
| Next.js app | `frontend/` | `localhost:3000` | Browser UI, session storage, API client, maps, upload endpoint |
| Embedding service | `backend/ml/service.py` | `localhost:8085` | Text embeddings, semantic matching, optional Pinecone upserts |
| YOLO service | `backend/ml/yolo_service.py` | `localhost:8086` | Image classification for hazard submissions |

The Docker Compose file currently starts only PostgreSQL. The Go, Next.js, embedding, and YOLO processes are started separately.

## 3. Repository Map

### Root and operations

- `Readme.md`: Root entry point; links to this blueprint.
- `PROJECT_BLUEPRINT.md`: This full architecture, setup, API, data-flow, and audit document.
- `docker-compose.yml`: PostGIS database container, exposed on host port `5433`.
- `.env.example`: Root-level environment template if present; do not commit real secrets.
- `.gitignore`: Ignore rules. A tracked environment file still needs to be removed from version control and its credentials rotated.
- `.vscode/settings.json`: Workspace Python analysis settings.
- `image.png`: Project image used by the original root README.

### Backend

- `backend/main.go`: Application composition root. Loads configuration, creates the database pool, initializes SQLC, ML, dispatch, middleware, routes, and graceful shutdown.
- `backend/go.mod`, `backend/go.sum`: Go module and dependency lock data.
- `backend/Makefile`: SQLC generation, migrations, server, YOLO, tests, and build commands.
- `backend/sqlc.yaml`: SQLC source/output configuration and PostGIS/vector type mappings.
- `backend/.env.example`: Backend configuration template.
- `backend/README.md`: Backend integration guide. It contains useful setup/API material but also stale sections that describe already-existing frontend types/functions as future additions.
- `backend/requirements.txt`: Python dependency list; currently declares Ultralytics only.

#### Backend domain packages

- `backend/internal/config/config.go`: Environment loading and defaults.
- `backend/internal/auth/auth.go`: Bcrypt password hashing and HS256 JWT creation/validation.
- `backend/internal/middleware/middleware.go`: Logging, CORS, optional/required auth, account-type guards, and role guards.
- `backend/internal/database/db.go`: SQLC query container and transaction helpers.
- `backend/internal/database/models.go`: Generated database enums and model types.
- `backend/internal/database/*.sql.go`: Generated SQLC implementations for users, providers, hazards, requests, reports, resources, camps, mutual aid, and dispatch.
- `backend/internal/handlers/handlers.go`: Shared handler state, auth endpoints, profile endpoints, health check, DTOs, and response helpers.
- `backend/internal/handlers/road_hazards.go`: Hazard submission, YOLO classification fallback, priority, clustering, listing, and verification-related data.
- `backend/internal/handlers/assistance_requests.go`: Request submission, category/priority logic, nearby-hazard escalation, tracking, and request feeds.
- `backend/internal/handlers/disaster_reports.go`: Disaster report creation and listing.
- `backend/internal/handlers/mutual_aid.go`: Mutual-aid creation, listing, and claiming.
- `backend/internal/handlers/resources.go`: Provider resource CRUD, capacity, embeddings, and semantic upsert.
- `backend/internal/handlers/distribution_camps.go`: Provider camp CRUD and soft deletion.
- `backend/internal/handlers/dispatch.go`: Provider pings, accept/reject, exhausted alerts, and admin overview metrics.
- `backend/internal/handlers/admin.go`: Admin views, controls, verification, audit, cluster rebuild, and manual assignment.
- `backend/internal/dispatch/dispatch.go`: Candidate selection, ping creation, acceptance/rejection, handshake codes, capacity deduction, and provider task counts.
- `backend/internal/dispatch/worker.go`: Five-second monitor for expired dispatch pings.
- `backend/internal/ml/client.go`: Go HTTP client for embedding, matching, upsert, and YOLO calls.

#### Database source

- `backend/db/migrations/00001_create_enums_and_auth_tables.sql`: Enums, users, and providers.
- `00002_create_incident_and_request_tables.sql`: Reports, assistance requests, hazards, mutual aid, and resources.
- `00003_create_dispatch_tables.sql`: Provider dispatch settings, request dispatch metadata, pings, and matches.
- `00004_add_user_location.sql`: User geographic location.
- `00005_add_hazard_ai_metadata.sql`: Hazard image, predicted class, confidence, priority, and cluster fields.
- `00006_create_distribution_camps.sql`: Distribution camps.
- `00007_add_admin_controls.sql`: Active/verified flags and audit logs.
- `backend/db/queries/*.sql`: SQLC query sources for users, providers, requests, reports, hazards, resources, mutual aid, and dispatch.

#### Backend ML assets

- `backend/ml/service.py`: SentenceTransformer embedding API with local vector fallback, vernacular term expansion, `/embed`, `/upsert`, `/match`, and `/health`.
- `backend/ml/yolo_service.py`: YOLO classifier API with `/predict` and `/health`.
- `backend/bestmodel.pt`: YOLO classifier artifact expected by the YOLO service. It is a binary model payload, not source code.

#### Backend tests

- `backend/main_test.go`: Database-backed auth, report, and endpoint integration tests with a separately assembled test router.
- `backend/internal/auth/auth_test.go`: Password and JWT unit tests.
- `backend/internal/middleware/middleware_test.go`: Auth and role middleware tests.
- `backend/internal/dispatch/dispatch_test.go`: Dispatch cascade, acceptance, inventory, exhaustion, and timeout tests.
- `backend/internal/handlers/assistance_priority_test.go`: Assistance priority tests.

### Frontend

- `frontend/app/layout.tsx`: Global metadata, fonts, global stylesheet, and skip link.
- `frontend/app/page.tsx`: Public landing page composition.
- `frontend/app/login/page.tsx`: User/provider login and session persistence.
- `frontend/app/register/user/page.tsx`: User registration and browser geolocation.
- `frontend/app/register/provider/page.tsx`: Provider registration and browser geolocation.
- `frontend/app/provider/page.tsx`: Provider resources, camps, demand map, dispatch polling, and accept/reject UI.
- `frontend/app/admin/page.tsx`: Admin metrics, maps, exports, users, providers, hazards, requests, resources, camps, audit, and cluster rebuild.
- `frontend/app/globals.css`: Tailwind base and portal-specific styling.
- `frontend/components/HeroSection.tsx`: Landing hero and illustration.
- `frontend/components/ResponsePortal.tsx`: Public hazard, assistance, tracking, camp, and feed workflows.
- `frontend/components/Navbar.tsx`: Session-aware navigation, sign out, and text-size controls.
- `frontend/components/RegistrationModal.tsx`: User/provider registration choice.
- `frontend/components/ProviderRequestMap.tsx`: Leaflet provider demand map.
- `frontend/components/AdminHazardMap.tsx`: Leaflet admin hazard/cluster map.
- `frontend/components/CampLocationPicker.tsx`: Leaflet camp coordinate picker.
- `frontend/components/PageHeader.tsx`: Internal page header and breadcrumb.
- `frontend/components/Footer.tsx`: Static footer; several links are placeholders.
- `frontend/components/ui/FormField.tsx`: Shared form field wrappers.
- `frontend/lib/api.ts`: Fetch wrappers, local-storage sessions, auth, hazard, request, report, mutual-aid, resource, camp, dispatch, admin, and upload client functions.
- `frontend/types/index.ts`: Shared TypeScript request/response interfaces and enums.
- `frontend/pages/api/upload.ts`: Multer-based Next API route that writes uploaded files to `frontend/public/uploads`.
- `frontend/package.json`, `package-lock.json`: Next.js scripts and locked dependencies.
- `frontend/next.config.mjs`, `tsconfig.json`, `tailwind.config.ts`, `postcss.config.mjs`: Framework and build configuration.
- `frontend/next-env.d.ts`: Generated Next.js type declarations.
- `frontend/images/Hero-illustration.png`: Hero asset.
- `frontend/public/uploads/`: Uploaded image storage used by the local upload flow.

## 4. Authentication and Authorization

1. Users register with phone/password and a user role; providers register with organization and contact data.
2. Passwords are bcrypt-hashed before database storage.
3. The Go API signs HS256 JWTs containing account identity, account type, role, and expiry.
4. The frontend stores the token, account type, and profile in `localStorage`.
5. Protected requests send `Authorization: Bearer <token>`.
6. The backend is the actual authorization boundary. User routes, provider routes, victim request submission, and admin routes use separate middleware guards.
7. Admin access requires a user JWT with the `ADMIN` role.
8. A newly registered provider can log in, but dispatch candidate queries require an active and verified provider. An administrator must verify the provider before automated dispatch can reach it.
9. After a role change, the current JWT remains stale until the user signs in again.

Security characteristics that should be treated as current limitations:

- JWTs are bearer tokens stored in browser local storage; there are no refresh tokens or server sessions.
- CORS currently permits every origin.
- Public request list/detail/tracking responses include requester contact fields.
- Uploads are unauthenticated and validate size but not the actual file content.
- `backend/.env` is tracked in the repository and must be treated as exposed secret material; rotate credentials before any public release.

## 5. Main Data Flows

### Hazard report

1. The browser optionally uploads a photo to Next `/api/upload`.
2. Multer stores the file under `frontend/public/uploads` and returns a relative URL.
3. The browser submits hazard details and coordinates to Go `POST /api/hazards`.
4. Go calls YOLO when an image is available. If YOLO is unavailable, it falls back to the submitted/default hazard type.
5. Hazard priority is calculated from the submitted data and model metadata.
6. Same-class hazards within 100 meters can be associated into a cluster.
7. The report is stored with verification, priority, AI metadata, location, and timestamps.

### Assistance request and dispatch

1. An authenticated user with the `VICTIM` role submits a request.
2. The API requires a location, infers a resource category where needed, and calculates priority from urgency, vulnerability, category, and nearby hazards.
3. The request may receive an embedding from the embedding service and is stored with a public tracking code.
4. An asynchronous dispatch process selects active, verified providers with matching capacity, ordered by geographic distance and optionally semantic similarity.
5. A provider receives a pending ping for up to three minutes. The dashboard polls for active pings.
6. Acceptance transactionally creates a match and handshake code, marks the request matched, deducts capacity, and increments provider task state.
7. Rejection or expiry cascades to the next candidate. If no candidate remains, the request becomes exhausted and appears in admin alerts.

### Provider resources and camps

Providers publish resource capacity and optional coordinates. Resource creation can produce an embedding and call the embedding service `/upsert`. Providers can update/delete their own resources. Camps are created from map coordinates and are soft-deleted by setting `is_active = false`; public users can list active camps.

### Community mutual aid

Authenticated users can offer an item, public users can list available items, and authenticated users can claim an available item. A personal feed is available at `/api/me/mutual-aid`.

## 6. HTTP API Surface

All API routes below use the Go server and the `/api` prefix. Protected routes require a Bearer JWT as indicated.

### Health and authentication

| Method | Path | Access | Purpose |
|---|---|---|---|
| GET | `/healthz` | Public | Health check |
| POST | `/api/auth/users/register` | Public | Register user |
| POST | `/api/auth/users/login` | Public | Log in user |
| GET | `/api/auth/users/me` | User JWT | Current user profile |
| POST | `/api/auth/providers/register` | Public | Register provider |
| POST | `/api/auth/providers/login` | Public | Log in provider |
| GET | `/api/auth/providers/me` | Provider JWT | Current provider profile |

### Public reports and requests

| Method | Path | Access | Purpose |
|---|---|---|---|
| POST | `/api/hazards` | Optional JWT | Submit road hazard |
| GET | `/api/hazards` | Public | Paginated hazard feed |
| GET | `/api/hazards/{id}` | Public | Hazard detail |
| GET | `/api/me/hazards` | User JWT | Current user's hazards |
| POST | `/api/disaster-reports` | Optional JWT | Create disaster report |
| GET | `/api/disaster-reports` | Public | Disaster report feed |
| GET | `/api/disaster-reports/{id}` | Public | Disaster report detail |
| POST | `/api/requests` | Victim user JWT | Create assistance request |
| GET | `/api/requests` | Public | Request feed |
| GET | `/api/requests/{id}` | Public | Request detail |
| GET | `/api/requests/track/{code}` | Public | Track by request code |
| GET | `/api/me/requests` | User JWT | Current user's requests |

### Resources, camps, and mutual aid

| Method | Path | Access | Purpose |
|---|---|---|---|
| GET | `/api/resources` | Public | Resource feed/filter |
| GET | `/api/resources/{id}` | Public | Resource detail |
| POST | `/api/resources` | Provider JWT | Create resource |
| PUT | `/api/resources/{id}` | Provider JWT | Update own resource |
| DELETE | `/api/resources/{id}` | Provider JWT | Delete own resource |
| GET | `/api/provider/my-resources` | Provider JWT | Own resource inventory |
| GET | `/api/distribution-camps` | Public | Active camp list |
| POST | `/api/provider/distribution-camps` | Provider JWT | Create camp |
| PUT | `/api/provider/distribution-camps/{id}` | Provider JWT | Update own camp |
| DELETE | `/api/provider/distribution-camps/{id}` | Provider JWT | Soft-delete own camp |
| GET | `/api/mutual-aid` | Public | Mutual-aid feed |
| POST | `/api/mutual-aid` | User JWT | Offer mutual aid |
| POST | `/api/mutual-aid/{id}/claim` | User JWT | Claim mutual aid |
| GET | `/api/me/mutual-aid` | User JWT | Own mutual-aid items |

### Provider dispatch

| Method | Path | Access | Purpose |
|---|---|---|---|
| GET | `/api/provider/requests` | Provider JWT | Provider demand feed |
| GET | `/api/provider/pings/active` | Provider JWT | Current pending dispatch ping |
| POST | `/api/provider/pings/{id}/accept` | Provider JWT | Accept a ping |
| POST | `/api/provider/pings/{id}/reject` | Provider JWT | Reject a ping |

### Admin

All admin routes require a user JWT with role `ADMIN`.

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/admin/overview` | Dashboard metrics |
| GET | `/api/admin/alerts` | Exhausted request alerts |
| GET | `/api/admin/users` | User administration list |
| PUT | `/api/admin/users/{id}/role` | Change user role |
| GET | `/api/admin/providers` | Provider administration list |
| PUT | `/api/admin/providers/{id}/status` | Change active/verified provider state |
| GET | `/api/admin/audit` | Audit log |
| GET | `/api/admin/hazards` | Hazard moderation list |
| PUT | `/api/admin/hazards/{id}/verify` | Verify hazard |
| POST | `/api/admin/ai/rebuild-hazard-clusters` | Rebuild same-class geographic clusters |
| GET | `/api/admin/requests` | Request administration list |
| POST | `/api/admin/requests/{id}/assign` | Manually assign a request |
| GET | `/api/admin/resources` | Resource administration list |
| GET | `/api/admin/camps` | Camp administration list |

## 7. Database Model

The migrations build these main areas:

- Identity: `users`, `providers`, user/provider enums, active flags, provider verification.
- Incidents: `road_hazards`, `disaster_reports`, hazard verification and AI metadata.
- Assistance: `assistance_requests`, priority/status/dispatch state, tracking code, coordinates, requester data.
- Supply: `resources`, categories, capacity/status, provider ownership, optional embedding.
- Community: `mutual_aid_items`, claim ownership and availability.
- Dispatch: provider settings, candidate pings, matches, handshake codes, expiry and exhaustion state.
- Distribution: provider-owned camps with coordinates and active state.
- Administration: audit logs and administrative state changes.

The schema expects PostGIS functions such as `ST_GeomFromText`, `ST_DWithin`, `ST_Distance`, and `ST_AsText`. The migrations include text-domain fallbacks when extensions are unavailable, but application queries still require PostGIS behavior; the supported local setup is therefore the PostGIS Docker image. The project also attempts to use pgvector where available for embeddings.

## 8. Local Setup

### Prerequisites

- Docker and Docker Compose.
- Go version compatible with `backend/go.mod`.
- Node.js/npm compatible with Next.js 14.
- Python for the optional ML services.
- PostgreSQL client tools if manually promoting an admin or inspecting data.

### 1. Start the database

From the repository root:

```bash
docker compose up -d --wait db
```

### 2. Apply migrations and start the Go API

```bash
cd backend
make migrate-up
make run
```

The API should be available at `http://localhost:8080`; check `http://localhost:8080/healthz`.

The Makefile uses:

```text
postgres://resqio_user:resqio_password@localhost:5433/auth_db?sslmode=disable
```

Set `DATABASE_URL` explicitly when using another database.

### 3. Start the optional embedding service

The Go client defaults to `http://localhost:8085` when no embedding URL is configured, but the service is not started by Compose or the Makefile.

```bash
cd backend
python -m pip install -r requirements.txt
python ml/service.py
```

`requirements.txt` currently declares only Ultralytics even though `service.py` imports sentence-transformers and can use Pinecone. Install the service's actual Python dependencies in the environment used for it before relying on semantic matching.

### 4. Start the optional YOLO service

```bash
cd backend
make yolo-install
make yolo-run
```

The classifier listens on `http://localhost:8086`. Hazard submissions continue without it, using the submitted/default hazard type as a fallback.

### 5. Configure and start the frontend

Create `frontend/.env.local`:

```env
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080/api
```

Then:

```bash
cd frontend
npm install
npm run dev
```

Open `http://localhost:3000`.

### Admin bootstrap

Public registration does not create an admin. Promote an existing user directly in PostgreSQL, then sign in again to obtain a JWT containing the new role:

```sql
UPDATE users SET role = 'ADMIN' WHERE phone = '<admin-phone-number>';
```

## 9. Configuration Reference

### Go API

- `PORT`: HTTP port; default `8080`.
- `DATABASE_URL`: PostgreSQL connection string. Docker uses host port `5433`.
- `JWT_SECRET`: HS256 signing secret; use a strong secret outside development.
- `JWT_EXPIRATION`: Go duration such as `24h`.
- `EMBEDDING_SERVICE_URL`: Embedding service base URL; code defaults to `http://localhost:8085`.
- `YOLO_SERVICE_URL`: YOLO base URL; example uses `http://localhost:8086`.

### ML services

- `EMBEDDING_MODEL`: SentenceTransformer model selection.
- `EMBEDDING_PORT`: Embedding service port.
- `PINECONE_API_KEY`, `PINECONE_INDEX_NAME`: Optional remote vector store.
- `YOLO_MODEL_PATH`: Optional classifier model path; defaults to `backend/bestmodel.pt`.
- `YOLO_PORT`: YOLO service port.
- `UPLOADS_DIR`: Directory shared with frontend-uploaded files.

### Frontend

- `NEXT_PUBLIC_API_BASE_URL`: Go API base URL including `/api`.

Do not commit real `.env` files, database passwords, JWT secrets, or API keys. The currently tracked backend environment file should be rotated and removed from version control before publication.

## 10. Verification Commands

Run backend commands from `backend/`:

```bash
go test ./...
go vet ./...
make build
```

Run frontend commands from `frontend/`:

```bash
npm run lint
npm run build
```

There are currently no frontend unit tests, browser end-to-end tests, ML service tests, migration tests, or generated API contract tests.

## 11. Current Findings and Technical Debt

### High priority

- Rotate and remove tracked secrets from `backend/.env`.
- Restrict public assistance responses so names and phone numbers are not exposed unnecessarily.
- Define and enforce the provider verification lifecycle in the UI and API documentation.
- Add the missing embedding-service dependencies and a supported startup command.

### Medium priority

- Align `config.go` database defaults with Docker/Makefile defaults.
- Share production route construction with integration tests so the test router cannot drift.
- Replace or formally constrain the non-PostGIS migration fallback.
- Make dispatch candidate creation and request transitions enforce their concurrency invariants in the database.
- Abort or compensate the acceptance transaction when capacity/accounting updates fail.
- Restrict CORS to configured frontend origins.
- Authenticate and validate uploads; plan durable object storage and cleanup.

### Product completeness

- Add a visible disaster-report workflow.
- Add complete mutual-aid offer/claim screens.
- Replace placeholder footer links with real policy, help, and verification-source pages.
- Add status history and evidence provenance if public verification is a core product promise.

### Test coverage

- Add frontend component and API-client tests.
- Add browser tests covering registration, reporting, requesting, tracking, provider acceptance, and admin moderation.
- Add ML health/fallback tests and migration smoke tests.
- Add API contract tests shared by Go handlers and TypeScript types.

### Operations

- Decide whether the committed YOLO model and uploaded files belong in Git or in artifact/object storage.
- Add health checks and structured logs for all services.
- Add a production deployment definition for the frontend, Go API, database, and optional ML services.
- Document backup, restore, retention, and personal-data deletion procedures.

## 12. Definition of Done for the Portal

A production-ready version should be able to demonstrate all of the following:

1. A person can report a blocked road and see whether it is unverified, verified, clustered, and prioritized.
2. A victim can submit an assistance request, receive a tracking code, and see status changes without exposing private contact data publicly.
3. A verified provider can publish capacity, receive a matching request, accept it, and receive a handshake code while inventory remains consistent.
4. An administrator can verify evidence, activate providers, monitor exhausted requests, and review an audit trail.
5. The database, frontend, API, uploads, and optional ML services can be started from documented commands with health checks.
6. Automated tests cover the critical cross-service workflows, not only isolated backend logic.
