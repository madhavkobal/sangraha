// api/client.ts — typed fetch wrappers for the sangraha admin API.

let BASE_URL = ''

export function setBaseURL(url: string) {
  BASE_URL = url.replace(/\/$/, '')
}

export function getBaseURL(): string {
  return BASE_URL
}

class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(BASE_URL + path, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const body = await res.json()
      msg = body.error ?? msg
    } catch {
      // ignore parse error
    }
    throw new ApiError(res.status, msg)
  }
  if (res.status === 204) return undefined as unknown as T
  return res.json() as Promise<T>
}

// ---- Types ----

export interface HealthResponse {
  status: string
}

export interface InfoResponse {
  version: string
  build_time: string
  uptime_sec: number
}

export interface User {
  access_key: string
  owner: string
  is_root: boolean
  secret_key?: string
}

export interface CreateUserRequest {
  owner: string
}

export interface SafeConfig {
  server: {
    s3_address: string
    admin_address: string
    tls: { enabled: boolean; cert_file: string; key_file: string; auto_self_signed: boolean }
  }
  storage: { backend: string; data_dir: string }
  metadata: { path: string }
  auth: { root_access_key: string; root_secret_key: string }
  logging: { level: string; format: string; audit_log: string }
  limits: { max_object_size: string; max_bucket_count: number; rate_limit_rps: number }
}

export interface ConfigPatch {
  logging?: { level?: string; format?: string }
  limits?: { rate_limit_rps?: number; max_bucket_count?: number }
}

export interface ConfigUpdateResponse {
  applied: boolean
  restart_required: boolean
  message: string
}

export interface ConfigValidateResponse {
  valid: boolean
  errors?: string[]
}

export interface TLSInfo {
  status?: string
  subject?: string
  issuer?: string
  not_before?: string
  not_after?: string
  days_until_expiry?: number
  fingerprint_sha256?: string
  is_self_signed?: boolean
}

export interface ConnectionStatus {
  active_connections: number
}

export interface GCStatus {
  running: boolean
  scanned: number
  deleted: number
  freed_bytes: number
  last_run?: string
}

// ---- API calls ----

export const api = {
  health: () => request<HealthResponse>('/admin/v1/health'),
  info: () => request<InfoResponse>('/admin/v1/info'),

  users: {
    list: () => request<User[]>('/admin/v1/users'),
    create: (body: CreateUserRequest) =>
      request<User>('/admin/v1/users', { method: 'POST', body: JSON.stringify(body) }),
    delete: (ak: string) => request<void>(`/admin/v1/users/${ak}`, { method: 'DELETE' }),
    rotateKey: (ak: string) =>
      request<User>(`/admin/v1/users/${ak}/keys/rotate`, { method: 'POST' }),
  },

  config: {
    get: () => request<SafeConfig>('/admin/v1/config'),
    validate: (patch: ConfigPatch) =>
      request<ConfigValidateResponse>('/admin/v1/config/validate', {
        method: 'POST',
        body: JSON.stringify(patch),
      }),
    update: (patch: ConfigPatch) =>
      request<ConfigUpdateResponse>('/admin/v1/config', {
        method: 'PUT',
        body: JSON.stringify(patch),
      }),
  },

  tls: {
    info: () => request<TLSInfo>('/admin/v1/tls'),
    renew: () => request<{ renewed: boolean; message: string }>('/admin/v1/tls/renew', { method: 'POST' }),
  },

  server: {
    reload: () => request<{ reloaded: boolean; message: string }>('/admin/v1/server/reload', { method: 'POST' }),
    connections: () => request<ConnectionStatus>('/admin/v1/connections'),
  },

  gc: {
    trigger: () => request<{ status: string }>('/admin/v1/gc', { method: 'POST' }),
    status: () => request<GCStatus>('/admin/v1/gc/status'),
  },
}
