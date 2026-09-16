import type { ApiResponse } from './types'

const BASE = '/api/v1'

function getToken(): string | null {
  return localStorage.getItem('token')
}

async function request<T>(
  method: string,
  path: string,
  body?: any,
  auth = false,
): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  if (auth) {
    const token = getToken()
    if (token) headers['Authorization'] = `Bearer ${token}`
  }

  const res = await fetch(`${BASE}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })

  // Token invalid — redirect to login (skip if already there)
  if (res.status === 401 && auth && !['/', '/login'].includes(window.location.pathname)) {
    localStorage.removeItem('token')
    window.location.href = '/login'
    throw new Error('Token expired')
  }

  // 后端可能返回空 body（如未知接口）或 HTML（反代错误页），
  // 直接 res.json() 会抛出难以理解的解析错误，这里统一转成友好提示。
  const raw = await res.text()
  if (!raw) {
    throw new Error(res.ok ? '服务返回了空响应' : `请求失败 (${res.status})`)
  }

  let json: any
  try {
    json = JSON.parse(raw)
  } catch {
    throw new Error(`服务返回了非预期内容 (${res.status})`)
  }

  if (typeof json?.code === 'number' && json.code >= 400) {
    throw new Error(json.message || 'Request failed')
  }
  if (!res.ok) {
    throw new Error(json?.message || `请求失败 (${res.status})`)
  }
  return json as T
}

export const api = {
  // Public
  getSummary: () => request<ApiResponse<import('./types').SummaryResponse>>('GET', '/summary'),
  // 公开服务接口一律用随机 hash 定位，不暴露数据库自增 ID
  getServiceHistory: (hash: string, days = 90) =>
    request<ApiResponse<import('./types').ServiceHistoryResponse>>('GET', `/services/${encodeURIComponent(hash)}/history?days=${days}`),
  getServiceLatency: (hash: string, days = 1) =>
    request<ApiResponse<{ start: string; interval: number; latencies: number[]; statuses: number[] }>>('GET', `/services/${encodeURIComponent(hash)}/latency?days=${days}`),
  getPublicIncidents: (page = 1, limit = 20) =>
    request<ApiResponse<{ incidents: import('./types').Incident[]; pagination: import('./types').Pagination }>>('GET', `/incidents?page=${page}&limit=${limit}`),
  // 公开事件详情使用随机 hash 访问，不暴露数据库自增 ID
  getPublicIncident: (hash: string) =>
    request<ApiResponse<import('./types').Incident>>('GET', `/incidents/${encodeURIComponent(hash)}`),
  getMaintenances: () => request<ApiResponse<import('./types').Maintenance[]>>('GET', '/maintenances'),
  getServiceDailyStats: (hash: string, days = 90) =>
    request<ApiResponse<import('./types').ServiceDailyStats>>('GET', `/services/${encodeURIComponent(hash)}/daily-stats?days=${days}`),
  // 批量版本：首页一次性取回所有服务的状态矩阵，避免按服务逐个请求
  getBatchDailyStats: (days = 90) =>
    request<ApiResponse<import('./types').BatchDailyStatsResponse>>('GET', `/daily-stats?days=${days}`),
  getSiteConfig: () =>
    request<ApiResponse<Record<string, string>>>('GET', '/site-config'),
  getPublicServices: () =>
    request<ApiResponse<import('./types').ServiceSummary[]>>('GET', '/services'),

  // Public subscription
  subscribe: (email: string, services?: number[]) =>
    request<ApiResponse<void>>('POST', '/subscribe', services ? { email, services } : { email }),

  // Auth
  login: (username: string, password: string) =>
    request<ApiResponse<{ token: string; needsSetup: boolean }>>('POST', '/admin/login', { username, password }),

  // Admin - Setup
  setup: (username: string, password: string) =>
    request<ApiResponse<void>>('POST', '/admin/setup', { username, password }),

  // Admin - Settings
  getSettings: () => request<ApiResponse<Record<string, string>>>('GET', '/admin/settings', undefined, true),
  updateSettings: (settings: Record<string, string>) =>
    request<ApiResponse<void>>('PUT', '/admin/settings', settings, true),

  // Admin - Dashboard
  getStats: () => request<ApiResponse<import('./types').DashboardStats>>('GET', '/admin/stats', undefined, true),
  // 管理端批量每日统计（含未在首页展示的服务）
  getAdminBatchDailyStats: (days = 90) =>
    request<ApiResponse<import('./types').BatchDailyStatsResponse>>('GET', `/admin/daily-stats?days=${days}`, undefined, true),

  // Admin - Profile
  getCurrentUser: () => request<ApiResponse<{ username: string }>>('GET', '/admin/current-user', undefined, true),
  updateProfile: (data: { oldPassword: string; newUsername?: string; newPassword?: string }) =>
    request<ApiResponse<void>>('PUT', '/admin/profile', data, true),

  // Admin - Notifications
  testEmail: (to: string) =>
    request<ApiResponse<void>>('POST', '/admin/test-email', { to }, true),

  // Admin - Logs
  getLogs: (page = 1, limit = 50, serviceId = 0, status = 'all') =>
    request<ApiResponse<{ logs: import('./types').LogEntry[]; pagination: import('./types').Pagination }>>('GET', `/admin/logs?page=${page}&limit=${limit}&serviceId=${serviceId}&status=${status}`, undefined, true),

  // Admin - Services
  getAdminServices: () =>
    request<ApiResponse<(import('./types').Service & { uptime: number; latency: number })[]>>('GET', '/admin/services', undefined, true),
  createService: (data: any) =>
    request<ApiResponse<import('./types').Service>>('POST', '/admin/services', data, true),
  updateService: (id: number, data: any) =>
    request<ApiResponse<import('./types').Service>>('PUT', `/admin/services/${id}`, data, true),
  deleteService: (id: number) =>
    request<ApiResponse<void>>('DELETE', `/admin/services/${id}`, undefined, true),
  reorderServices: (services: { id: number; sortOrder: number }[]) =>
    request<ApiResponse<void>>('PUT', '/admin/services/reorder', { services }, true),

  // Admin - Incidents
  getAdminIncidents: (page = 1, limit = 20) =>
    request<ApiResponse<{ incidents: import('./types').Incident[]; pagination: import('./types').Pagination }>>('GET', `/admin/incidents?page=${page}&limit=${limit}`, undefined, true),
  createIncident: (data: any) =>
    request<ApiResponse<import('./types').Incident>>('POST', '/admin/incidents', data, true),
  updateIncident: (id: number, data: any) =>
    request<ApiResponse<import('./types').Incident>>('PATCH', `/admin/incidents/${id}`, data, true),
  deleteIncident: (id: number) =>
    request<ApiResponse<void>>('DELETE', `/admin/incidents/${id}`, undefined, true),
  createIncidentUpdate: (id: number, data: any) =>
    request<ApiResponse<import('./types').IncidentUpdate>>('POST', `/admin/incidents/${id}/updates`, data, true),
  updateIncidentUpdate: (incidentId: number, updateId: number, data: any) =>
    request<ApiResponse<void>>('PUT', `/admin/incidents/${incidentId}/updates/${updateId}`, data, true),
  deleteIncidentUpdate: (incidentId: number, updateId: number) =>
    request<ApiResponse<void>>('DELETE', `/admin/incidents/${incidentId}/updates/${updateId}`, undefined, true),

  // Admin - ApiKeys
  getApiKeys: () =>
    request<ApiResponse<import('./types').ApiKey[]>>('GET', '/admin/api-keys', undefined, true),
  createApiKey: (data: { name: string; expiresAt: string }) =>
    request<ApiResponse<import('./types').ApiKeyCreated>>('POST', '/admin/api-keys', data, true),
  deleteApiKey: (id: number) =>
    request<ApiResponse<void>>('DELETE', `/admin/api-keys/${id}`, undefined, true),
  updateApiKey: (id: number, data: { name: string }) =>
    request<ApiResponse<void>>('PUT', `/admin/api-keys/${id}`, data, true),

  // Admin - Maintenances
  getAdminMaintenances: () =>
    request<ApiResponse<import('./types').Maintenance[]>>('GET', '/admin/maintenances', undefined, true),
  createMaintenance: (data: any) =>
    request<ApiResponse<import('./types').Maintenance>>('POST', '/admin/maintenances', data, true),
  updateMaintenance: (id: number, data: any) =>
    request<ApiResponse<import('./types').Maintenance>>('PUT', `/admin/maintenances/${id}`, data, true),
  deleteMaintenance: (id: number) =>
    request<ApiResponse<void>>('DELETE', `/admin/maintenances/${id}`, undefined, true),

  // Admin - Servers
  getServers: () =>
    request<ApiResponse<import('./types').Server[]>>('GET', '/admin/servers', undefined, true),
  createServer: (data: any) =>
    request<ApiResponse<import('./types').Server>>('POST', '/admin/servers', data, true),
  updateServer: (id: number, data: any) =>
    request<ApiResponse<import('./types').Server>>('PUT', `/admin/servers/${id}`, data, true),
  deleteServer: (id: number) =>
    request<ApiResponse<void>>('DELETE', `/admin/servers/${id}`, undefined, true),

  // Admin - Probe Tasks
  getProbeTasks: () =>
    request<ApiResponse<import('./types').ProbeTask[]>>('GET', '/admin/probe-tasks', undefined, true),
  createProbeTask: (data: any) =>
    request<ApiResponse<import('./types').ProbeTask>>('POST', '/admin/probe-tasks', data, true),
  updateProbeTask: (id: number, data: any) =>
    request<ApiResponse<import('./types').ProbeTask>>('PUT', `/admin/probe-tasks/${id}`, data, true),
  deleteProbeTask: (id: number) =>
    request<ApiResponse<void>>('DELETE', `/admin/probe-tasks/${id}`, undefined, true),

  // Admin - Incident detail (with children and all updates)
  getAdminIncident: (id: number) =>
    request<ApiResponse<import('./types').Incident>>('GET', `/admin/incidents/${id}`, undefined, true),

  // Admin - Incident merge/split
  mergeIncident: (id: number, sourceId: number) =>
    request<ApiResponse<void>>('POST', `/admin/incidents/${id}/merge`, { sourceId }, true),
  splitIncident: (id: number) =>
    request<ApiResponse<void>>('POST', `/admin/incidents/${id}/split`, undefined, true),

  // Generic request for custom endpoints
  request: <T = any>(method: string, path: string, body?: any, auth = false): Promise<T> =>
    request<T>(method, path, body, auth),
}
