package sqlite

import (
	"context"
	"lumipluse-backend/internal/model"
	"time"
)

const apiKeyColumns = `id, name, key, key_prefix, expires_at, last_used_at, last_used_ip, is_active, scope, rate_limit_per_minute, created_at`

func (r *repo) CreateApiKey(ctx context.Context, k *model.ApiKey) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	query := `INSERT INTO ApiKey (name, key, key_prefix, expires_at, is_active, scope, rate_limit_per_minute, created_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, k.Name, k.Key, k.KeyPrefix, k.ExpiresAt, 1,
		k.Scope, k.RateLimitPerMinute, now)
	if err != nil {
		return err
	}
	// 回填自增 ID：创建接口要把它返回给调用方，漏掉会返回 id=0
	if id, err := res.LastInsertId(); err == nil {
		k.ID = id
	}
	k.CreatedAt = now
	k.IsActive = true
	return nil
}

func (r *repo) ListApiKeys(ctx context.Context) ([]*model.ApiKey, error) {
	var keys []*model.ApiKey
	query := `SELECT ` + apiKeyColumns + ` FROM ApiKey ORDER BY created_at DESC`
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
	query := `SELECT ` + apiKeyColumns + ` FROM ApiKey WHERE id = ?`
	err := r.db.GetContext(ctx, &k, query, id)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (r *repo) GetApiKeyByKey(ctx context.Context, key string) (*model.ApiKey, error) {
	var k model.ApiKey
	query := `SELECT ` + apiKeyColumns + ` FROM ApiKey WHERE key = ?`
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

// UpdateApiKey 更新名称、权限范围与限流（界面上的「编辑密钥」）
func (r *repo) UpdateApiKey(ctx context.Context, id int64, name, scope string, rateLimitPerMinute int) error {
	query := `UPDATE ApiKey SET name = ?, scope = ?, rate_limit_per_minute = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, name, scope, rateLimitPerMinute, id)
	return err
}

func (r *repo) DeleteApiKey(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM ApiKey WHERE id = ?", id)
	return err
}
