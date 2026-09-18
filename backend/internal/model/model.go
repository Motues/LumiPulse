package model

// Service 监控服务/节点
type Service struct {
	ID             int64  `db:"id" json:"id"`
	Name           string `db:"name" json:"name"`
	Description    string `db:"description" json:"description,omitempty"`
	URL            string `db:"url" json:"url"`
	Type           string `db:"type" json:"type"` // http, tcp, ping
	Interval       int    `db:"interval" json:"interval"`
	Status         string `db:"status" json:"status"` // operational, degraded, outage
	IsActive       bool   `db:"is_active" json:"isActive"`
	SortOrder      int    `db:"sort_order" json:"sortOrder"`
	ShowOnHomepage bool   `db:"show_on_homepage" json:"showOnHomepage"`
	// PublicHash 公开访问标识（32 位随机十六进制）。公开页面的服务详情 URL 使用它，
	// 而不是自增 ID，避免对外暴露数据库主键与记录规模。
	PublicHash string `db:"public_hash" json:"publicHash"`
	// InsecureSkipVerify 仅对该服务跳过 HTTPS 证书校验（默认关闭）。
	// 用于自签证书的内网服务，避免为了个别服务而全局关闭校验。
	InsecureSkipVerify bool   `db:"insecure_skip_verify" json:"insecureSkipVerify"`
	CreatedAt          string `db:"created_at" json:"createdAt"`
	UpdatedAt          string `db:"updated_at" json:"updatedAt"`
}

// Heartbeat 健康检查记录
type Heartbeat struct {
	ID        int64  `db:"id" json:"id"`
	ServiceID int64  `db:"service_id" json:"serviceId"`
	Status    int    `db:"status" json:"status"`             // HTTP status code or 1/0 for tcp/ping
	Latency   int    `db:"latency" json:"latency"`           // milliseconds
	Message   string `db:"message" json:"message,omitempty"` // error detail or response summary
	CreatedAt string `db:"created_at" json:"createdAt"`
}

// LatencyResponse 紧凑延迟数据响应
type LatencyResponse struct {
	Start     string        `json:"start"`     // 起始时间 ISO
	Interval  int           `json:"interval"`  // 间隔分钟数
	Latencies []int         `json:"latencies"` // 延迟数组 (ms)
	Statuses  []int         `json:"statuses"`  // 状态数组 (0=正常, 1=故障, -1=无数据)
	Stats     *LatencyStats `json:"stats"`     // 窗口内延迟分位数汇总（无样本时为 null）
}

// LatencyStats 窗口内延迟分位数汇总。
// 只统计成功样本：故障请求的耗时是超时等异常值，混进分位数会把 p95/p99 顶到超时上限，
// 反而看不出"正常请求到底慢不慢"。
type LatencyStats struct {
	Samples int     `db:"samples" json:"samples"` // 参与统计的成功样本数
	Avg     float64 `db:"avg" json:"avg"`         // 平均延迟
	P95     float64 `db:"p95" json:"p95"`         // 95 分位（近邻插值）
	P99     float64 `db:"p99" json:"p99"`         // 99 分位（近邻插值）
	Max     int     `db:"max" json:"max"`         // 最大延迟
}

// LatencyBucket 延迟分桶聚合结果（在 SQLite 内聚合，避免把原始心跳搬到内存）
type LatencyBucket struct {
	Bucket     int   `db:"bucket"`      // 桶序号（自 start 起，每个桶 interval 分钟）
	AvgLatency int64 `db:"avg_latency"` // 桶内平均延迟
	Failures   int64 `db:"failures"`    // 桶内失败次数
	Total      int64 `db:"total"`       // 桶内样本数
}

// ServiceDailyStats 单个服务的每日统计（批量接口用）
type ServiceDailyStats struct {
	// ServiceID 为自增 ID，仅供前端与本地服务列表匹配，不作为 URL 使用
	ServiceID int64 `json:"serviceId"`
	// PublicHash 公开访问标识，公开页面用它拼详情页 URL
	PublicHash string   `json:"publicHash"`
	Days       [][3]int `json:"days"`
}

