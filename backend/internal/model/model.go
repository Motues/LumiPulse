package model

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

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
	InsecureSkipVerify bool `db:"insecure_skip_verify" json:"insecureSkipVerify"`
	// TimeoutSeconds 单次探测超时（秒）。0 表示使用默认值 10 秒，
	// 生效范围 1~300 秒。慢接口需要调大，避免被误判为故障。
	TimeoutSeconds int `db:"timeout_seconds" json:"timeoutSeconds"`
	// CertExpiresAt HTTPS 证书到期时间（RFC3339，UTC）。空值表示未知：
	// 非 HTTPS 服务、尚未探测成功，或证书信息还没写回。
	CertExpiresAt string `db:"cert_expires_at" json:"certExpiresAt,omitempty"`
	// CertNotifyLevel 已经发过告警的到期等级（0 / 30 / 7，单位：天）。
	// 只在等级升高时通知一次，避免每次探测都轰炸；证书续期后重置为 0。
	CertNotifyLevel int    `db:"cert_notify_level" json:"-"`
	CreatedAt       string `db:"created_at" json:"createdAt"`
	UpdatedAt       string `db:"updated_at" json:"updatedAt"`

	// --- HTTP 探测高级匹配（仅 type=http 生效）---
	// HTTPMethod 请求方法，空值等价于 GET（保持历史行为）。
	HTTPMethod string `db:"http_method" json:"httpMethod"`
	// HTTPHeaders 自定义请求头，存 JSON 对象（如 {"Authorization":"Bearer xxx"}）。
	// 属于敏感信息：管理端读接口一律返回空串，写入时留空表示「保持原值」。
	HTTPHeaders string `db:"http_headers" json:"httpHeaders"`
	// HTTPBody 请求体纯文本，可配合 POST/PUT 的健康检查接口使用。同样属于敏感信息，
	// 读写语义与 HTTPHeaders 一致。仅在 method 允许携带请求体时发送。
	HTTPBody string `db:"http_body" json:"httpBody"`
	// ExpectStatus 期望状态码模式，支持 200 / 200,301 / 200-299 / 2xx 组合，
	// 用逗号分隔。空值沿用默认判定 200 ≤ status < 400。
	ExpectStatus string `db:"expect_status" json:"expectStatus"`
	// ExpectKeyword 期望响应关键字。非空时响应体必须包含该关键字才算成功，
	// 用于识别「返回 200 但内容是错误页」的情况。
	ExpectKeyword string `db:"expect_keyword" json:"expectKeyword"`
	// FolderID 所属服务分组（服务聚合文件夹）。NULL 表示未分组，服务独立展示。
	// 公开首页会把同一分组下的服务融合成一个条目，展开后再看各服务明细。
	FolderID *int64 `db:"folder_id" json:"folderId,omitempty"`
	// FolderName 关联查询带出的分组名（不落库，仅用于管理端列表回显）
	FolderName string `db:"folder_name" json:"folderName,omitempty"`
	// HomepageBlocks 公开首页「服务详情」要展示的内容块，逗号分隔（见 HomepageBlock* 常量）。
	// 空串 = 全部展示（历史数据默认值）；HomepageBlocksNone = 全部不展示。
	// 只影响公开页面的展示，管理后台始终展示全部内容。
	HomepageBlocks string `db:"homepage_blocks" json:"homepageBlocks"`
}

// MaskServiceSecrets 清空服务里的敏感探测配置，用于管理端响应回显。
// 前端拿不到原值就无法在校验失败时重放，也避免密钥出现在浏览器历史/日志里；
// 写入时空值表示保持原值，因此回显清空不会覆盖已保存的配置。
func (s *Service) MaskServiceSecrets() *Service {
	if s == nil {
		return s
	}
	s.HTTPHeaders = ""
	s.HTTPBody = ""
	return s
}

