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

// probeTaskRow 查询探测任务用的行结构。
// server_id 可为 NULL，用 sql.NullInt64 直接扫，避免像以前那样为了拿到它
// 再按行查一次（列表接口的 N+1）。
type probeTaskRow struct {
	ID           int64         `db:"id"`
	ServiceID    int64         `db:"service_id"`
	ServerID     sql.NullInt64 `db:"server_id"`
	TriggerCount int           `db:"trigger_count"`
	IsActive     bool          `db:"is_active"`
	CreatedAt    string        `db:"created_at"`
	UpdatedAt    string        `db:"updated_at"`
	ServiceName  string        `db:"service_name"`
	ServerName   string        `db:"server_name"`
}

func (row *probeTaskRow) toModel() *model.ProbeTask {
	task := &model.ProbeTask{
		ID:           row.ID,
		ServiceID:    row.ServiceID,
		TriggerCount: row.TriggerCount,
		IsActive:     row.IsActive,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
		ServiceName:  row.ServiceName,
		ServerName:   row.ServerName,
	}
	if row.ServerID.Valid {
		serverID := row.ServerID.Int64
		task.ServerID = &serverID
	}
	return task
}

func (r *repo) ListProbeTasks(ctx context.Context) ([]*model.ProbeTask, error) {
	query := `SELECT pt.id, pt.service_id, pt.server_id, pt.trigger_count, pt.is_active,
	                 pt.created_at, pt.updated_at,
	                 COALESCE(s.name, '') AS service_name, COALESCE(sv.name, '') AS server_name
			  FROM ProbeTask pt
			  LEFT JOIN Service s ON s.id = pt.service_id
			  LEFT JOIN Server sv ON sv.id = pt.server_id
			  ORDER BY pt.id ASC`

	var rows []*probeTaskRow
	if err := r.db.SelectContext(ctx, &rows, query); err != nil {
		return nil, err
	}

	tasks := make([]*model.ProbeTask, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, row.toModel())
	}
	return tasks, nil
}

const probeTaskCols = "id, service_id, server_id, trigger_count, is_active, created_at, updated_at"

func (r *repo) GetProbeTask(ctx context.Context, id int64) (*model.ProbeTask, error) {
	var row probeTaskRow
	err := r.db.GetContext(ctx, &row, "SELECT "+probeTaskCols+" FROM ProbeTask WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return row.toModel(), nil
}

func (r *repo) GetProbeTaskByService(ctx context.Context, serviceID int64) (*model.ProbeTask, error) {
	var row probeTaskRow
	err := r.db.GetContext(ctx, &row, "SELECT "+probeTaskCols+" FROM ProbeTask WHERE service_id = ?", serviceID)
	if err != nil {
		return nil, err
	}
	return row.toModel(), nil
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