// ServiceMonthly 月度 SLA 汇总。
// ServiceDaily 受 DAILY_RETENTION_DAYS 限制会被清理，月报要能长期留存，
// 因此在月末（或首次查询历史月份时）把每日汇总固化到这张表。
type ServiceMonthly struct {
	ID                   int64  `db:"id" json:"id"`
	ServiceID            int64  `db:"service_id" json:"serviceId"`
	Month                string `db:"month" json:"month"` // YYYY-MM
	UptimeCount          int    `db:"uptime_count" json:"uptimeCount"`
	DowntimeCount        int    `db:"downtime_count" json:"downtimeCount"`
	TotalLatency         int    `db:"total_latency" json:"totalLatency"`
	IncidentTotal        int    `db:"incident_total" json:"incidentTotal"`
	IncidentDowntimeSecs int    `db:"incident_downtime_seconds" json:"incidentDowntimeSeconds"`
	RecordedAt           string `db:"recorded_at" json:"recordedAt"`
	ClosedAt             string `db:"closed_at" json:"closedAt"` // 空=月份尚未结束，数据仍可能变化
	// CoveredDays 该月内有探测数据的天数（仅实时计算时填充，不落库）
	CoveredDays int `db:"-" json:"coveredDays"`
}

// --- 月度 SLA 报告（API 响应模型）---

// MonthlySLAReport 单个月份的 SLA 报告
type MonthlySLAReport struct {
	Month    string               `json:"month"` // YYYY-MM
	Label    string               `json:"label"` // 展示用月份标签
	Days     int                  `json:"days"`  // 该月天数
	Final    bool                 `json:"final"` // 是否已固化（月份已结束且已归档）
	Summary  MonthlySLASummary    `json:"summary"`
	Services []*ServiceSLASummary `json:"services"`
}

// MonthlySLASummary 整站汇总
type MonthlySLASummary struct {
	Uptime          float64 `json:"uptime"`          // 整体可用率（按探测次数加权）
	TotalProbes     int     `json:"totalProbes"`     // 总探测次数
	DowntimeProbes  int     `json:"downtimeProbes"`  // 失败探测次数
	Incidents       int     `json:"incidents"`       // 事件总数
	DowntimeSeconds int     `json:"downtimeSeconds"` // 事件导致的累计不可用时长（秒）
	AvgLatency      float64 `json:"avgLatency"`      // 平均响应时间（仅成功样本）
}

// ServiceSLASummary 单个服务的月度可用率
type ServiceSLASummary struct {
	ServiceID       int64   `json:"serviceId"`
	PublicHash      string  `json:"publicHash"`
	Name            string  `json:"name"`
	Uptime          float64 `json:"uptime"`
	TotalProbes     int     `json:"totalProbes"`
	DowntimeProbes  int     `json:"downtimeProbes"`
	Incidents       int     `json:"incidents"`
	DowntimeSeconds int     `json:"downtimeSeconds"`
	AvgLatency      float64 `json:"avgLatency"`
	CoveredDays     int     `json:"coveredDays"` // 该月内有探测数据的天数
}

// SLATrendPoint 趋势图上的一个月份
type SLATrendPoint struct {
	Month       string  `json:"month"`
	Label       string  `json:"label"`
	Uptime      float64 `json:"uptime"`
	Incidents   int     `json:"incidents"`
	AvgLatency  float64 `json:"avgLatency"`
	TotalProbes int     `json:"totalProbes"`
}

// SLATrendResponse 最近若干个月的整站趋势
type SLATrendResponse struct {
	Months   []*SLATrendPoint `json:"months"`
	Retained int              `json:"retainedDays"` // 每日明细保留天数，用于提示可回填范围
}

// Server 服务器（逻辑分组）
type Server struct {
	ID                 int64  `db:"id" json:"id"`
	Name               string `db:"name" json:"name"`
	Description        string `db:"description" json:"description,omitempty"`
	AutoMerge          bool   `db:"auto_merge" json:"autoMerge"`
	AutoMergeThreshold int    `db:"auto_merge_threshold" json:"autoMergeThreshold"`
	CreatedAt          string `db:"created_at" json:"createdAt"`
	UpdatedAt          string `db:"updated_at" json:"updatedAt"`
}

// ProbeTask 探测任务
type ProbeTask struct {
	ID           int64  `db:"id" json:"id"`
	ServiceID    int64  `db:"service_id" json:"serviceId"`
	ServerID     *int64 `db:"server_id" json:"serverId,omitempty"`
	TriggerCount int    `db:"trigger_count" json:"triggerCount"` // 连续失败阈值，默认 5
	IsActive     bool   `db:"is_active" json:"isActive"`
	CreatedAt    string `db:"created_at" json:"createdAt"`
	UpdatedAt    string `db:"updated_at" json:"updatedAt"`
	// Joined fields
	ServiceName string `db:"service_name" json:"serviceName,omitempty"`
	ServerName  string `db:"server_name" json:"serverName,omitempty"`
}

