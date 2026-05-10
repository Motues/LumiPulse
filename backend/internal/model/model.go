package model
// Service 监控服务/节点
type Service struct {
	ID          int64  `db:"id" json:"id"`
	Name        string `db:"name" json:"name"`
	Description string `db:"description" json:"description,omitempty"`
	URL         string `db:"url" json:"url"`
	Type        string `db:"type" json:"type"` // http, tcp, ping
	Interval    int    `db:"interval" json:"interval"`
	Status      string `db:"status" json:"status"` // operational, degraded, outage
	IsActive    bool   `db:"is_active" json:"isActive"`
	SortOrder   int    `db:"sort_order" json:"sortOrder"`
	CreatedAt   string `db:"created_at" json:"createdAt"`
	UpdatedAt   string `db:"updated_at" json:"updatedAt"`
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
// Incident 故障事件
type Incident struct {
	ID        int64  `db:"id" json:"id"`
	ServiceID int64  `db:"service_id" json:"serviceId"`
	Title     string `db:"title" json:"title"`
	Impact    string `db:"impact" json:"impact"` // minor, major, critical
	Status    string `db:"status" json:"status"` // investigating, identified, monitoring, resolved
	CreatedAt string `db:"created_at" json:"createdAt"`
	UpdatedAt string `db:"updated_at" json:"updatedAt"`
	// Joined fields
	Updates []*IncidentUpdate `db:"-" json:"updates,omitempty"`
}
// IncidentUpdate 故障事件进展更新
type IncidentUpdate struct {
	ID         int64  `db:"id" json:"id"`
	IncidentID int64  `db:"incident_id" json:"incidentId"`
	Status     string `db:"status" json:"status"`
	Content    string `db:"content" json:"content"`
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
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	URL         string `json:"url" binding:"required"`
	Type        string `json:"type"`
	Interval    int    `json:"interval"`
	SortOrder   int    `json:"sortOrder"`
}
type UpdateServiceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Type        string `json:"type"`
	Interval    int    `json:"interval"`
	Status      string `json:"status"`
	IsActive    *bool  `json:"isActive"`
	SortOrder   int    `json:"sortOrder"`
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
