import type {
  AuthResponse,
  LoginProviderRequest,
  LoginUserRequest,
  Provider,
  RegisterProviderRequest,
  RegisterUserRequest,
  User,
} from "@/types";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080/api";

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

export function saveSession(response: AuthResponse<unknown>) {
  if (typeof window !== "undefined") {
    localStorage.setItem("resqio_token", response.token);
    localStorage.setItem("resqio_account_type", response.account_type);
  }
}

export function clearSession() {
  if (typeof window !== "undefined") {
    localStorage.removeItem("resqio_token");
    localStorage.removeItem("resqio_account_type");
  }
}

export function getSession() {
  if (typeof window === "undefined") return null;
  const token = localStorage.getItem("resqio_token");
  const accountType = localStorage.getItem("resqio_account_type") as
    | "user"
    | "provider"
    | null;
  return token && accountType ? { token, accountType } : null;
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
