export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

export interface ApiClientConfig {
  baseUrl?: string;
  tenantId?: string;
  userId?: string;
}

interface ErrorPayload {
  error?: string;
}

function normalizeBaseUrl(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, "");
}

async function toApiError(res: Response): Promise<ApiError> {
  let message = `Request failed with status ${res.status}`;
  try {
    const body: unknown = await res.json();
    if (
      body !== null &&
      typeof body === "object" &&
      "error" in body &&
      typeof (body as ErrorPayload).error === "string"
    ) {
      message = (body as ErrorPayload).error as string;
    }
  } catch {
    // Non-JSON error bodies keep the default message.
  }
  return new ApiError(res.status, message);
}

export function createApiClient(config: ApiClientConfig = {}) {
  const baseUrl = normalizeBaseUrl(
    config.baseUrl ?? process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080",
  );
  const tenantId = config.tenantId ?? process.env.NEXT_PUBLIC_TENANT_ID ?? "";
  const userId = config.userId ?? process.env.NEXT_PUBLIC_USER_ID ?? "";

  async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const headers = new Headers(init.headers);
    if (init.body && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
    if (tenantId) {
      headers.set("X-Tenant-ID", tenantId);
    }
    if (userId) {
      headers.set("X-User-ID", userId);
    }

    const res = await fetch(`${baseUrl}${path}`, { ...init, headers });
    if (!res.ok) {
      throw await toApiError(res);
    }
    if (res.status === 204) {
      return undefined as T;
    }
    return (await res.json()) as T;
  }

  return {
    get: <T>(path: string) => request<T>(path),
    post: <T>(path: string, body: unknown) =>
      request<T>(path, { method: "POST", body: JSON.stringify(body) }),
    patch: <T>(path: string, body: unknown) =>
      request<T>(path, { method: "PATCH", body: JSON.stringify(body) }),
  };
}

export type ApiClient = ReturnType<typeof createApiClient>;

export const api = createApiClient();
