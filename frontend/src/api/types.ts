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
  /** 单次探测超时（秒）。0 表示使用默认值 10 秒 */
  timeoutSeconds: number
  /** HTTPS 证书到期时间（RFC3339）；空值表示无证书信息（非 HTTPS 或尚未探测成功） */
  certExpiresAt?: string
  /** HTTP 探测请求方法，空值等价于 GET（仅 type=http 生效） */
  httpMethod: string
  /** 自定义请求头（JSON 字符串）。属于敏感信息：后端读接口一律返回空串，
   *  前端留空提交表示「保持原值」 */
  httpHeaders: string
  /** 请求体纯文本，语义同 httpHeaders */
  httpBody: string
  /** 期望状态码模式：200 / 200,301 / 200-299 / 2xx，留空沿用 200-399 */
  expectStatus: string
  /** 期望响应关键字，留空表示不做关键字匹配 */
  expectKeyword: string
  /** 所属服务分组（服务聚合文件夹）。未返回表示未分组 */
  folderId?: number
  /** 所属服务分组名称（管理端列表接口回显用） */
  folderName?: string
  /** 公开首页「服务详情」要展示的内容块，逗号分隔（见 utils/homepageBlocks.ts）。
   *  空串 = 全部展示，'none' = 全部不展示；只影响公开页面，管理后台始终展示全部 */
  homepageBlocks?: string
  createdAt: string
  updatedAt: string
}

export interface ServiceSummary {
  id: number
  /** 公开访问标识：公开页面的服务详情 URL 使用它，而不是自增 id */
  publicHash: string
  /** 所属服务分组（未分组时为空），公开首页据此把服务聚合成一个条目 */
  folderId?: number
  name: string
  status: 'operational' | 'degraded' | 'outage'
  url: string
  type: string
  uptime: number
  latency: number
  interval: number
  showOnHomepage: boolean
  /** HTTPS 证书到期时间（RFC3339）；空值表示无证书信息 */
  certExpiresAt?: string
  /** 公开首页「服务详情」要展示的内容块，逗号分隔（见 utils/homepageBlocks.ts）。
   *  空串 = 全部展示，'none' = 全部不展示 */
  homepageBlocks?: string
}

/** 服务分组（服务聚合文件夹）：把多个服务聚合成一个首页条目展示 */
export interface ServiceFolder {
  id: number
  name: string
  description?: string
  /** 是否在公开首页展示；关闭时分组内的服务也不再单独展示 */
  showOnHomepage: boolean
  sortOrder: number
  createdAt: string
  updatedAt: string
  /** 分组内服务数量（管理端列表接口返回） */
  serviceCount?: number
}

/** 服务分组的融合结果：可用率与延迟按探测次数加权融合 */
export interface FolderSummary {
  id: number
  name: string
  description?: string
  /** 聚合状态：全部正常 → operational，全部故障 → outage，其余 → degraded */
  status: 'operational' | 'degraded' | 'outage'
  uptime: number
  latency: number
  /** 分组内的服务明细，展开下拉时展示 */
  services: ServiceSummary[]
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
  /** 事后复盘：根因（仅在勾选对外公开时公开接口才会返回） */
  rootCause?: string
  /** 事后复盘：处理措施 */
  resolution?: string
  /** 事后复盘：复盘文档链接 */
  postmortemUrl?: string
  /** 是否对外公开复盘内容；公开接口在 false 时不会返回上面三个字段 */
  postmortemPublic?: boolean
  /** 是否已被人工确认。确认后不再发送升级通知 */
  acknowledged: boolean
  /** 确认时间（RFC3339）；未确认时为空 */
  acknowledgedAt?: string
  /** 确认人（当前为单管理员模型，仅作留痕） */
  acknowledgedBy?: string
  /** 已发送的升级通知次数 */
  escalationCount: number
  /** 上次升级通知时间（RFC3339） */
  lastEscalatedAt?: string
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
  /** 服务分组的融合结果；首页用它把同一分组下的服务合并成一个条目 */
  folders?: FolderSummary[]
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

/** 响应时间热力图的一个格子（某天某小时，北京时间） */
export interface LatencyHeatmapCell {
  /** YYYY-MM-DD */
  day: string
  /** 0~23 */
  hour: number
  /** 成功样本的平均延迟（ms） */
  avg: number
  /** 参与统计的成功样本数 */
  samples: number
  /** 该小时内的失败探测次数 */
  failures: number
  /** 该小时是否落在维护窗口内：是则失败探测用蓝色（计划内停机）展示 */
  maintenance?: boolean
}

/** 响应时间热力图响应：只含有数据的格子，前端按 from~to 补齐 */
export interface LatencyHeatmapResponse {
  from: string
  to: string
  days: number
  /** 所有格子的最大平均延迟，用于色阶 */
  maxAvg: number
  cells: LatencyHeatmapCell[]
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
  /** 后端按该服务的判定口径（期望状态码 + 期望关键字）给出的结论。
   *  前端不再自行按 status 猜测，避免与筛选、统计口径不一致。 */
  isSuccess: boolean
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
  /** 每天一个四元组 [upCount, downCount, statusCode, maintenanceCount]。
   *  maintenanceCount 是维护窗口内的失败探测次数：不计入可用率，
   *  矩阵里用蓝色（计划内停机）而不是红色展示 */
  days: [number, number, number, number][]
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

export interface Subscriber {
  id: number
  email: string
  verified: boolean
  subscribedServices: string
  createdAt: string
  updatedAt: string
}

/** 退订页读取到的订阅信息（services 为空 = 订阅全部服务） */
export interface SubscriptionView {
  email: string
  services: number[]
  siteName: string
}

export interface ApiKey {
  id: number
  name: string
  maskedKey: string
  expiresAt: string
  lastUsedAt: string
  lastUsedIP: string
  isActive: boolean
  /** 权限范围：read 只能读（GET/HEAD），write 允许写操作；高危操作两者都不可调用 */
  scope: 'read' | 'write'
  /** 每分钟请求上限，0 表示不限制 */
  rateLimitPerMinute: number
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