// --- 公开首页「服务详情」内容块 ---
//
// 每个服务可以单独选择公开首页要展示哪些内容块（探测频率、证书到期、延迟曲线等），
// 管理后台始终展示全部内容，不受这些开关影响。
// 存储为逗号分隔的 key 列表：空串是历史数据的默认值，语义为「全部展示」，
// 因此「什么都不展示」必须用 HomepageBlocksNone 显式表达，不能复用空串。
const (
	// HomepageBlockMetrics 在线率 / 响应时间指标
	HomepageBlockMetrics = "metrics"
	// HomepageBlockInterval 探测频率
	HomepageBlockInterval = "interval"
	// HomepageBlockCert 证书到期
	HomepageBlockCert = "cert"
	// HomepageBlockLatency 最近 24 小时延迟曲线（含均值 / P95 / P99）
	HomepageBlockLatency = "latency"
	// HomepageBlockHeatmap 响应时间热力图
	HomepageBlockHeatmap = "heatmap"
	// HomepageBlockHistory 服务历史矩阵
	HomepageBlockHistory = "history"
)

// HomepageBlocksNone 「全部不展示」的显式标记
const HomepageBlocksNone = "none"

// homepageBlockKeys 全部可配置内容块，顺序即「全部展示」时的输出顺序
var homepageBlockKeys = []string{
	HomepageBlockMetrics,
	HomepageBlockInterval,
	HomepageBlockCert,
	HomepageBlockLatency,
	HomepageBlockHeatmap,
	HomepageBlockHistory,
}

// IsHomepageBlock 判断 key 是否是已知的内容块
func IsHomepageBlock(key string) bool {
	return slices.Contains(homepageBlockKeys, key)
}

// NormalizeHomepageBlocks 规范化服务提交的内容块配置：
// 只保留已知 key 并去重，按固定顺序输出；列表为空时写成 HomepageBlocksNone，
// 避免与「空串 = 全部展示」的历史语义冲突。
func NormalizeHomepageBlocks(raw string) string {
	enabled := make(map[string]bool, len(homepageBlockKeys))
	for _, part := range strings.Split(raw, ",") {
		key := strings.ToLower(strings.TrimSpace(part))
		if IsHomepageBlock(key) {
			enabled[key] = true
		}
	}
	keys := make([]string, 0, len(enabled))
	for _, key := range homepageBlockKeys {
		if enabled[key] {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return HomepageBlocksNone
	}
	return strings.Join(keys, ",")
}

// NormalizeHTTPMethod 归一化请求方法：空值等价于 GET，未知方法返回 false。
func NormalizeHTTPMethod(method string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "":
		return "GET", true
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS":
		return strings.ToUpper(strings.TrimSpace(method)), true
	default:
		return "", false
	}
}

// ParseCustomHeaders 解析自定义请求头 JSON。
// 空串返回 nil（未配置）；格式非法返回错误，调用方应拒绝保存，
// 避免运行期每次探测都因为配置错误而失败。
func ParseCustomHeaders(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return nil, fmt.Errorf("自定义请求头必须是 JSON 对象，例如 {\"Authorization\": \"Bearer xxx\"}")
	}
	return headers, nil
}

// ValidateProbeConfig 校验 HTTP 探测高级配置，返回错误提示（空串表示通过）。
// 管理端保存与运行期解析共用同一套规则，避免「能存进去但探测永远失败」。
func ValidateProbeConfig(method, headers, expectStatus string) string {
	if _, ok := NormalizeHTTPMethod(method); !ok {
		return "请求方法无效，支持 GET/HEAD/POST/PUT/PATCH/DELETE/OPTIONS"
	}
	if _, err := ParseCustomHeaders(headers); err != nil {
		return "自定义请求头格式无效，需要 JSON 对象，例如 {\"Authorization\": \"Bearer xxx\"}"
	}
	if _, ok := ParseExpectStatus(expectStatus); !ok {
		return "期望状态码格式无效，支持 200 / 200,301 / 200-299 / 2xx 组合，用逗号分隔"
	}
	return ""
}

// StatusRange 期望状态码解析出的一段闭区间。
// 统计查询（repository/sqlite）需要按同一规则生成 SQL 判定，因此导出。
type StatusRange struct {
	Min int
	Max int
}

