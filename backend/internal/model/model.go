package model
// Service 监控服务/节点
type Service struct {
	ID              int64  `db:"id" json:"id"`
	Name            string `db:"name" json:"name"`
	Description     string `db:"description" json:"description,omitempty"`
	URL             string `db:"url" json:"url"`
	Type            string `db:"type" json:"type"` // http, tcp, ping
	Interval        int    `db:"interval" json:"interval"`
	Status          string `db:"status" json:"status"` // operational, degraded, outage
	IsActive        bool   `db:"is_active" json:"isActive"`
	SortOrder       int    `db:"sort_order" json:"sortOrder"`
	ShowOnHomepage  bool   `db:"show_on_homepage" json:"showOnHomepage"`
	CreatedAt       string `db:"created_at" json:"createdAt"`
	UpdatedAt       string `db:"updated_at" json:"updatedAt"`
}
// Heartbeat 健康检查记录
type Heartbeat struct {
	ID        int64  `db:"id" json:"id"`
	ServiceID int64  `db:"service_id" json:"serviceId"`
	Status    int    `db:"status" json:"status"`     // HTTP status code or 1/0 for tcp/ping
	Latency   int    `db:"latency" json:"latency"`    // milliseconds
	Message   string `db:"message" json:"message,omitempty"` // error detail or response summary
	CreatedAt string `db:"created_at" json:"createdAt"`
}

// LatencyResponse 紧凑延迟数据响应
type LatencyResponse struct {
	Start    string `json:"start"`              // 起始时间 ISO
	Interval int    `json:"interval"`            // 间隔分钟数
	Latencies []int  `json:"latencies"`          // 延迟数组 (ms)
	Statuses  []int  `json:"statuses"`           // 状态数组 (0=正常, 1=故障, -1=无数据)
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
	ID               int64       `db:"id" json:"id"`
	ServiceID        int64       `db:"service_id" json:"serviceId"`
	Title            string      `db:"title" json:"title"`
	Impact           string      `db:"impact" json:"impact"`           // minor, major, critical
	Status           string      `db:"status" json:"status"`           // investigating, identified, monitoring, resolved
	AffectedServices string      `db:"affected_services" json:"affectedServices"` // 逗号分隔的服务 ID
	ParentID         *int64      `db:"parent_id" json:"parentId,omitempty"`
	CreatedAt        string      `db:"created_at" json:"createdAt"`
	UpdatedAt        string      `db:"updated_at" json:"updatedAt"`
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
	Status           string `db:"status" json:"status"` // scheduled, in_progress, completed, cancelled
	AffectedServices string `db:"affected_services" json:"affectedServices"` // JSON string or comma-separated IDs
	CreatedAt        string `db:"created_at" json:"createdAt"`
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
	ID          int64  `db:"id" json:"id"`
	Name        string `db:"name" json:"name"`
	Key         string `db:"key" json:"key,omitempty"`          // 完整密钥，仅创建时返回
	KeyPrefix   string `db:"key_prefix" json:"keyPrefix"`       // 前8位用于显示区分
	ExpiresAt   string `db:"expires_at" json:"expiresAt"`       // 空值=永久有效
	LastUsedAt  string `db:"last_used_at" json:"lastUsedAt,omitempty"`
	LastUsedIP  string `db:"last_used_ip" json:"lastUsedIP,omitempty"`
	IsActive    bool   `db:"is_active" json:"isActive"`
	CreatedAt   string `db:"created_at" json:"createdAt"`
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
	OverallStatus string             `json:"overallStatus"`
	Services      []ServiceSummary   `json:"services"`
	Incidents     []*Incident        `json:"activeIncidents,omitempty"`
	Maintenances  []*Maintenance     `json:"maintenances,omitempty"`
}
type ServiceSummary struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	URL      string  `json:"url"`
	Type     string  `json:"type"`
	Uptime   float64 `json:"uptime"` // 90-day uptime percentage
	Latency  int     `json:"latency"`  // latest heartbeat latency
	Interval int     `json:"interval"` // probe interval in seconds
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
	Name            string `json:"name" binding:"required"`
	Description     string `json:"description"`
	URL             string `json:"url" binding:"required"`
	Type            string `json:"type"`
	Interval        int    `json:"interval"`
	SortOrder       int    `json:"sortOrder"`
	ShowOnHomepage  *bool  `json:"showOnHomepage"`
}
type UpdateServiceRequest struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	URL             string `json:"url"`
	Type            string `json:"type"`
	Interval        int    `json:"interval"`
	Status          string `json:"status"`
	IsActive        *bool  `json:"isActive"`
	SortOrder       int    `json:"sortOrder"`
	ShowOnHomepage  *bool  `json:"showOnHomepage"`
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
}
type UpdateMaintenanceRequest struct {
	Title            string `json:"title"`
	Description      string `json:"description"`
	ScheduledStart   string `json:"scheduledStart"`
	ScheduledEnd     string `json:"scheduledEnd"`
	Status           string `json:"status"`
	AffectedServices string `json:"affectedServices"`
}

// Subscriber 公开订阅用户
type Subscriber struct {
	ID                int64  `db:"id" json:"id"`
	Email             string `db:"email" json:"email"`
	Verified          bool   `db:"verified" json:"verified"`
	SubscribedServices string `db:"subscribed_services" json:"subscribedServices"` // 逗号分隔的服务 ID，空=全部
	CreatedAt         string `db:"created_at" json:"createdAt"`
	UpdatedAt         string `db:"updated_at" json:"updatedAt"`
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
	Version         int                `json:"version"`
	ExportedAt      string             `json:"exportedAt"`
	Services        []*Service         `json:"services,omitempty"`
	Incidents       []*Incident        `json:"incidents,omitempty"`
	IncidentUpdates []*IncidentUpdate  `json:"incidentUpdates,omitempty"`
	Maintenances    []*Maintenance     `json:"maintenances,omitempty"`
	Settings        map[string]string  `json:"settings,omitempty"`
}

// ImportData 导入参数（不含 settings，由 handler 单独处理）
type ImportData struct {
	Services        []*Service        `json:"services,omitempty"`
	Incidents       []*Incident       `json:"incidents,omitempty"`
	IncidentUpdates []*IncidentUpdate `json:"incidentUpdates,omitempty"`
	Maintenances    []*Maintenance    `json:"maintenances,omitempty"`
}
