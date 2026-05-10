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

  const json = await res.json()
  if (json.code >= 400) {
    throw new Error(json.message || 'Request failed')
  }
  return json as T
}

export const api = {
  // Public
  getSummary: () => request<ApiResponse<import('./types').SummaryResponse>>('GET', '/summary'),
  getServiceHistory: (id: number, days = 90) =>
    request<ApiResponse<import('./types').ServiceHistoryResponse>>('GET', `/services/${id}/history?days=${days}`),
  getPublicIncidents: (page = 1, limit = 20) =>
    request<ApiResponse<{ incidents: import('./types').Incident[]; pagination: import('./types').Pagination }>>('GET', `/incidents?page=${page}&limit=${limit}`),
  getMaintenances: () => request<ApiResponse<import('./types').Maintenance[]>>('GET', '/maintenances'),
  getServiceDailyStats: (id: number, days = 90) =>
    request<ApiResponse<import('./types').ServiceDailyStats>>('GET', `/services/${id}/daily-stats?days=${days}`),
  getSiteConfig: () =>
    request<ApiResponse<Record<string, string>>>('GET', '/site-config'),

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
}
