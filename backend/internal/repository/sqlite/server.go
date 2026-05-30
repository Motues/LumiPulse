package sqlite

import (
	"context"
	"lumipluse-backend/internal/model"
	"time"
)

func (r *repo) CreateServer(ctx context.Context, s *model.Server) error {
	now := time.Now().Format(time.RFC3339)
	query := `INSERT INTO Server (name, description, auto_merge, auto_merge_threshold, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, s.Name, s.Description, s.AutoMerge, s.AutoMergeThreshold, now, now)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	s.ID = id
	s.CreatedAt = now
	s.UpdatedAt = now
	return nil
}

func (r *repo) ListServers(ctx context.Context) ([]*model.Server, error) {
	var servers []*model.Server
	err := r.db.SelectContext(ctx, &servers, "SELECT * FROM Server ORDER BY id ASC")
	return servers, err
}

func (r *repo) GetServer(ctx context.Context, id int64) (*model.Server, error) {
	var s model.Server
	err := r.db.GetContext(ctx, &s, "SELECT * FROM Server WHERE id = ?", id)
	return &s, err
}

func (r *repo) UpdateServer(ctx context.Context, s *model.Server) error {
	now := time.Now().Format(time.RFC3339)
	query := `UPDATE Server SET name=?, description=?, auto_merge=?, auto_merge_threshold=?, updated_at=? WHERE id=?`
	_, err := r.db.ExecContext(ctx, query, s.Name, s.Description, s.AutoMerge, s.AutoMergeThreshold, now, s.ID)
	if err != nil {
		return err
	}
	s.UpdatedAt = now
	return nil
}

func (r *repo) DeleteServer(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM Server WHERE id = ?", id)
	return err
}
