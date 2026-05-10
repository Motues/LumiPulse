package sqlite

import (
	"context"
	"lumipluse-backend/internal/model"
	"time"
)

func (r *repo) CreateService(ctx context.Context, s *model.Service) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	query := `INSERT INTO Service (name, description, url, type, interval, status, is_active, sort_order, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, s.Name, s.Description, s.URL, s.Type, s.Interval,
		"operational", true, s.SortOrder, now, now)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	s.ID = id
	s.CreatedAt = now
	s.UpdatedAt = now
	s.Status = "operational"
	s.IsActive = true
	return nil
}

func (r *repo) ListServices(ctx context.Context) ([]*model.Service, error) {
	var services []*model.Service
	query := `SELECT id, name, description, url, type, interval, status, is_active, sort_order, created_at, updated_at
			  FROM Service ORDER BY sort_order ASC, id ASC`
	err := r.db.SelectContext(ctx, &services, query)
	return services, err
}

func (r *repo) GetService(ctx context.Context, id int64) (*model.Service, error) {
	var s model.Service
	query := `SELECT id, name, description, url, type, interval, status, is_active, sort_order, created_at, updated_at
			  FROM Service WHERE id = ?`
	err := r.db.GetContext(ctx, &s, query, id)
	return &s, err
}

func (r *repo) UpdateService(ctx context.Context, s *model.Service) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	query := `UPDATE Service SET name=?, description=?, url=?, type=?, interval=?, status=?, is_active=?, sort_order=?, updated_at=?
			  WHERE id=?`
	_, err := r.db.ExecContext(ctx, query, s.Name, s.Description, s.URL, s.Type, s.Interval,
		s.Status, s.IsActive, s.SortOrder, now, s.ID)
	if err != nil {
		return err
	}
	s.UpdatedAt = now
	return nil
}

func (r *repo) DeleteService(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM Service WHERE id = ?", id)
	return err
}

// Heartbeat

func (r *repo) CreateHeartbeat(ctx context.Context, h *model.Heartbeat) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	query := `INSERT INTO Heartbeat (service_id, status, latency, message, created_at)
			  VALUES (?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, h.ServiceID, h.Status, h.Latency, h.Message, now)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	h.ID = id
	h.CreatedAt = now
	return nil
}

func (r *repo) GetServiceHistory(ctx context.Context, serviceID int64, days int) ([]*model.Heartbeat, error) {
	var heartbeats []*model.Heartbeat
	since := time.Now().AddDate(0, 0, -days).UTC().Format("2006-01-02T15:04:05Z")
	query := `SELECT id, service_id, status, latency, message, created_at
			  FROM Heartbeat WHERE service_id = ? AND created_at >= ?
			  ORDER BY created_at ASC`
	err := r.db.SelectContext(ctx, &heartbeats, query, serviceID, since)
	return heartbeats, err
}

func (r *repo) ListHeartbeats(ctx context.Context, serviceID int64, statusFilter string, page, limit int) ([]*model.LogEntry, int64, error) {
	conditions := []string{}
	args := []interface{}{}

	if serviceID > 0 {
		conditions = append(conditions, "h.service_id = ?")
		args = append(args, serviceID)
	}

	if statusFilter == "success" {
		conditions = append(conditions, "((h.status >= 200 AND h.status < 400) OR h.status = 1)")
	} else if statusFilter == "failure" {
		conditions = append(conditions, "NOT ((h.status >= 200 AND h.status < 400) OR h.status = 1)")
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			where += " AND " + conditions[i]
		}
	}

	// Count total
	var total int64
	countQuery := `SELECT COUNT(*) FROM Heartbeat h ` + where
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	// Fetch page
	offset := (page - 1) * limit
	fetchArgs := append([]interface{}{}, args...)
	query := `SELECT h.id, h.service_id, h.status, h.latency, h.message, h.created_at,
	          COALESCE(s.name, '') as service_name
	          FROM Heartbeat h
	          LEFT JOIN Service s ON h.service_id = s.id
	          ` + where + ` ORDER BY h.created_at DESC LIMIT ? OFFSET ?`
	fetchArgs = append(fetchArgs, limit, offset)

	var entries []*model.LogEntry
	if err := r.db.SelectContext(ctx, &entries, query, fetchArgs...); err != nil {
		return nil, 0, err
	}
	if entries == nil {
		entries = []*model.LogEntry{}
	}
	return entries, total, nil
}

func (r *repo) GetLatestHeartbeat(ctx context.Context, serviceID int64) (*model.Heartbeat, error) {
	var h model.Heartbeat
	query := `SELECT id, service_id, status, latency, message, created_at
			  FROM Heartbeat WHERE service_id = ? ORDER BY created_at DESC LIMIT 1`
	err := r.db.GetContext(ctx, &h, query, serviceID)
	return &h, err
}

func (r *repo) DeleteOldHeartbeats(ctx context.Context, before string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM Heartbeat WHERE created_at < ?", before)
	return err
}
