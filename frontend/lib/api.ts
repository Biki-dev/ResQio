import type {
  AuthResponse,
  AssistanceRequestResponse,
  DisasterReportResponse,
  LoginProviderRequest,
  LoginUserRequest,
  MutualAidItemRequest,
  MutualAidItemResponse,
  Provider,
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

export function publishResource(payload: ResourceRequest) {
  return request<ResourceResponse, ResourceRequest>("/resources", {
    method: "POST",
    body: payload,
  });
}