// ParseExpectStatus 解析期望状态码模式，支持逗号分隔的单值（200）、区间（200-299）
// 与 x 通配（2xx）任意组合，例如 "200,301-303,5xx"。
// 返回 nil 表示未配置（沿用默认判定 200 ≤ status < 400）；
// 第二个返回值为 false 表示模式非法，调用方应拒绝保存。
func ParseExpectStatus(pattern string) ([]StatusRange, bool) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, true
	}

	ranges := make([]StatusRange, 0, 4)
	for _, part := range strings.Split(strings.ToUpper(pattern), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// 通配写法：2xx
		if len(part) == 3 && (part[1] == 'X' && part[2] == 'X') {
			digit, err := strconv.Atoi(part[:1])
			if err != nil {
				return nil, false
			}
			ranges = append(ranges, StatusRange{Min: digit * 100, Max: digit*100 + 99})
			continue
		}
		// 区间写法：200-299
		if lo, hi, found := strings.Cut(part, "-"); found {
			minVal, err1 := strconv.Atoi(strings.TrimSpace(lo))
			maxVal, err2 := strconv.Atoi(strings.TrimSpace(hi))
			if err1 != nil || err2 != nil || !validHTTPStatus(minVal) || !validHTTPStatus(maxVal) || minVal > maxVal {
				return nil, false
			}
			ranges = append(ranges, StatusRange{Min: minVal, Max: maxVal})
			continue
		}
		// 单值写法：200
		code, err := strconv.Atoi(part)
		if err != nil || !validHTTPStatus(code) {
			return nil, false
		}
		ranges = append(ranges, StatusRange{Min: code, Max: code})
	}

	if len(ranges) == 0 {
		return nil, false
	}
	return ranges, true
}

// validHTTPStatus 合法的 HTTP 状态码范围
func validHTTPStatus(code int) bool {
	return code >= 100 && code <= 599
}

// MatchesExpectStatus 判断状态码是否符合期望模式。未配置模式时沿用默认判定
// （200 ≤ status < 400），保证既有服务的判定口径不变。
func (s *Service) MatchesExpectStatus(status int) bool {
	ranges, ok := ParseExpectStatus(s.ExpectStatus)
	if !ok || len(ranges) == 0 {
		return status >= 200 && status < 400
	}
	for _, r := range ranges {
		if status >= r.Min && status <= r.Max {
			return true
		}
	}
	return false
}

// MatchesBody 判断响应体是否符合期望关键字。未配置关键字时一律通过。
func (s *Service) MatchesBody(body string) bool {
	keyword := strings.TrimSpace(s.ExpectKeyword)
	if keyword == "" {
		return true
	}
	return strings.Contains(body, keyword)
}

// MatchesProbe 判断一次 HTTP 探测是否成功：状态码与响应关键字都满足才算成功。
// 成功判定集中在这里，检查器与各处统计口径都由它统一。
func (s *Service) MatchesProbe(status int, body string) bool {
	return s.MatchesExpectStatus(status) && s.MatchesBody(body)
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
	PublicHash string `json:"publicHash"`
	// Days 每天一个四元组 [upCount, downCount, statusCode, maintenanceCount]。
	// maintenanceCount 是该天内落在维护窗口内的失败探测次数：不计入可用率，
	// 前端把这些「计划内停机」用蓝色而不是红色展示。
	Days [][4]int `json:"days"`
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
	// --- 事后复盘（Postmortem）---
	// RootCause 根因描述、Resolution 处理措施、PostmortemURL 复盘文档链接。
	RootCause     string `db:"root_cause" json:"rootCause,omitempty"`
	Resolution    string `db:"resolution" json:"resolution,omitempty"`
	PostmortemURL string `db:"postmortem_url" json:"postmortemUrl,omitempty"`
	// PostmortemPublic 是否允许在公开页面展示上述复盘内容（默认不公开）。
	// 关闭时公开接口会清空这三个字段，避免内部根因描述不小心外泄。
	PostmortemPublic bool `db:"postmortem_public" json:"postmortemPublic"`
	// --- 告警升级（未确认则升级通知）---
	// Acknowledged 是否已被人工确认。确认表示「已有人接手」，
	// 未确认且超过升级时限的活跃事件会再次通知（见 checker 的 escalateIncidents）。
	Acknowledged bool `db:"acknowledged" json:"acknowledged"`
	// AcknowledgedAt 确认时间（RFC3339）。未确认时为空。
	AcknowledgedAt string `db:"acknowledged_at" json:"acknowledgedAt,omitempty"`
	// AcknowledgedBy 确认人（管理员用户名）。当前为单管理员模型，仅作留痕。
	AcknowledgedBy string `db:"acknowledged_by" json:"acknowledgedBy,omitempty"`
	// EscalationCount 已发送的升级通知次数，避免同一事件反复升级轰炸。
	EscalationCount int `db:"escalation_count" json:"escalationCount"`
	// LastEscalatedAt 上次升级通知时间（RFC3339）。空值表示尚未升级过。
	LastEscalatedAt string `db:"last_escalated_at" json:"lastEscalatedAt,omitempty"`
	// Joined fields
	Updates  []*IncidentUpdate `db:"-" json:"updates,omitempty"`
	Children []*Incident       `db:"-" json:"children,omitempty"`
}

