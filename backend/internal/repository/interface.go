package repository

import (
	"context"
	"errors"
	"lumipluse-backend/internal/model"
)

// ErrIncidentConflict 事件的前置条件在事务内不再成立：
// 已解决、已经是子事件、或合并会形成循环父子。
// 合并/拆分都是「读-改-写」，必须在同一个事务里重新校验，
// 否则并发请求（两个管理员、双击提交、脚本调用）可能把 A 挂到 B 下、
// 同时又把 B 挂到 A 下，产生两个都不再是根事件的循环父子。
var ErrIncidentConflict = errors.New("incident state conflict")

type Repository interface {
	// Ping 检查数据库连通性（readiness 探针使用）
	Ping(ctx context.Context) error

	// Service
	CreateService(ctx context.Context, s *model.Service) error
	ListServices(ctx context.Context) ([]*model.Service, error)
	GetService(ctx context.Context, id int64) (*model.Service, error)
	GetServiceByHash(ctx context.Context, hash string) (*model.Service, error)
	UpdateService(ctx context.Context, s *model.Service) error
	UpdateServiceSortOrder(ctx context.Context, id int64, sortOrder int) error
	// UpdateServiceCert 只写回证书到期信息与告警等级，避免覆盖并发的状态变更
	UpdateServiceCert(ctx context.Context, id int64, certExpiresAt string, notifyLevel int) error
	DeleteService(ctx context.Context, id int64) error

	// Heartbeat
	CreateHeartbeat(ctx context.Context, h *model.Heartbeat) error
	GetServiceHistory(ctx context.Context, serviceID int64, days int) ([]*model.Heartbeat, error)
	GetLatestHeartbeat(ctx context.Context, serviceID int64) (*model.Heartbeat, error)
	BatchGetLatestHeartbeats(ctx context.Context, serviceIDs []int64) (map[int64]*model.Heartbeat, error)
	GetLatencyBuckets(ctx context.Context, serviceID int64, since string, bucketSeconds, maxBucket int) ([]*model.LatencyBucket, error)
	GetLatencyStats(ctx context.Context, serviceID int64, since string) (*model.LatencyStats, error)
	// GetLatencyHeatmap 按「日期 × 小时」（北京时间）聚合延迟，供热力图使用
	GetLatencyHeatmap(ctx context.Context, serviceID int64, since string) ([]*model.LatencyHeatmapCell, error)
	ListHeartbeats(ctx context.Context, serviceID int64, statusFilter string, page, limit int) ([]*model.LogEntry, int64, error)
	DeleteOldHeartbeats(ctx context.Context, before string) error

	// ServiceDaily
	GetOrCreateServiceDaily(ctx context.Context, serviceID int64, date string) (*model.ServiceDaily, error)
	UpdateServiceDaily(ctx context.Context, d *model.ServiceDaily) error
	GetServiceDailies(ctx context.Context, serviceID int64, days int) ([]*model.ServiceDaily, error)
	BatchGetServiceDailies(ctx context.Context, serviceIDs []int64, days int) (map[int64][]*model.ServiceDaily, error)
	DeleteOldServiceDailies(ctx context.Context, before string) error

	// ServiceMonthly（月度 SLA 月报）
	AggregateServiceMonthly(ctx context.Context, from, toExclusive string) ([]*model.ServiceMonthly, error)
	ListServiceDailiesBetween(ctx context.Context, from, toExclusive string) (map[int64][]*model.ServiceDaily, error)
	CountCoveredDaysBetween(ctx context.Context, from, toExclusive string) (map[int64]int, error)
	AggregateIncidentDowntime(ctx context.Context, from, toExclusive string) (map[int64]int, error)
	CountIncidentsBetween(ctx context.Context, from, toExclusive string) (map[int64]int, error)
	GetServiceMonthly(ctx context.Context, serviceID int64, month string) (*model.ServiceMonthly, error)
	ListServiceMonthlies(ctx context.Context, month string) ([]*model.ServiceMonthly, error)
	UpsertServiceMonthly(ctx context.Context, m *model.ServiceMonthly, incremental bool) error
	SetServiceMonthlyClosed(ctx context.Context, serviceID int64, month, closedAt string) error

	// Incident
	CreateIncident(ctx context.Context, inc *model.Incident) error
	GetIncident(ctx context.Context, id int64) (*model.Incident, error)
	GetIncidentByHash(ctx context.Context, hash string) (*model.Incident, error)
	ListIncidents(ctx context.Context, page, limit int) ([]*model.Incident, int64, error)
	// ListIncidentsWithChildren 管理端事件列表：根事件与子事件都返回。
	// 自动合并产生的子事件不在公开列表展示，但管理端必须能看到，
	// 否则会出现"邮件告警了、事件管理里却找不到"的情况。
	ListIncidentsWithChildren(ctx context.Context, page, limit int) ([]*model.Incident, int64, error)
	ListActiveIncidents(ctx context.Context) ([]*model.Incident, error)
	ListServiceIncidents(ctx context.Context, serviceID int64, days int) ([]*model.Incident, error)
	ListIncidentsSince(ctx context.Context, days int) ([]*model.Incident, error)
	CountRecentIncidents(ctx context.Context, days int) (total int64, resolved int64, err error)
	GetActiveIncidentByService(ctx context.Context, serviceID int64) (*model.Incident, error)
	UpdateIncident(ctx context.Context, inc *model.Incident) error
	// MarkIncidentEscalated 只更新升级计数与时间，避免用陈旧快照整行覆盖
	// 状态/父子关系（否则可能把刚刚解决的事件又写回 investigating）。
	// 返回 false 表示事件已解决，不应再发送升级通知。
	MarkIncidentEscalated(ctx context.Context, id int64, count int, lastEscalatedAt string) (bool, error)
	DeleteIncident(ctx context.Context, id int64) error

	// IncidentUpdate
	CreateIncidentUpdate(ctx context.Context, u *model.IncidentUpdate) error
	ListIncidentUpdates(ctx context.Context, incidentID int64) ([]*model.IncidentUpdate, error)
	BatchListIncidentUpdates(ctx context.Context, incidentIDs []int64) (map[int64][]*model.IncidentUpdate, error)
	UpdateIncidentUpdate(ctx context.Context, u *model.IncidentUpdate) error
	DeleteIncidentUpdate(ctx context.Context, id int64) error

	// Maintenance
	CreateMaintenance(ctx context.Context, m *model.Maintenance) error
	ListMaintenances(ctx context.Context) ([]*model.Maintenance, error)
	ListActiveMaintenances(ctx context.Context) ([]*model.Maintenance, error)
	ListActiveMaintenancesByService(ctx context.Context, serviceID int64) ([]*model.Maintenance, error)
	GetMaintenance(ctx context.Context, id int64) (*model.Maintenance, error)
	UpdateMaintenance(ctx context.Context, m *model.Maintenance) error
	DeleteMaintenance(ctx context.Context, id int64) error

	// ApiKey
	CreateApiKey(ctx context.Context, k *model.ApiKey) error
	ListApiKeys(ctx context.Context) ([]*model.ApiKey, error)
	GetApiKey(ctx context.Context, id int64) (*model.ApiKey, error)
	GetApiKeyByKey(ctx context.Context, key string) (*model.ApiKey, error)
	UpdateApiKeyLastUsed(ctx context.Context, id int64, ip string) error
	// UpdateApiKey 更新名称、权限范围与每分钟限流
	UpdateApiKey(ctx context.Context, id int64, name, scope string, rateLimitPerMinute int) error
	DeleteApiKey(ctx context.Context, id int64) error

	// Subscriber
	CreateSubscriber(ctx context.Context, email string, services string) (*model.Subscriber, error)
	ListSubscribers(ctx context.Context) ([]*model.Subscriber, error)
	DeleteSubscriber(ctx context.Context, id int64) error
	GetSubscriberByEmail(ctx context.Context, email string) (*model.Subscriber, error)
	UpdateSubscriberServices(ctx context.Context, email string, services string) error

	// Server
	CreateServer(ctx context.Context, s *model.Server) error
	ListServers(ctx context.Context) ([]*model.Server, error)
	GetServer(ctx context.Context, id int64) (*model.Server, error)
	UpdateServer(ctx context.Context, s *model.Server) error
	DeleteServer(ctx context.Context, id int64) error

	// ServiceFolder 服务分组（服务聚合文件夹）
	CreateServiceFolder(ctx context.Context, f *model.ServiceFolder) error
	ListServiceFolders(ctx context.Context) ([]*model.ServiceFolder, error)
	GetServiceFolder(ctx context.Context, id int64) (*model.ServiceFolder, error)
	UpdateServiceFolder(ctx context.Context, f *model.ServiceFolder) error
	DeleteServiceFolder(ctx context.Context, id int64) error
	CountServicesInFolder(ctx context.Context, id int64) (int, error)
	AssignServiceFolder(ctx context.Context, serviceID int64, folderID *int64) error

	// ProbeTask
	CreateProbeTask(ctx context.Context, t *model.ProbeTask) error
	ListProbeTasks(ctx context.Context) ([]*model.ProbeTask, error)
	GetProbeTask(ctx context.Context, id int64) (*model.ProbeTask, error)
	GetProbeTaskByService(ctx context.Context, serviceID int64) (*model.ProbeTask, error)
	UpdateProbeTask(ctx context.Context, t *model.ProbeTask) error
	DeleteProbeTask(ctx context.Context, id int64) error
	ListProbeTasksByServer(ctx context.Context, serverID int64) ([]*model.ProbeTask, error)

	// Incident - extended
	ListIncidentsByServer(ctx context.Context, serverID int64) ([]*model.Incident, error)
	ListChildIncidents(ctx context.Context, parentID int64) ([]*model.Incident, error)
	UpdateChildrenStatus(ctx context.Context, parentID int64, status string) error
	// MergeIncidents 原子地把 source 挂为 target 的子事件：先校验双方都是未解决的
	// 根事件，再在同一事务里写父子关系、把 source 的整棵子树统一挂到 target 下，
	// 并把受影响服务取并集。并发合并的第二个事务会因条件更新命中 0 行而回滚，
	// 因此不会出现 A→B 与 B→A 同时成立。
	MergeIncidents(ctx context.Context, targetID, sourceID int64) error
	// SplitIncident 原子地把子事件从父事件拆出来，并从父事件的 affected_services
	// 中移除只有该子事件还在声明的服务。返回原父事件 ID。
	SplitIncident(ctx context.Context, id int64) (int64, error)
	// BreakIncidentCycles 修复历史脏数据里的循环父子（A→B 且 B→A）。
	// 这类事件的 parent_id 都不为空，列表与仪表盘会把它们全部隐藏，
	// 且检查器永远无法把它们解决。返回修复的环数量。
	BreakIncidentCycles(ctx context.Context) (int, error)
	// Data export/import
	ImportFullData(ctx context.Context, data *model.ImportData) error
}