// Incident 故障事件
type Incident struct {
	ID               int64   `db:"id" json:"id"`
	PublicHash       string  `db:"public_hash" json:"publicHash"` // 公开访问标识（替代 ID）
	ServiceID        int64   `db:"service_id" json:"serviceId"`
	Title            string  `db:"title" json:"title"`
	Impact           string  `db:"impact" json:"impact"`                      // minor, major, critical
	Status           string  `db:"status" json:"status"`                      // investigating, identified, monitoring, resolved
	AffectedServices string  `db:"affected_services" json:"affectedServices"` // 逗号分隔的服务 ID
	ParentID         *int64  `db:"parent_id" json:"parentId,omitempty"`
	ResolvedAt       *string `db:"resolved_at" json:"resolvedAt,omitempty"`
	CreatedAt        string  `db:"created_at" json:"createdAt"`
	UpdatedAt        string  `db:"updated_at" json:"updatedAt"`
	// Joined fields
	Updates  []*IncidentUpdate `db:"-" json:"updates,omitempty"`
	Children []*Incident       `db:"-" json:"children,omitempty"`
}

// IncidentUpdate 故障事件进展更新
type IncidentUpdate struct {
	ID         int64  `db:"id" json:"id"`
	IncidentID int64  `db:"incident_id" json:"incidentId"`
	Status     string `db:"status" json:"status"`
	Content    string `db:"content" json:"content"`
	IsInternal bool   `db:"is_internal" json:"isInternal"`
	CreatedAt  string `db:"created_at" json:"createdAt"`
}

// Maintenance 维护计划
type Maintenance struct {
	ID               int64  `db:"id" json:"id"`
	Title            string `db:"title" json:"title"`
	Description      string `db:"description" json:"description,omitempty"`
	ScheduledStart   string `db:"scheduled_start" json:"scheduledStart"`
	ScheduledEnd     string `db:"scheduled_end" json:"scheduledEnd"`
	Status           string `db:"status" json:"status"`                      // scheduled, in_progress, completed, cancelled
	AffectedServices string `db:"affected_services" json:"affectedServices"` // JSON string or comma-separated IDs
	CreatedAt        string `db:"created_at" json:"createdAt"`

	// --- 周期维护 ---
	// Recurrence 重复方式：daily / weekly / monthly。空值表示一次性维护窗口。
	// 每次窗口结束后，检查器把 scheduled_start / scheduled_end 推进到下一个窗口，
	// 因此同一个维护计划会一直代表「当前这次 + 下一次」的时间。
	Recurrence string `db:"recurrence" json:"recurrence"`
	// RecurrenceInterval 重复间隔，默认 1（每 N 天 / N 周 / N 月）
	RecurrenceInterval int `db:"recurrence_interval" json:"recurrenceInterval"`
	// RecurrenceWeekday 仅 weekly：1=周一 … 7=周日
	RecurrenceWeekday int `db:"recurrence_weekday" json:"recurrenceWeekday"`
	// RecurrenceMonthday 仅 monthly：1~31，超出当月天数时取当月最后一天
	RecurrenceMonthday int `db:"recurrence_monthday" json:"recurrenceMonthday"`
	// RecurrenceUntil 重复截止日期（含当天，YYYY-MM-DD）。空值表示一直重复。
	RecurrenceUntil string `db:"recurrence_until" json:"recurrenceUntil"`
}

// User 管理后台用户
type User struct {
	ID           int64  `db:"id" json:"id"`
	Username     string `db:"username" json:"username"`
	PasswordHash string `db:"password_hash" json:"-"`
	Email        string `db:"email" json:"email"`
	LastLogin    string `db:"last_login" json:"lastLogin,omitempty"`
	CreatedAt    string `db:"created_at" json:"createdAt"`
}

// ApiKey API密钥
type ApiKey struct {
	ID         int64  `db:"id" json:"id"`
	Name       string `db:"name" json:"name"`
	Key        string `db:"key" json:"key,omitempty"`    // 完整密钥，仅创建时返回
	KeyPrefix  string `db:"key_prefix" json:"keyPrefix"` // 前8位用于显示区分
	ExpiresAt  string `db:"expires_at" json:"expiresAt"` // 空值=永久有效
	LastUsedAt string `db:"last_used_at" json:"lastUsedAt,omitempty"`
	LastUsedIP string `db:"last_used_ip" json:"lastUsedIP,omitempty"`
	IsActive   bool   `db:"is_active" json:"isActive"`
	CreatedAt  string `db:"created_at" json:"createdAt"`
}
type CreateApiKeyRequest struct {
	Name      string `json:"name" binding:"required"`
	ExpiresAt string `json:"expiresAt"` // 空值=永久有效
}

