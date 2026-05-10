package repository

import (
	"context"
	"lumipluse-backend/internal/model"
)

type Repository interface {
	// Service
	CreateService(ctx context.Context, s *model.Service) error
	ListServices(ctx context.Context) ([]*model.Service, error)
	GetService(ctx context.Context, id int64) (*model.Service, error)
	UpdateService(ctx context.Context, s *model.Service) error
	DeleteService(ctx context.Context, id int64) error

	// Heartbeat
	CreateHeartbeat(ctx context.Context, h *model.Heartbeat) error
	GetServiceHistory(ctx context.Context, serviceID int64, days int) ([]*model.Heartbeat, error)
	GetLatestHeartbeat(ctx context.Context, serviceID int64) (*model.Heartbeat, error)
	ListHeartbeats(ctx context.Context, serviceID int64, statusFilter string, page, limit int) ([]*model.LogEntry, int64, error)
	DeleteOldHeartbeats(ctx context.Context, before string) error

	// ServiceDaily
	GetOrCreateServiceDaily(ctx context.Context, serviceID int64, date string) (*model.ServiceDaily, error)
	UpdateServiceDaily(ctx context.Context, d *model.ServiceDaily) error
	GetServiceDailies(ctx context.Context, serviceID int64, days int) ([]*model.ServiceDaily, error)
	DeleteOldServiceDailies(ctx context.Context, before string) error

	// Incident
	CreateIncident(ctx context.Context, inc *model.Incident) error
	GetIncident(ctx context.Context, id int64) (*model.Incident, error)
	ListIncidents(ctx context.Context, page, limit int) ([]*model.Incident, int64, error)
	ListActiveIncidents(ctx context.Context) ([]*model.Incident, error)
	ListServiceIncidents(ctx context.Context, serviceID int64, days int) ([]*model.Incident, error)
	GetActiveIncidentByService(ctx context.Context, serviceID int64) (*model.Incident, error)
	UpdateIncident(ctx context.Context, inc *model.Incident) error
	DeleteIncident(ctx context.Context, id int64) error

	// IncidentUpdate
	CreateIncidentUpdate(ctx context.Context, u *model.IncidentUpdate) error
	ListIncidentUpdates(ctx context.Context, incidentID int64) ([]*model.IncidentUpdate, error)

	// Maintenance
	CreateMaintenance(ctx context.Context, m *model.Maintenance) error
	ListMaintenances(ctx context.Context) ([]*model.Maintenance, error)
	ListActiveMaintenances(ctx context.Context) ([]*model.Maintenance, error)
	GetMaintenance(ctx context.Context, id int64) (*model.Maintenance, error)
	UpdateMaintenance(ctx context.Context, m *model.Maintenance) error
	DeleteMaintenance(ctx context.Context, id int64) error

	// ApiKey
	CreateApiKey(ctx context.Context, k *model.ApiKey) error
	ListApiKeys(ctx context.Context) ([]*model.ApiKey, error)
	GetApiKey(ctx context.Context, id int64) (*model.ApiKey, error)
	GetApiKeyByKey(ctx context.Context, key string) (*model.ApiKey, error)
	UpdateApiKeyLastUsed(ctx context.Context, id int64, ip string) error
	UpdateApiKeyName(ctx context.Context, id int64, name string) error
	DeleteApiKey(ctx context.Context, id int64) error
}
