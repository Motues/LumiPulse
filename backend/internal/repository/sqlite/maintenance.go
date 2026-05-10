package sqlite

import (
	"context"
	"lumipluse-backend/internal/model"
	"time"
)

func (r *repo) CreateMaintenance(ctx context.Context, m *model.Maintenance) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	query := `INSERT INTO Maintenance (title, description, scheduled_start, scheduled_end, status, affected_services, created_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, m.Title, m.Description, m.ScheduledStart, m.ScheduledEnd, m.Status, m.AffectedServices, now)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	m.ID = id
	m.CreatedAt = now
	return nil
}

func (r *repo) ListMaintenances(ctx context.Context) ([]*model.Maintenance, error) {
	var maintenances []*model.Maintenance
	query := `SELECT * FROM Maintenance ORDER BY scheduled_start DESC`
	err := r.db.SelectContext(ctx, &maintenances, query)
	return maintenances, err
}

func (r *repo) ListActiveMaintenances(ctx context.Context) ([]*model.Maintenance, error) {
	var maintenances []*model.Maintenance
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	query := `SELECT * FROM Maintenance WHERE status IN ('scheduled', 'in_progress') AND scheduled_end >= ? ORDER BY scheduled_start ASC`
	err := r.db.SelectContext(ctx, &maintenances, query, now)
	return maintenances, err
}

func (r *repo) GetMaintenance(ctx context.Context, id int64) (*model.Maintenance, error) {
	var m model.Maintenance
	err := r.db.GetContext(ctx, &m, "SELECT * FROM Maintenance WHERE id = ?", id)
	return &m, err
}

func (r *repo) UpdateMaintenance(ctx context.Context, m *model.Maintenance) error {
	query := `UPDATE Maintenance SET title=?, description=?, scheduled_start=?, scheduled_end=?, status=?, affected_services=?
			  WHERE id=?`
	_, err := r.db.ExecContext(ctx, query, m.Title, m.Description, m.ScheduledStart, m.ScheduledEnd, m.Status, m.AffectedServices, m.ID)
	return err
}

func (r *repo) DeleteMaintenance(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM Maintenance WHERE id = ?", id)
	return err
}