// --- API response models ---
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
type Pagination struct {
	Page      int   `json:"page"`
	Limit     int   `json:"limit"`
	TotalPage int64 `json:"totalPage"`
}

// SummaryResponse 公共状态页总览
type SummaryResponse struct {
	OverallStatus string           `json:"overallStatus"`
	Services      []ServiceSummary `json:"services"`
	Incidents     []*Incident      `json:"activeIncidents,omitempty"`
	Maintenances  []*Maintenance   `json:"maintenances,omitempty"`
}
type ServiceSummary struct {
	ID int64 `json:"id"`
	// PublicHash 公开访问标识：公开页面的服务详情 URL 使用它，而不是 ID
	PublicHash string  `json:"publicHash"`
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	URL        string  `json:"url"`
	Type       string  `json:"type"`
	Uptime     float64 `json:"uptime"`   // 90-day uptime percentage
	Latency    int     `json:"latency"`  // latest heartbeat latency
	Interval   int     `json:"interval"` // probe interval in seconds
}
type ServiceHistoryResponse struct {
	Service    Service      `json:"service"`
	Uptime     float64      `json:"uptime"`
	Heartbeats []*Heartbeat `json:"heartbeats"`
}

// ServiceDaily 服务每日汇总
type ServiceDaily struct {
	ID            int64  `db:"id" json:"id"`
	ServiceID     int64  `db:"service_id" json:"serviceId"`
	Date          string `db:"date" json:"date"`
	UptimeCount   int    `db:"uptime_count" json:"uptimeCount"`
	DowntimeCount int    `db:"downtime_count" json:"downtimeCount"`
	TotalLatency  int    `db:"total_latency" json:"totalLatency"`
}

// LogEntry 监控日志条目
type LogEntry struct {
	ID          int64  `db:"id" json:"id"`
	ServiceID   int64  `db:"service_id" json:"serviceId"`
	ServiceName string `db:"service_name" json:"serviceName"`
	Status      int    `db:"status" json:"status"`
	Latency     int    `db:"latency" json:"latency"`
	Message     string `db:"message" json:"message"`
	CreatedAt   string `db:"created_at" json:"createdAt"`
}

// DailyStat 每日状态统计（API返回精简格式）
type DailyStat struct {
	Date            string `json:"date"`
	UptimeMinutes   int    `json:"uptimeMinutes"`
	DowntimeMinutes int    `json:"downtimeMinutes"`
}
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}
type SetupRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}
type CreateServiceRequest struct {
	Name           string `json:"name" binding:"required"`
	Description    string `json:"description"`
	URL            string `json:"url" binding:"required"`
	Type           string `json:"type"`
	Interval       int    `json:"interval"`
	SortOrder      int    `json:"sortOrder"`
	ShowOnHomepage *bool  `json:"showOnHomepage"`
	// InsecureSkipVerify 仅对该服务跳过 HTTPS 证书校验
	InsecureSkipVerify *bool `json:"insecureSkipVerify"`
}
type UpdateServiceRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	URL            string `json:"url"`
	Type           string `json:"type"`
	Interval       int    `json:"interval"`
	Status         string `json:"status"`
	IsActive       *bool  `json:"isActive"`
	SortOrder      int    `json:"sortOrder"`
	ShowOnHomepage *bool  `json:"showOnHomepage"`
	// InsecureSkipVerify 仅对该服务跳过 HTTPS 证书校验
	InsecureSkipVerify *bool `json:"insecureSkipVerify"`
}
type ReorderServicesRequest struct {
	Services []ReorderItem `json:"services" binding:"required"`
}
type ReorderItem struct {
	ID        int64 `json:"id"`
	SortOrder int   `json:"sortOrder"`
}

