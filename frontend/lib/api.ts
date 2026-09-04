import type {
  ApiResponse,
  Provider,
  RegisterProviderRequest,
  RegisterUserRequest,
  User,
} from "@/types";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:4000/api";

async function post<TResponse, TBody>(
  path: string,
  body: TBody
): Promise<ApiResponse<TResponse>> {
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

    return { success: true, data: json?.data ?? json };
  } catch (err) {
    return {
      success: false,
      error: err instanceof Error ? err.message : "Network error",
    };
  }
}


export function registerUser(payload: RegisterUserRequest) {
  return post<User, RegisterUserRequest>("/auth/register/user", payload);
}

export function registerProvider(payload: RegisterProviderRequest) {
  return post<Provider, RegisterProviderRequest>(
    "/auth/register/provider",
    payload
  );
}
