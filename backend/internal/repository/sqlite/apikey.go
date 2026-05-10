package sqlite

import (
	"context"
	"lumipluse-backend/internal/model"
	"time"
)

func (r *repo) CreateApiKey(ctx context.Context, k *model.ApiKey) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	query := `INSERT INTO ApiKey (name, key, key_prefix, expires_at, is_active, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, k.Name, k.Key, k.KeyPrefix, k.ExpiresAt, 1, now)
	if err != nil {
		return err
	}
	k.CreatedAt = now
	k.IsActive = true
	return nil
}

func (r *repo) ListApiKeys(ctx context.Context) ([]*model.ApiKey, error) {
	var keys []*model.ApiKey
	query := `SELECT id, name, key, key_prefix, expires_at, last_used_at, last_used_ip, is_active, created_at FROM ApiKey ORDER BY created_at DESC`
	err := r.db.SelectContext(ctx, &keys, query)
	if err != nil {
		return nil, err
	}
	if keys == nil {
		keys = []*model.ApiKey{}
	}
	return keys, nil
}

func (r *repo) GetApiKey(ctx context.Context, id int64) (*model.ApiKey, error) {
	var k model.ApiKey
	query := `SELECT id, name, key, key_prefix, expires_at, last_used_at, last_used_ip, is_active, created_at FROM ApiKey WHERE id = ?`
	err := r.db.GetContext(ctx, &k, query, id)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (r *repo) GetApiKeyByKey(ctx context.Context, key string) (*model.ApiKey, error) {
	var k model.ApiKey
	query := `SELECT id, name, key, key_prefix, expires_at, last_used_at, last_used_ip, is_active, created_at FROM ApiKey WHERE key = ?`
	err := r.db.GetContext(ctx, &k, query, key)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (r *repo) UpdateApiKeyLastUsed(ctx context.Context, id int64, ip string) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	query := `UPDATE ApiKey SET last_used_at = ?, last_used_ip = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, now, ip, id)
	return err
}

func (r *repo) UpdateApiKeyName(ctx context.Context, id int64, name string) error {
	query := `UPDATE ApiKey SET name = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, name, id)
	return err
}

func (r *repo) DeleteApiKey(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM ApiKey WHERE id = ?", id)
	return err
}