type CreateIncidentRequest struct {
	ServiceID int64  `json:"serviceId" binding:"required"`
	Title     string `json:"title" binding:"required"`
	Impact    string `json:"impact" binding:"required"` // minor, major, critical
	Status    string `json:"status"`
}
type UpdateIncidentRequest struct {
	Impact string `json:"impact"`
	Status string `json:"status"`
	Title  string `json:"title"`
}
type CreateIncidentUpdateRequest struct {
	Status  string `json:"status" binding:"required"` // investigating, identified, monitoring, resolved
	Content string `json:"content" binding:"required"`
}
type CreateMaintenanceRequest struct {
	Title            string `json:"title" binding:"required"`
	Description      string `json:"description"`
	ScheduledStart   string `json:"scheduledStart" binding:"required"`
	ScheduledEnd     string `json:"scheduledEnd" binding:"required"`
	Status           string `json:"status"`
	AffectedServices string `json:"affectedServices"`
	// 周期维护参数（留空即为一次性维护窗口）
	Recurrence         string `json:"recurrence"`
	RecurrenceInterval int    `json:"recurrenceInterval"`
	RecurrenceWeekday  int    `json:"recurrenceWeekday"`
	RecurrenceMonthday int    `json:"recurrenceMonthday"`
	RecurrenceUntil    string `json:"recurrenceUntil"`
}
type UpdateMaintenanceRequest struct {
	Title            string `json:"title"`
	Description      string `json:"description"`
	ScheduledStart   string `json:"scheduledStart"`
	ScheduledEnd     string `json:"scheduledEnd"`
	Status           string `json:"status"`
	AffectedServices string `json:"affectedServices"`
	// 周期维护参数。Recurrence 传空字符串表示改为一次性维护窗口。
	Recurrence         *string `json:"recurrence"`
	RecurrenceInterval *int    `json:"recurrenceInterval"`
	RecurrenceWeekday  *int    `json:"recurrenceWeekday"`
	RecurrenceMonthday *int    `json:"recurrenceMonthday"`
	RecurrenceUntil    *string `json:"recurrenceUntil"`
}

// Subscriber 公开订阅用户
type Subscriber struct {
	ID                 int64  `db:"id" json:"id"`
	Email              string `db:"email" json:"email"`
	Verified           bool   `db:"verified" json:"verified"`
	SubscribedServices string `db:"subscribed_services" json:"subscribedServices"` // 逗号分隔的服务 ID，空=全部
	CreatedAt          string `db:"created_at" json:"createdAt"`
	UpdatedAt          string `db:"updated_at" json:"updatedAt"`
}

type SubscribeRequest struct {
	Email    string  `json:"email" binding:"required"`
	Services []int64 `json:"services,omitempty"` // 要订阅的服务 ID 列表，空=全部
}

// Server DTOs
type CreateServerRequest struct {
	Name               string `json:"name" binding:"required"`
	Description        string `json:"description"`
	AutoMerge          *bool  `json:"autoMerge"`
	AutoMergeThreshold int    `json:"autoMergeThreshold"`
}
type UpdateServerRequest struct {
	Name               string `json:"name"`
	Description        string `json:"description"`
	AutoMerge          *bool  `json:"autoMerge"`
	AutoMergeThreshold int    `json:"autoMergeThreshold"`
}

// ProbeTask DTOs
type CreateProbeTaskRequest struct {
	ServiceID    int64  `json:"serviceId" binding:"required"`
	ServerID     *int64 `json:"serverId"`
	TriggerCount int    `json:"triggerCount"`
}
type UpdateProbeTaskRequest struct {
	ServerID     *int64 `json:"serverId"`
	TriggerCount int    `json:"triggerCount"`
	IsActive     *bool  `json:"isActive"`
}

// Incident merge DTO
type MergeIncidentRequest struct {
	SourceID int64 `json:"sourceId" binding:"required"` // 要合并的源事件 ID
}

// ExportData 导入导出数据结构
type ExportData struct {
	Version         int               `json:"version"`
	ExportedAt      string            `json:"exportedAt"`
	Services        []*Service        `json:"services,omitempty"`
	Incidents       []*Incident       `json:"incidents,omitempty"`
	IncidentUpdates []*IncidentUpdate `json:"incidentUpdates,omitempty"`
	Maintenances    []*Maintenance    `json:"maintenances,omitempty"`
	Settings        map[string]string `json:"settings,omitempty"`
}

// ImportData 导入参数（不含 settings，由 handler 单独处理）
type ImportData struct {
	Services        []*Service        `json:"services,omitempty"`
	Incidents       []*Incident       `json:"incidents,omitempty"`
	IncidentUpdates []*IncidentUpdate `json:"incidentUpdates,omitempty"`
	Maintenances    []*Maintenance    `json:"maintenances,omitempty"`
}
