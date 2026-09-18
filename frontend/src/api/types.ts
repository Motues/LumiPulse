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
  showOnHomepage: boolean
  /** 公开访问标识：公开页面的服务详情 URL 使用它，而不是自增 id */
  publicHash: string
  /** 仅对该服务跳过 HTTPS 证书校验（自签证书的内网服务） */
  insecureSkipVerify: boolean
  createdAt: string
  updatedAt: string
}

export interface ServiceSummary {
  id: number
  /** 公开访问标识：公开页面的服务详情 URL 使用它，而不是自增 id */
  publicHash: string
  name: string
  status: 'operational' | 'degraded' | 'outage'
  url: string
  type: string
  uptime: number
  latency: number
  interval: number
  showOnHomepage: boolean
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
  publicHash: string
  serviceId: number
  title: string
  impact: 'minor' | 'major' | 'critical'
  status: 'investigating' | 'identified' | 'monitoring' | 'resolved'
  affectedServices: string
  parentId?: number
  resolvedAt?: string
  createdAt: string
  updatedAt: string
  updates: IncidentUpdate[]
  children?: Incident[]
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
  /** 重复方式：'' = 一次性窗口，daily / weekly / monthly = 周期维护 */
  recurrence: '' | 'daily' | 'weekly' | 'monthly'
  /** 重复间隔，默认 1 */
  recurrenceInterval: number
  /** 仅 weekly：1=周一 … 7=周日（仅用于回显） */
  recurrenceWeekday: number
  /** 仅 monthly：1~31 */
  recurrenceMonthday: number
  /** 重复截止日期 YYYY-MM-DD，空值表示一直重复 */
  recurrenceUntil: string
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

/** 窗口内延迟分位数汇总（仅统计成功样本） */
export interface LatencyStats {
  samples: number
  avg: number
  p95: number
  p99: number
  max: number
}

/** 延迟分桶 + 分位数汇总响应 */
export interface LatencyResponse {
  start: string
  interval: number
  latencies: number[]
  statuses: number[]
  /** 窗口内无成功样本时为 null */
  stats: LatencyStats | null
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
  recentIncidentsTotal: number
  recentIncidentsResolved: number
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
  /** 公开访问标识：公开页面用它拼服务详情 URL */
  publicHash: string
  days: [number, number, number][]
}

/** 批量每日统计接口响应 */
export interface BatchDailyStatsResponse {
  days: number
  services: ServiceDailyStats[]
}

/** 月度 SLA 报告：整站汇总 */
export interface MonthlySLASummary {
  uptime: number
  totalProbes: number
  downtimeProbes: number
  incidents: number
  /** 事件导致的累计不可用时长（秒） */
  downtimeSeconds: number
  avgLatency: number
}

/** 月度 SLA 报告：单服务 */
export interface ServiceSLASummary {
  serviceId: number
  publicHash: string
  name: string
  uptime: number
  totalProbes: number
  downtimeProbes: number
  incidents: number
  downtimeSeconds: number
  avgLatency: number
  /** 该月内有探测数据的天数，0 表示数据缺失 */
  coveredDays: number
}

/** 月度 SLA 报告 */
export interface MonthlySLAReport {
  /** YYYY-MM */
  month: string
  label: string
  days: number
  /** 月份已结束、数据已固化 */
  final: boolean
  summary: MonthlySLASummary
  services: ServiceSLASummary[]
}

/** SLA 趋势中的一个月份 */
export interface SLATrendPoint {
  month: string
  label: string
  uptime: number
  incidents: number
  avgLatency: number
  totalProbes: number
}

export interface SLATrendResponse {
  months: SLATrendPoint[]
  /** 每日明细保留天数，超出该范围的月份依赖已固化的月报 */
  retainedDays: number
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

export interface Server {
  id: number
  name: string
  description: string
  autoMerge: boolean
  autoMergeThreshold: number
  createdAt: string
  updatedAt: string
}

export interface ProbeTask {
  id: number
  serviceId: number
  serverId?: number
  triggerCount: number
  isActive: boolean
  serviceName: string
  serverName: string
  createdAt: string
  updatedAt: string
}
