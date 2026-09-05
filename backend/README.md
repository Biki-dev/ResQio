# ResQio Backend - Frontend Integration Guide

This guide provides end-to-end instructions for connecting the **Go REST Backend** (`backend/`) with the **Next.js 14 Frontend** (`frontend/`).

---

## 1. Quick Start & Local Setup

### Step 1: Start the Backend Server
```bash
cd backend

# 1. Run migrations to initialize PostgreSQL tables & PostGIS/pgvector extensions
make migrate-up

# 2. Start the Go HTTP server (runs on port 8080 by default)
make run
```
*Health Check*: Verify server is running by opening `http://localhost:8080/healthz` in your browser.

### Step 2: Configure Frontend Environment
Create or verify `frontend/.env.local`:
```env
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080/api
```

### Step 3: Run the Frontend
```bash
cd frontend
npm install
npm run dev
```
The frontend is available at `http://localhost:3000`.

---

## 2. API Endpoints Reference

All endpoints are served under `http://localhost:8080/api`:

### A. Main Page Forms (Wireframe)
| Form / Feature | HTTP Method | Endpoint | Auth | Description |
|---|---|---|---|---|
| **Submit Issue** (Left Form) | `POST` | `/hazards` | Optional Bearer JWT | Submits road hazard / incident with coordinates |
| **Previous Issue Submissions** | `GET` | `/hazards` | Public | Paginated list of submitted hazards (`?limit=10&offset=0`) |
| **View Issue Details** | `GET` | `/hazards/:id` | Public | Fetches single hazard by UUID |
| **Submit Need / Call** (Right Form) | `POST` | `/requests` | Bearer JWT (`VICTIM` role) | Submits assistance request; returns `tracking_code` |
| **Previous Calls Feed** | `GET` | `/requests` | Public | Paginated list of assistance requests (`?limit=10&offset=0`) |
| **View Call Details** | `GET` | `/requests/:id` | Public | Fetches single assistance request by UUID |
| **Track Request** | `GET` | `/requests/track/:code` | Public | Public tracking using `REQ-XXXXXXXX` code |

### B. Disaster Reporting Portal
| Feature | HTTP Method | Endpoint | Auth | Description |
|---|---|---|---|---|
| **Submit Disaster Report** | `POST` | `/disaster-reports` | Optional Bearer JWT | Submits 1536d image embedding + coordinates |
| **List Disaster Reports** | `GET` | `/disaster-reports` | Public | Paginated disaster reports feed |
| **View Disaster Report** | `GET` | `/disaster-reports/:id` | Public | Fetches single disaster report by UUID |

### C. Community Aid & Provider Inventory
| Feature | HTTP Method | Endpoint | Auth | Description |
|---|---|---|---|---|
| **List Mutual Aid Items** | `GET` | `/mutual-aid` | Public | Community item feed (`?available=true`) |
| **Offer Mutual Aid** | `POST` | `/mutual-aid` | Bearer JWT (User) | Post emergency item/supply |
| **Claim Mutual Aid** | `POST` | `/mutual-aid/:id/claim` | Bearer JWT (User) | Claim an available mutual aid item |
| **List Provider Resources** | `GET` | `/resources` | Public | NGO/Agency supply feed (`?category=WATER`) |
| **Publish Resource** | `POST` | `/resources` | Bearer JWT (Provider) | Create resource with total/current capacity |

### D. Authentication
| Feature | HTTP Method | Endpoint | Description |
|---|---|---|---|
| **Register User** | `POST` | `/auth/users/register` | Register citizen/victim/volunteer |
| **Login User** | `POST` | `/auth/users/login` | Phone + password login |
| **Get User Profile** | `GET` | `/auth/users/me` | Current authenticated user profile |
| **Register Provider** | `POST` | `/auth/providers/register` | Register NGO/relief organization |
| **Login Provider** | `POST` | `/auth/providers/login` | Email/phone + password login |
| **Get Provider Profile** | `GET` | `/auth/providers/me` | Current authenticated provider profile |

---

## 3. TypeScript Interfaces to Add to `frontend/types/index.ts`

