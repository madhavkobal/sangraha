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

// --- Bucket types ---

export interface Bucket {
  name: string
  owner: string
  region: string
  versioning: string
  acl: string
  object_count: number
  total_bytes: number
  created_at: string
  sse_algorithm?: string
}

export interface CreateBucketRequest {
  name: string
  region?: string
  acl?: string
}

// --- Object types ---

export interface ObjectRecord {
  key: string
  size: number
  etag: string
  content_type: string
  last_modified: string
  owner: string
  storage_class: string
  tags?: Record<string, string>
  version_id?: string
}

export interface ListObjectsResponse {
  objects: ObjectRecord[]
  prefixes: string[]
}

// --- Alert types ---

export interface AlertRule {
  id: string
  metric: string
  operator: string
  threshold: number
  label: string
  created_at: string
}

export interface CreateAlertRuleRequest {
  metric: string
  operator: string
  threshold: number
  label: string
}

export interface AlertEvent {
  id: string
  rule_id: string
  rule_label: string
  metric: string
  fired_at: string
  value: number
  threshold: number
  resolved: boolean
  resolved_at?: string
}

// --- Audit types ---

export interface AuditEntry {
  time: string
  request_id: string
  user: string
  action: string
  bucket?: string
  key?: string
  source_ip?: string
  status: number
  bytes?: number
  duration_ms: number
  error?: string
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

  buckets: {
    list: () => request<Bucket[]>('/admin/v1/buckets'),
    create: (body: CreateBucketRequest) =>
      request<Bucket>('/admin/v1/buckets', { method: 'POST', body: JSON.stringify(body) }),
    delete: (name: string) => request<void>(`/admin/v1/buckets/${name}`, { method: 'DELETE' }),
    listObjects: (name: string, prefix?: string, continuationToken?: string) => {
      const params = new URLSearchParams()
      if (prefix) params.set('prefix', prefix)
      if (continuationToken) params.set('continuation_token', continuationToken)
      return request<ListObjectsResponse>(`/admin/v1/buckets/${name}/objects?${params}`)
    },
    deleteObject: (bucket: string, key: string) =>
      request<void>(`/admin/v1/buckets/${bucket}/objects/${key}`, { method: 'DELETE' }),
  },

  alerts: {
    listRules: () => request<AlertRule[]>('/admin/v1/alerts'),
    createRule: (body: CreateAlertRuleRequest) =>
      request<AlertRule>('/admin/v1/alerts', { method: 'POST', body: JSON.stringify(body) }),
    deleteRule: (id: string) => request<void>(`/admin/v1/alerts/${id}`, { method: 'DELETE' }),
    history: () => request<AlertEvent[]>('/admin/v1/alerts/history'),
  },

  audit: {
    query: (params: { from?: string; to?: string; user?: string; bucket?: string; action?: string; limit?: number }) => {
      const p = new URLSearchParams()
      if (params.from) p.set('from', params.from)
      if (params.to) p.set('to', params.to)
      if (params.user) p.set('user', params.user)
      if (params.bucket) p.set('bucket', params.bucket)
      if (params.action) p.set('action', params.action)
      if (params.limit) p.set('limit', String(params.limit))
      return request<AuditEntry[]>(`/admin/v1/audit?${p}`)
    },
  },
}