// IsActive 事件是否仍在处理中（未解决）。
func (i *Incident) IsActive() bool {
	return i != nil && i.Status != "resolved"
}

// --- 服务分组（服务聚合文件夹）---

// ServiceFolder 服务分组：把多个服务聚合在一起，公开首页融合展示为一个条目。
// 分组只影响展示口径，不影响探测、事件与 SLA 统计（那些仍然按服务计算）。
type ServiceFolder struct {
	ID          int64  `db:"id" json:"id"`
	Name        string `db:"name" json:"name"`
	Description string `db:"description" json:"description,omitempty"`
	// ShowOnHomepage 是否在公开首页展示。关闭时该分组下的服务也不会单独展示。
	ShowOnHomepage bool   `db:"show_on_homepage" json:"showOnHomepage"`
	SortOrder      int    `db:"sort_order" json:"sortOrder"`
	CreatedAt      string `db:"created_at" json:"createdAt"`
	UpdatedAt      string `db:"updated_at" json:"updatedAt"`
	// Services 分组下的服务（仅接口响应的聚合结果里填充，不落库）
	Services []*Service `db:"-" json:"services,omitempty"`
}

// FolderSummary 服务分组在公开首页上的聚合结果。
// 可用率与延迟按「探测次数加权」融合，避免各服务探测频率不同导致偏差。
type FolderSummary struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Status 聚合状态：全部正常 → operational，全部故障 → outage，其余 → degraded
	Status string `json:"status"`
	// Uptime 融合可用率（0~100，按探测次数加权）
	Uptime float64 `json:"uptime"`
	// Latency 融合响应时间（ms，仅成功样本按次数加权）
	Latency int `json:"latency"`
	// Services 分组内的服务明细（展开下拉时展示），聚合口径与 ServiceSummary 一致
	Services []ServiceSummary `json:"services"`
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
	// Reminded 是否已发送过「即将开始」提前提醒。推进到下一个周期时重置为 false，
	// 保证每个窗口最多提醒一次。
	Reminded bool `db:"reminded" json:"reminded"`
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
	// Scope 权限范围：read 只允许 GET/HEAD，write 允许写操作。
	// 高危操作（删除服务 / 导入覆盖数据 / 改管理员账号）两种范围都不允许。
	Scope string `db:"scope" json:"scope"`
	// RateLimitPerMinute 每分钟请求上限，0 表示不限制
	RateLimitPerMinute int    `db:"rate_limit_per_minute" json:"rateLimitPerMinute"`
	CreatedAt          string `db:"created_at" json:"createdAt"`
}
type CreateApiKeyRequest struct {
	Name string `json:"name" binding:"required"`
	// Scope 留空时按 read 处理（最小权限）
	Scope              string `json:"scope"`
	ExpiresAt          string `json:"expiresAt"` // 空值=永久有效
	RateLimitPerMinute int    `json:"rateLimitPerMinute"`
}

