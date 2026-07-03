import type { BanEntry, RequestLog, RoutingRule } from "./types";

const DEFAULT_BASE_URL = "http://127.0.0.1:8081";

export function getApiConfig(): { baseUrl: string; token: string } {
  if (typeof window === "undefined") {
    return { baseUrl: DEFAULT_BASE_URL, token: "" };
  }
  return {
    baseUrl: window.localStorage.getItem("eidolon_api_url") || DEFAULT_BASE_URL,
    token: window.localStorage.getItem("eidolon_admin_token") || "",
  };
}

export function setApiConfig(baseUrl: string, token: string) {
  window.localStorage.setItem("eidolon_api_url", baseUrl);
  window.localStorage.setItem("eidolon_admin_token", token);
}

class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const { baseUrl, token } = getApiConfig();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string> | undefined),
  };
  if (token) headers["Authorization"] = `Bearer ${token}`;

  const res = await fetch(`${baseUrl}${path}`, { ...options, headers });

  if (!res.ok) {
    let message = `Erro ${res.status}`;
    try {
      const body = await res.json();
      message = body.error || message;
    } catch {
      /* resposta não era JSON, mantém mensagem padrão */
    }
    throw new ApiError(res.status, message);
  }

  const contentType = res.headers.get("content-type") || "";
  if (contentType.includes("application/json")) {
    return res.json() as Promise<T>;
  }
  return undefined as unknown as T;
}

export const api = {
  listLogs: (limit = 100) => request<RequestLog[]>(`/api/logs?limit=${limit}`),

  listBans: () => request<BanEntry[]>(`/api/security/blacklist`),
  ban: (ip: string, durationSeconds: number) =>
    request(`/api/security/blacklist`, {
      method: "POST",
      body: JSON.stringify({ ip, duration_seconds: durationSeconds }),
    }),
  unban: (ip: string) =>
    request(`/api/security/blacklist/${encodeURIComponent(ip)}`, { method: "DELETE" }),

  listRules: () => request<RoutingRule[]>(`/api/routing/rules`),
  upsertRule: (rule: Partial<RoutingRule>) =>
    request<RoutingRule>(`/api/routing/rules`, {
      method: "POST",
      body: JSON.stringify(rule),
    }),
  deleteRule: (id: string) =>
    request(`/api/routing/rules/${encodeURIComponent(id)}`, { method: "DELETE" }),

  health: () => request<string>(`/__eidolon/health`),
};

export { ApiError };
