import type {
  ActivePing,
  AdminOverview,
  AdminAuditLog,
  AdminProvider,
  AdminUser,
  AuthResponse,
  AssistanceRequestResponse,
  DisasterReportResponse,
  DistributionCamp,
  DistributionCampRequest,
  ExhaustedAlert,
  LoginProviderRequest,
  LoginUserRequest,
  MatchResponse,
  MutualAidItemRequest,
  MutualAidItemResponse,
  Provider,
  ProviderAssistanceRequest,
  RegisterProviderRequest,
  RegisterUserRequest,
  ResourceRequest,
  ResourceResponse,
  RoadHazardResponse,
  SubmitAssistanceRequest,
  SubmitDisasterReportRequest,
  SubmitRoadHazardRequest,
  User,
} from "@/types";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080/api";

type ApiResult<T> =
  | { success: true; data: T }
  | { success: false; error: string };

function getAuthHeader(): Record<string, string> {
  const session = getSession();
  return session?.token ? { Authorization: `Bearer ${session.token}` } : {};
}

async function request<TResponse, TBody = never>(
  path: string,
  options: { method?: string; body?: TBody; auth?: boolean } = {}
): Promise<ApiResult<TResponse>> {
  try {
    const headers: Record<string, string> = { ...getAuthHeader() };
    if (options.body !== undefined) headers["Content-Type"] = "application/json";

    const res = await fetch(`${API_BASE_URL}${path}`, {
      method: options.method ?? "GET",
      headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
    });
    const raw = await res.text();
    let json: unknown = null;
    try {
      json = raw ? JSON.parse(raw) : null;
    } catch {
      json = null;
    }

    if (!res.ok) {
      const message =
        typeof json === "object" && json !== null && "error" in json
          ? String(json.error)
          : `Request failed with status ${res.status}`;
      return { success: false, error: message };
    }

    return { success: true, data: json as TResponse };
  } catch (err) {
    return {
      success: false,
      error: err instanceof Error ? err.message : "Network error",
    };
  }
}