// UpdateApiKeyRequest 更新密钥。Scope / RateLimitPerMinute 用指针区分「未传」与「显式改值」。
type UpdateApiKeyRequest struct {
	Name               string  `json:"name" binding:"required"`
	Scope              *string `json:"scope"`
	RateLimitPerMinute *int    `json:"rateLimitPerMinute"`
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
	// Folders 服务聚合分组：把属于同一分组的服务融合成一个条目，
	// 前端可用它替代分组内服务的独立展示，展开后再看 services 里的明细。
	Folders      []FolderSummary `json:"folders,omitempty"`
	Incidents    []*Incident     `json:"activeIncidents,omitempty"`
	Maintenances []*Maintenance  `json:"maintenances,omitempty"`
}
type ServiceSummary struct {
	ID int64 `json:"id"`
	// PublicHash 公开访问标识：公开页面的服务详情 URL 使用它，而不是 ID
	PublicHash string `json:"publicHash"`
	// FolderID 所属服务分组（nil 表示未分组），公开首页据此把服务聚合成一个条目
	FolderID *int64  `json:"folderId,omitempty"`
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	URL      string  `json:"url"`
	Type     string  `json:"type"`
	Uptime   float64 `json:"uptime"`   // 90-day uptime percentage
	Latency  int     `json:"latency"`  // latest heartbeat latency
	Interval int     `json:"interval"` // probe interval in seconds
	// CertExpiresAt HTTPS 证书到期时间（RFC3339）；空值表示无证书信息
	CertExpiresAt string `json:"certExpiresAt,omitempty"`
	// HomepageBlocks 公开首页「服务详情」要展示的内容块（逗号分隔，见 HomepageBlock*）。
	// 空串 = 全部展示。
	HomepageBlocks string `json:"homepageBlocks"`
}
type ServiceHistoryResponse struct {
	Service    Service      `json:"service"`
	Uptime     float64      `json:"uptime"`
	Heartbeats []*Heartbeat `json:"heartbeats"`
}

// LatencyHeatmapCell 延迟热力图的一个格子（某天某小时，北京时间）。
// 统计口径与延迟分桶接口一致：avg 只统计成功样本，failures 单独计数。
type LatencyHeatmapCell struct {
	Day      string `json:"day"`      // YYYY-MM-DD（北京时间）
	Hour     int    `json:"hour"`     // 0~23（北京时间）
	Avg      int    `json:"avg"`      // 成功样本的平均延迟（ms）
	Samples  int    `json:"samples"`  // 参与统计的成功样本数
	Failures int    `json:"failures"` // 该小时内的失败探测次数
	// Maintenance 该小时是否落在维护窗口内：是则失败探测用蓝色（计划内停机）
	// 而不是红色（真实故障）展示。
	Maintenance bool `json:"maintenance,omitempty"`
}

// LatencyHeatmapResponse 「日期 × 小时」聚合结果。
// 只返回有数据的小时（稀疏），前端按 from~to 补齐空格子。
type LatencyHeatmapResponse struct {
	From   string                `json:"from"`   // 起始日期（含）
	To     string                `json:"to"`     // 结束日期（含）
	Days   int                   `json:"days"`   // 覆盖天数
	MaxAvg int                   `json:"maxAvg"` // 所有格子的最大平均延迟，前端据此定色阶
	Cells  []*LatencyHeatmapCell `json:"cells"`
}

