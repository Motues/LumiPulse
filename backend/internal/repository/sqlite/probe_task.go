package sqlite

import (
	"context"
	"database/sql"
	"lumipluse-backend/internal/model"
	"time"
)

func (r *repo) CreateProbeTask(ctx context.Context, t *model.ProbeTask) error {
	now := time.Now().Format(time.RFC3339)
	query := `INSERT INTO ProbeTask (service_id, server_id, trigger_count, is_active, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?)`
	var serverID interface{}
	if t.ServerID != nil {
		serverID = *t.ServerID
	}
	res, err := r.db.ExecContext(ctx, query, t.ServiceID, serverID, t.TriggerCount, t.IsActive, now, now)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	t.ID = id
	t.CreatedAt = now
	t.UpdatedAt = now
	return nil
}

func (r *repo) ListProbeTasks(ctx context.Context) ([]*model.ProbeTask, error) {
	var tasks []*model.ProbeTask
	query := `SELECT pt.*, s.name as service_name, COALESCE(sv.name, '') as server_name
			  FROM ProbeTask pt
			  LEFT JOIN Service s ON s.id = pt.service_id
			  LEFT JOIN Server sv ON sv.id = pt.server_id
			  ORDER BY pt.id ASC`
	err := r.db.SelectContext(ctx, &tasks, query)
	if err != nil {
		return nil, err
	}
	// Handle nullable server_id
	for _, t := range tasks {
		var serverID sql.NullInt64
		r.db.GetContext(ctx, &serverID, "SELECT server_id FROM ProbeTask WHERE id = ?", t.ID)
		if serverID.Valid {
			v := serverID.Int64
			t.ServerID = &v
		}
	}
	return tasks, nil
}

const probeTaskCols = "id, service_id, server_id, trigger_count, is_active, created_at, updated_at"

func (r *repo) GetProbeTask(ctx context.Context, id int64) (*model.ProbeTask, error) {
	var t model.ProbeTask
	err := r.db.GetContext(ctx, &t, "SELECT "+probeTaskCols+" FROM ProbeTask WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	// Handle nullable server_id
	var serverID sql.NullInt64
	r.db.GetContext(ctx, &serverID, "SELECT server_id FROM ProbeTask WHERE id = ?", id)
	if serverID.Valid {
		v := serverID.Int64
		t.ServerID = &v
	} else {
		t.ServerID = nil
	}
	return &t, nil
}

func (r *repo) GetProbeTaskByService(ctx context.Context, serviceID int64) (*model.ProbeTask, error) {
	var t model.ProbeTask
	err := r.db.GetContext(ctx, &t, "SELECT "+probeTaskCols+" FROM ProbeTask WHERE service_id = ?", serviceID)
	if err != nil {
		return nil, err
	}
	var serverID sql.NullInt64
	r.db.GetContext(ctx, &serverID, "SELECT server_id FROM ProbeTask WHERE id = ?", t.ID)
	if serverID.Valid {
		v := serverID.Int64
		t.ServerID = &v
	} else {
		t.ServerID = nil
	}
	return &t, nil
}

func (r *repo) UpdateProbeTask(ctx context.Context, t *model.ProbeTask) error {
	now := time.Now().Format(time.RFC3339)
	var serverID interface{}
	if t.ServerID != nil {
		serverID = *t.ServerID
	}
	query := `UPDATE ProbeTask SET server_id=?, trigger_count=?, is_active=?, updated_at=? WHERE id=?`
	_, err := r.db.ExecContext(ctx, query, serverID, t.TriggerCount, t.IsActive, now, t.ID)
	if err != nil {
		return err
	}
	t.UpdatedAt = now
	return nil
}

func (r *repo) DeleteProbeTask(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM ProbeTask WHERE id = ?", id)
	return err
}

func (r *repo) ListProbeTasksByServer(ctx context.Context, serverID int64) ([]*model.ProbeTask, error) {
	var tasks []*model.ProbeTask
	err := r.db.SelectContext(ctx, &tasks, "SELECT "+probeTaskCols+" FROM ProbeTask WHERE server_id = ?", serverID)
	return tasks, err
}