async function post<TResponse, TBody>(
  path: string,
  body: TBody
): Promise<{ success: true; data: TResponse } | { success: false; error: string }> {
  try {
    const res = await fetch(`${API_BASE_URL}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    const json = await res.json().catch(() => null);

    if (!res.ok) {
      return {
        success: false,
        error: json?.error ?? `Request failed with status ${res.status}`,
      };
    }

    return { success: true, data: json as TResponse };
  } catch (err) {
    return {
      success: false,
      error: err instanceof Error ? err.message : "Network error",
    };
  }
}

async function get<TResponse>(
  path: string,
  token: string
): Promise<{ success: true; data: TResponse } | { success: false; error: string }> {
  try {
    const res = await fetch(`${API_BASE_URL}${path}`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    const json = await res.json().catch(() => null);

    if (!res.ok) {
      return {
        success: false,
        error: json?.error ?? `Request failed with status ${res.status}`,
      };
    }

    return { success: true, data: json as TResponse };
  } catch (err) {
    return {
      success: false,
      error: err instanceof Error ? err.message : "Network error",
    };
  }
}

export interface SessionData {
  token: string;
  accountType: "user" | "provider";
  role?: string;
  phone?: string;
  fullName?: string;
  accountId?: string;
}

export function parseJwtPayload(token: string): Record<string, unknown> | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const base64Url = parts[1];
    const base64 = base64Url.replace(/-/g, "+").replace(/_/g, "/");
    const jsonPayload = decodeURIComponent(
      atob(base64)
        .split("")
        .map((c) => "%" + ("00" + c.charCodeAt(0).toString(16)).slice(-2))
        .join("")
    );
    return JSON.parse(jsonPayload);
  } catch {
    return null;
  }
}

export function saveSession(response: AuthResponse<unknown>) {
  if (typeof window !== "undefined") {
    localStorage.setItem("resqio_token", response.token);
    localStorage.setItem("resqio_account_type", response.account_type);
    if (response.profile) {
      localStorage.setItem("resqio_profile", JSON.stringify(response.profile));
    }
  }
}

export function clearSession() {
  if (typeof window !== "undefined") {
    localStorage.removeItem("resqio_token");
    localStorage.removeItem("resqio_account_type");
    localStorage.removeItem("resqio_profile");
  }
}

export function getSession(): SessionData | null {
  if (typeof window === "undefined") return null;
  const token = localStorage.getItem("resqio_token");
  const accountType = localStorage.getItem("resqio_account_type") as
    | "user"
    | "provider"
    | null;
  if (!token || !accountType) return null;

  let profile: Record<string, unknown> | null = null;
  try {
    const raw = localStorage.getItem("resqio_profile");
    if (raw) profile = JSON.parse(raw);
  } catch {
    profile = null;
  }

  const claims = parseJwtPayload(token);
  const role = (profile?.role || claims?.role || "") as string;
  const phone = (profile?.phone || profile?.ph_no || claims?.phone || "") as string;
  const fullName = (profile?.full_name || profile?.name || "") as string;
  const accountId = (profile?.id || claims?.account_id || claims?.sub || "") as string;

  return {
    token,
    accountType,
    role,
    phone,
    fullName,
    accountId,
  };
}


export function registerUser(payload: RegisterUserRequest) {
  return post<AuthResponse<User>, RegisterUserRequest>(
    "/auth/users/register",
    payload
  ).then((result) => {
    if (result.success) saveSession(result.data);
    return result;
  });
}

export function registerProvider(payload: RegisterProviderRequest) {
  return post<AuthResponse<Provider>, RegisterProviderRequest>(
    "/auth/providers/register",
    payload
  ).then((result) => {
    if (result.success) saveSession(result.data);
    return result;
  });
}

export function loginUser(payload: LoginUserRequest) {
  return post<AuthResponse<User>, LoginUserRequest>(
    "/auth/users/login",
    payload
  ).then((result) => {
    if (result.success) saveSession(result.data);
    return result;
  });
}

export function loginProvider(payload: LoginProviderRequest) {
  return post<AuthResponse<Provider>, LoginProviderRequest>(
    "/auth/providers/login",
    payload
  ).then((result) => {
    if (result.success) saveSession(result.data);
    return result;
  });
}

export function getUserMe(token: string) {
  return get<User>("/auth/users/me", token);
}

export function getProviderMe(token: string) {
  return get<Provider>("/auth/providers/me", token);
}

export function submitRoadHazard(payload: SubmitRoadHazardRequest) {
  return request<RoadHazardResponse, SubmitRoadHazardRequest>("/hazards", {
    method: "POST",
    body: payload,
  });
}

export function getRoadHazards(limit = 20, offset = 0) {
  return request<RoadHazardResponse[]>(`/hazards?limit=${limit}&offset=${offset}`);
}

export function getMyRoadHazards(limit = 20, offset = 0) {
  return request<RoadHazardResponse[]>(`/me/hazards?limit=${limit}&offset=${offset}`);
}

export function getRoadHazardById(id: string) {
  return request<RoadHazardResponse>(`/hazards/${encodeURIComponent(id)}`);
}

export function submitAssistanceRequest(payload: SubmitAssistanceRequest) {
  return request<AssistanceRequestResponse, SubmitAssistanceRequest>("/requests", {
    method: "POST",
    body: payload,
  });
}

export function getAssistanceRequests(limit = 20, offset = 0) {
  return request<AssistanceRequestResponse[]>(`/requests?limit=${limit}&offset=${offset}`);
}

export function getMyAssistanceRequests(limit = 20, offset = 0) {
  return request<AssistanceRequestResponse[]>(`/me/requests?limit=${limit}&offset=${offset}`);
}

export function getMyMutualAidItems(limit = 20, offset = 0) {
  return request<MutualAidItemResponse[]>(`/me/mutual-aid?limit=${limit}&offset=${offset}`);
}

export function getMyProviderResources() {
  return request<ResourceResponse[]>("/provider/my-resources");
}

export function getAssistanceRequestById(id: string) {
  return request<AssistanceRequestResponse>(`/requests/${encodeURIComponent(id)}`);
}

export function trackAssistanceRequest(code: string) {
  return request<AssistanceRequestResponse>(`/requests/track/${encodeURIComponent(code)}`);
}

export function submitDisasterReport(payload: SubmitDisasterReportRequest) {
  return request<DisasterReportResponse, SubmitDisasterReportRequest>("/disaster-reports", {
    method: "POST",
    body: payload,
  });
}

export function getDisasterReports(limit = 20, offset = 0) {
  return request<DisasterReportResponse[]>(
    `/disaster-reports?limit=${limit}&offset=${offset}`
  );
}

export function getDisasterReportById(id: string) {
  return request<DisasterReportResponse>(`/disaster-reports/${encodeURIComponent(id)}`);
}

export function getMutualAidItems(limit = 20, offset = 0, available = true) {
  return request<MutualAidItemResponse[]>(
    `/mutual-aid?limit=${limit}&offset=${offset}&available=${available}`
  );
}

export function offerMutualAid(payload: MutualAidItemRequest) {
  return request<MutualAidItemResponse, MutualAidItemRequest>("/mutual-aid", {
    method: "POST",
    body: payload,
  });
}

export function claimMutualAid(id: string) {
  return request<MutualAidItemResponse>(`/mutual-aid/${encodeURIComponent(id)}/claim`, {
    method: "POST",
  });
}

export function getResources(limit = 20, offset = 0, category?: string) {
  const categoryParam = category ? `&category=${encodeURIComponent(category)}` : "";
  return request<ResourceResponse[]>(`/resources?limit=${limit}&offset=${offset}${categoryParam}`);
}

export function getDistributionCamps(limit = 100) {
  return request<DistributionCamp[]>(`/distribution-camps?limit=${limit}`);
}

export function createDistributionCamp(payload: DistributionCampRequest) {
  return request<DistributionCamp, DistributionCampRequest>("/provider/distribution-camps", {
    method: "POST",
    body: payload,
  });
}

export function updateDistributionCamp(id: string, payload: DistributionCampRequest) {
  return request<DistributionCamp, DistributionCampRequest>(`/provider/distribution-camps/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: payload,
  });
}