// ServiceDaily 服务每日汇总
type ServiceDaily struct {
	ID            int64  `db:"id" json:"id"`
	ServiceID     int64  `db:"service_id" json:"serviceId"`
	Date          string `db:"date" json:"date"`
	UptimeCount   int    `db:"uptime_count" json:"uptimeCount"`
	DowntimeCount int    `db:"downtime_count" json:"downtimeCount"`
	TotalLatency  int    `db:"total_latency" json:"totalLatency"`
	// MaintenanceCount 维护窗口内失败的探测次数。这类失败不属于真实故障，
	// 因此不计入 DowntimeCount（可用率口径不变），只用于把矩阵里的
	// 「计划内停机」标成蓝色。
	MaintenanceCount int `db:"maintenance_count" json:"maintenanceCount"`
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
	// IsSuccess 后端按服务自己的判定口径（期望状态码 + 期望关键字）给出的结论，
	// 前端不再自行按 status 猜测，避免与筛选/统计口径不一致。
	IsSuccess bool `db:"is_success" json:"isSuccess"`
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
	// TimeoutSeconds 单次探测超时（秒），0 或省略表示使用默认值
	TimeoutSeconds *int `json:"timeoutSeconds"`
	// --- HTTP 探测高级匹配（留空一律维持默认行为）---
	HTTPMethod    string `json:"httpMethod"`
	HTTPHeaders   string `json:"httpHeaders"`
	HTTPBody      string `json:"httpBody"`
	ExpectStatus  string `json:"expectStatus"`
	ExpectKeyword string `json:"expectKeyword"`
	// FolderID 所属服务分组（服务聚合文件夹）。0 / 省略表示未分组。
	FolderID *int64 `json:"folderId"`
	// HomepageBlocks 公开首页「服务详情」展示的内容块，逗号分隔（见 HomepageBlock*）。
	// 省略 / 空串表示全部展示。
	HomepageBlocks *string `json:"homepageBlocks"`
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
	// TimeoutSeconds 单次探测超时（秒），0 表示使用默认值
	TimeoutSeconds *int `json:"timeoutSeconds"`
	// --- HTTP 探测高级匹配 ---
	// 这四个字段用指针区分「未传」与「显式清空」：传空串表示清空该配置，
	// 不传则保持原值。请求头/请求体属于敏感信息，管理端不会回显原值。
	HTTPMethod    *string `json:"httpMethod"`
	HTTPHeaders   *string `json:"httpHeaders"`
	HTTPBody      *string `json:"httpBody"`
	ExpectStatus  *string `json:"expectStatus"`
	ExpectKeyword *string `json:"expectKeyword"`
	// FolderID 所属服务分组。指针语义：不传表示保持原值，传 0 / null 表示取消分组。
	FolderID *int64 `json:"folderId"`
	// HomepageBlocks 公开首页「服务详情」展示的内容块。指针语义：不传表示保持原值，
	// 传空串表示「全部不展示」（规范化后落库为 HomepageBlocksNone）。
	HomepageBlocks *string `json:"homepageBlocks"`
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
	// 复盘字段用指针区分「未传」与「显式清空」：传空字符串表示清空该字段。
	// 不这样做就没法撤回已经写错的根因描述。
	RootCause        *string `json:"rootCause"`
	Resolution       *string `json:"resolution"`
	PostmortemURL    *string `json:"postmortemUrl"`
	PostmortemPublic *bool   `json:"postmortemPublic"`
	// Acknowledged 人工确认开关。指针区分「未传」与「显式取消确认」；
	// 置为 true 时记录确认时间与确认人，置为 false 会清空确认信息并重新允许升级。
	Acknowledged *bool `json:"acknowledged"`
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

// ServiceFolder DTOs（服务分组 / 服务聚合文件夹）
type CreateServiceFolderRequest struct {
	Name           string `json:"name" binding:"required"`
	Description    string `json:"description"`
	ShowOnHomepage *bool  `json:"showOnHomepage"`
	SortOrder      int    `json:"sortOrder"`
}
type UpdateServiceFolderRequest struct {
	// 指针语义：nil 表示不修改
	Name           *string `json:"name"`
	Description    *string `json:"description"`
	ShowOnHomepage *bool   `json:"showOnHomepage"`
	SortOrder      *int    `json:"sortOrder"`
}

// AssignServiceFolderRequest folderId 传 null 或 0 表示取消分组
type AssignServiceFolderRequest struct {
	FolderID *int64 `json:"folderId"`
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
	// ServiceFolders 服务分组。导出时不写库内自增 ID，而写「导出内引用号」
	// （从 1 开始的紧凑序号），服务的 FolderID 指向同一个引用号；
	// 导入时按引用号重新映射到新分组 ID（详见 sqlite.ImportFullData）。
	ServiceFolders []*ServiceFolder `json:"serviceFolders,omitempty"`
}

// ImportData 导入参数（不含 settings，由 handler 单独处理）
type ImportData struct {
	Services        []*Service        `json:"services,omitempty"`
	Incidents       []*Incident       `json:"incidents,omitempty"`
	IncidentUpdates []*IncidentUpdate `json:"incidentUpdates,omitempty"`
	Maintenances    []*Maintenance    `json:"maintenances,omitempty"`
	ServiceFolders  []*ServiceFolder  `json:"serviceFolders,omitempty"`
}
