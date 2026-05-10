export interface Service {
  id: number
  name: string
  description: string
  url: string
  type: string
  interval: number
  status: 'operational' | 'degraded' | 'outage'
  isActive: boolean
  sortOrder: number
  createdAt: string
  updatedAt: string
}

export interface ServiceSummary {
  id: number
  name: string
  status: 'operational' | 'degraded' | 'outage'
  url: string
  type: string
  uptime: number
  latency: number
  interval: number
}

export interface ServiceDetail extends Service {
  uptime: number
  latency: number
}

export interface Heartbeat {
  id: number
  serviceId: number
  status: number
  latency: number
  message: string
  createdAt: string
}

export interface IncidentUpdate {
  id: number
  incidentId: number
  status: string
  content: string
  createdAt: string
}

export interface Incident {
  id: number
  serviceId: number
  title: string
  impact: 'minor' | 'major' | 'critical'
  status: 'investigating' | 'identified' | 'monitoring' | 'resolved'
  createdAt: string
  updatedAt: string
  updates: IncidentUpdate[]
}

export interface Maintenance {
  id: number
  title: string
  description: string
  scheduledStart: string
  scheduledEnd: string
  status: 'scheduled' | 'in_progress' | 'completed' | 'cancelled'
  affectedServices: string
  createdAt: string
}

export interface SummaryResponse {
  overallStatus: 'operational' | 'degraded' | 'outage'
  services: ServiceSummary[]
  activeIncidents: Incident[]
  maintenances: Maintenance[]
}

export interface ServiceHistoryResponse {
  service: Service
  uptime: number
  heartbeats: Heartbeat[]
}

export interface DashboardStats {
  totalServices: number
  operationalCount: number
  degradedCount: number
  outageCount: number
  activeIncidents: number
  activeMaintenances: number
  services: ServiceSummary[]
  recentIncidents: Incident[]
}

export interface Pagination {
  page: number
  limit: number
  totalPage: number
}

export interface LogEntry {
  id: number
  serviceId: number
  serviceName: string
  status: number
  latency: number
  message: string
  createdAt: string
}

export interface DailyStat {
  date: string
  uptimeMinutes: number
  downtimeMinutes: number
}

export interface ServiceDailyStats {
  serviceId: number
  days: [number, number, number][]
}

export interface ApiResponse<T = any> {
  code: number
  message: string
  data: T
}

export interface ApiKey {
  id: number
  name: string
  maskedKey: string
  expiresAt: string
  lastUsedAt: string
  lastUsedIP: string
  isActive: boolean
  createdAt: string
}

export interface ApiKeyCreated extends ApiKey {
  key: string
}