export function deleteDistributionCamp(id: string) {
  return request<{ message: string }>(`/provider/distribution-camps/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

export function getProviderResources(providerId: string, limit = 50, offset = 0) {
  return request<ResourceResponse[]>(
    `/resources?provider_id=${encodeURIComponent(providerId)}&limit=${limit}&offset=${offset}`
  );
}

export function publishResource(payload: ResourceRequest) {
  return request<ResourceResponse, ResourceRequest>("/resources", {
    method: "POST",
    body: payload,
  });
}

export function createResource(payload: ResourceRequest) {
  return publishResource(payload);
}

export function updateResource(id: string, payload: Partial<ResourceRequest>) {
  return request<ResourceResponse, Partial<ResourceRequest>>(`/resources/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: payload,
  });
}

export function deleteResource(id: string) {
  return request<{ message: string }>(`/resources/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

export async function uploadPhotoWithMulter(file: File): Promise<ApiResult<{ url: string }>> {
  try {
    const formData = new FormData();
    formData.append("photo", file);

    const res = await fetch("/api/upload", {
      method: "POST",
      body: formData,
    });

    const json = await res.json().catch(() => null);

    if (!res.ok) {
      return {
        success: false,
        error: json?.error ?? `Upload failed with status ${res.status}`,
      };
    }

    return {
      success: true,
      data: { url: json.url },
    };
  } catch (err) {
    return {
      success: false,
      error: err instanceof Error ? err.message : "File upload failed",
    };
  }
}

export function getActiveProviderPing() {
  return request<{ ping: ActivePing | null }>("/provider/pings/active");
}

export function getProviderAssistanceRequests(limit = 100) {
  return request<ProviderAssistanceRequest[]>(`/provider/requests?limit=${limit}`);
}

export function acceptDispatchPing(pingId: string) {
  return request<MatchResponse>(`/provider/pings/${encodeURIComponent(pingId)}/accept`, {
    method: "POST",
  });
}

export function rejectDispatchPing(pingId: string) {
  return request<{ message: string }>(`/provider/pings/${encodeURIComponent(pingId)}/reject`, {
    method: "POST",
  });
}

export function getAdminAlerts() {
  return request<{ alerts: ExhaustedAlert[]; total: number }>("/admin/alerts");
}

export function getAdminOverview() {
  return request<AdminOverview>("/admin/overview");
}

export function getAdminUsers() {
  return request<AdminUser[]>("/admin/users?limit=100");
}

export function updateAdminUserRole(id: string, role: string) {
  return request<AdminUser, { role: string }>(`/admin/users/${encodeURIComponent(id)}/role`, { method: "PUT", body: { role } });
}

export function getAdminProviders() {
  return request<AdminProvider[]>("/admin/providers?limit=100");
}

export function updateAdminProviderStatus(id: string, payload: { is_active?: boolean; is_verified?: boolean }) {
  return request<AdminProvider, { is_active?: boolean; is_verified?: boolean }>(`/admin/providers/${encodeURIComponent(id)}/status`, { method: "PUT", body: payload });
}

export function getAdminAuditLogs() {
  return request<AdminAuditLog[]>("/admin/audit?limit=100");
}

export function getAdminHazards() { return request<RoadHazardResponse[]>("/admin/hazards?limit=100"); }
export function verifyAdminHazard(id: string, isVerified: boolean) { return request<{ id: string; is_verified: boolean }, { is_verified: boolean }>(`/admin/hazards/${encodeURIComponent(id)}/verify`, { method: "PUT", body: { is_verified: isVerified } }); }
export function getAdminRequests() { return request<AssistanceRequestResponse[]>("/admin/requests?limit=100"); }
export function getAdminResources() { return request<ResourceResponse[]>("/admin/resources?limit=100"); }
export function getAdminCamps() { return request<DistributionCamp[]>("/admin/camps?limit=100"); }
export function assignAdminRequest(id: string, providerId: string) { return request<{ message: string; ping_id: string }, { provider_id: string }>(`/admin/requests/${encodeURIComponent(id)}/assign`, { method: "POST", body: { provider_id: providerId } }); }
export function rebuildAdminHazardClusters() { return request<{ message: string; processed: number }>("/admin/ai/rebuild-hazard-clusters", { method: "POST" }); }


