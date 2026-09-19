package repository

import (
	"context"
	"lumipluse-backend/internal/model"
)

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
	ListActiveIncidents(ctx context.Context) ([]*model.Incident, error)
	ListServiceIncidents(ctx context.Context, serviceID int64, days int) ([]*model.Incident, error)
	ListIncidentsSince(ctx context.Context, days int) ([]*model.Incident, error)
	CountRecentIncidents(ctx context.Context, days int) (total int64, resolved int64, err error)
	GetActiveIncidentByService(ctx context.Context, serviceID int64) (*model.Incident, error)
	UpdateIncident(ctx context.Context, inc *model.Incident) error
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
	// Data export/import
	ImportFullData(ctx context.Context, data *model.ImportData) error
}