Append these types to [`frontend/types/index.ts`](file:///Users/knibirdgautam/Documents/CS_Coding_Projects/ResQio/frontend/types/index.ts):

```typescript
// ==================== ENUMS ====================

export enum ResourceCategory {
  FOOD = "FOOD",
  WATER = "WATER",
  MEDICINE = "MEDICINE",
  SHELTER = "SHELTER",
  EQUIPMENT = "EQUIPMENT",
  VOLUNTEER = "VOLUNTEER",
  OTHER = "OTHER",
}

export enum RequestPriority {
  LOW = "LOW",
  MEDIUM = "MEDIUM",
  HIGH = "HIGH",
  CRITICAL = "CRITICAL",
}

export enum RequestStatus {
  SUBMITTED = "SUBMITTED",
  ACKNOWLEDGED = "ACKNOWLEDGED",
  IN_PROGRESS = "IN_PROGRESS",
  FULFILLED = "FULFILLED",
  CANCELLED = "CANCELLED",
}

// ==================== ROAD HAZARD (ISSUE FORM) ====================

export interface SubmitRoadHazardRequest {
  name: string;
  phone_number: string;
  hazard_type?: string;
  severity?: string;
  description?: string;
  photo_url?: string;
  location?: string;
  latitude?: number;
  longitude?: number;
}

export interface RoadHazardResponse {
  id: string;
  reporter_id?: string;
  name: string;
  phone_number: string;
  hazard_type: string;
  severity: string;
  location: string;
  description: string;
  is_verified: boolean;
  created_at: string;
}

// ==================== ASSISTANCE REQUEST (NEED FORM) ====================

export interface SubmitAssistanceRequest {
  name: string;
  identity?: string;
  phone_number: string;
  things_needed: string;
  category?: ResourceCategory | string;
  amount: number;
  description?: string;
  photo_url?: string;
  priority?: RequestPriority | string;
  location?: string;
  latitude?: number;
  longitude?: number;
  address_text?: string;
}

export interface AssistanceRequestResponse {
  id: string;
  requester_id?: string;
  tracking_code: string;
  category: ResourceCategory | string;
  quantity_needed: number;
  description: string;
  priority: RequestPriority | string;
  status: RequestStatus | string;
  assigned_coordinator_id?: string;
  location: string;
  address_text: string;
  requester_name: string;
  contact_phone: string;
  created_at: string;
  updated_at: string;
}

// ==================== DISASTER REPORT (IMAGE EMBEDDING) ====================

export interface SubmitDisasterReportRequest {
  reporter_id?: string;
  embedding?: number[];
  vector_str?: string;
  location?: string;
  latitude?: number;
  longitude?: number;
}

export interface DisasterReportResponse {
  id: string;
  reporter_id: string;
  location: string;
  created_at: string;
}
```

---

## 4. API Client Functions to Add to `frontend/lib/api.ts`

Add these functions into [`frontend/lib/api.ts`](file:///Users/knibirdgautam/Documents/CS_Coding_Projects/ResQio/frontend/lib/api.ts):

```typescript
// Helper to include JWT token if the user is currently logged in
function getAuthHeader(): Record<string, string> {
  const session = getSession();
  return session?.token ? { Authorization: `Bearer ${session.token}` } : {};
}

// ==================== ISSUE / ROAD HAZARD APIS ====================

export async function submitRoadHazard(payload: SubmitRoadHazardRequest) {
  const session = getSession();
  return post<RoadHazardResponse, SubmitRoadHazardRequest>("/hazards", payload);
}

export async function getRoadHazards(limit = 20, offset = 0) {
  return fetch(`${API_BASE_URL}/hazards?limit=${limit}&offset=${offset}`)
    .then((res) => res.json())
    .catch((err) => ({ error: err.message }));
}

export async function getRoadHazardById(id: string) {
  return fetch(`${API_BASE_URL}/hazards/${id}`)
    .then((res) => res.json())
    .catch((err) => ({ error: err.message }));
}

// ==================== ASSISTANCE REQUEST / CALLS APIS ====================

export async function submitAssistanceRequest(payload: SubmitAssistanceRequest) {
  return post<AssistanceRequestResponse, SubmitAssistanceRequest>("/requests", payload);
}

export async function getAssistanceRequests(limit = 20, offset = 0) {
  return fetch(`${API_BASE_URL}/requests?limit=${limit}&offset=${offset}`)
    .then((res) => res.json())
    .catch((err) => ({ error: err.message }));
}

export async function getAssistanceRequestById(id: string) {
  return fetch(`${API_BASE_URL}/requests/${id}`)
    .then((res) => res.json())
    .catch((err) => ({ error: err.message }));
}

export async function trackAssistanceRequest(code: string) {
  return fetch(`${API_BASE_URL}/requests/track/${encodeURIComponent(code)}`)
    .then((res) => res.json())
    .catch((err) => ({ error: err.message }));
}

// ==================== DISASTER REPORTING APIS ====================

export async function submitDisasterReport(payload: SubmitDisasterReportRequest) {
  return post<DisasterReportResponse, SubmitDisasterReportRequest>("/disaster-reports", payload);
}

export async function getDisasterReports(limit = 20, offset = 0) {
  return fetch(`${API_BASE_URL}/disaster-reports?limit=${limit}&offset=${offset}`)
    .then((res) => res.json())
    .catch((err) => ({ error: err.message }));
}
```

---

## 5. Main Page Form Implementation Guide

In your React/Next.js page (e.g. `frontend/app/portal/page.tsx` or `frontend/app/page.tsx`), implement the two forms and two feed lists:

### Getting Device Geolocation ("Location - permission" button):
```typescript
const requestLocation = (): Promise<{ latitude: number; longitude: number }> => {
  return new Promise((resolve, reject) => {
    if (!navigator.geolocation) {
      reject(new Error("Geolocation is not supported by your browser"));
      return;
    }
    navigator.geolocation.getCurrentPosition(
      (position) => {
        resolve({
          latitude: position.coords.latitude,
          longitude: position.coords.longitude,
        });
      },
      (error) => reject(error),
      { enableHighAccuracy: true, timeout: 10000 }
    );
  });
};
```

### Left Form: Submit Issue (Road Hazard)
```typescript
const handleIssueSubmit = async (e: React.FormEvent) => {
  e.preventDefault();
  const coords = await requestLocation();
  const res = await submitRoadHazard({
    name: issueName,
    phone_number: issuePhone,
    description: issueDescription,
    hazard_type: "ROAD_INCIDENT",
    severity: "MEDIUM",
    latitude: coords.latitude,
    longitude: coords.longitude,
  });

  if (res.success) {
    alert("Issue submitted successfully!");
    refreshIssuesFeed();
  } else {
    alert(`Submission failed: ${res.error}`);
  }
};
```

### Right Form: Ask for something they need (Assistance Request)
```typescript
const handleNeedSubmit = async (e: React.FormEvent) => {
  e.preventDefault();
  const coords = await requestLocation();
  const res = await submitAssistanceRequest({
    name: needName,
    identity: needIdentity,
    phone_number: needPhone,
    things_needed: needThings,
    amount: Number(needAmount),
    description: needDescription,
    latitude: coords.latitude,
    longitude: coords.longitude,
  });

  if (res.success) {
    alert(`Assistance request registered! Your Tracking Code is: ${res.data.tracking_code}`);
    refreshCallsFeed();
  } else {
    alert(`Submission failed: ${res.error}`);
  }
};
```

---

## 6. Verification Checklist

1. **CORS Headers**: Already configured in [backend/internal/middleware/middleware.go](file:///Users/knibirdgautam/Documents/CS_Coding_Projects/ResQio/backend/internal/middleware/middleware.go) to allow all origins and headers (`*`).
2. **Anonymous vs Authenticated**:
   - Both forms work without logging in (ideal for disaster victims).
   - If a token exists in `localStorage`, the backend automatically links the submission to the logged-in `reporter_id` / `requester_id`.
3. **Database Constraints**: All coordinates are persisted as PostGIS geometries (`POINT(longitude latitude)`).
