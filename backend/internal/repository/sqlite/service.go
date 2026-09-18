package sqlite

import (
	"context"
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"
	"time"

	"github.com/jmoiron/sqlx"
)

func (r *repo) CreateService(ctx context.Context, s *model.Service) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	if s.PublicHash == "" {
		s.PublicHash = utils.GeneratePublicHash()
	}
	query := `INSERT INTO Service (name, description, url, type, interval, status, is_active, sort_order, show_on_homepage, insecure_skip_verify, public_hash, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, s.Name, s.Description, s.URL, s.Type, s.Interval,
		"operational", true, s.SortOrder, s.ShowOnHomepage, s.InsecureSkipVerify, s.PublicHash, now, now)
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
	query := `SELECT id, name, description, url, type, interval, status, is_active, sort_order, show_on_homepage, insecure_skip_verify, public_hash, created_at, updated_at
			  FROM Service ORDER BY sort_order ASC, id ASC`
	err := r.db.SelectContext(ctx, &services, query)
	return services, err
}

func (r *repo) GetService(ctx context.Context, id int64) (*model.Service, error) {
	var s model.Service
	query := `SELECT id, name, description, url, type, interval, status, is_active, sort_order, show_on_homepage, insecure_skip_verify, public_hash, created_at, updated_at
			  FROM Service WHERE id = ?`
	err := r.db.GetContext(ctx, &s, query, id)
	return &s, err
}

// GetServiceByHash 通过公开标识获取服务（公开页面使用，替代自增 ID）
func (r *repo) GetServiceByHash(ctx context.Context, hash string) (*model.Service, error) {
	var s model.Service
	query := `SELECT id, name, description, url, type, interval, status, is_active, sort_order, show_on_homepage, insecure_skip_verify, public_hash, created_at, updated_at
			  FROM Service WHERE public_hash = ?`
	err := r.db.GetContext(ctx, &s, query, hash)
	return &s, err
}

func (r *repo) UpdateService(ctx context.Context, s *model.Service) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	query := `UPDATE Service SET name=?, description=?, url=?, type=?, interval=?, status=?, is_active=?, sort_order=?, show_on_homepage=?, insecure_skip_verify=?, updated_at=?
			  WHERE id=?`
	_, err := r.db.ExecContext(ctx, query, s.Name, s.Description, s.URL, s.Type, s.Interval,
		s.Status, s.IsActive, s.SortOrder, s.ShowOnHomepage, s.InsecureSkipVerify, now, s.ID)
	if err != nil {
		return err
	}
	s.UpdatedAt = now
	return nil
}

func (r *repo) UpdateServiceSortOrder(ctx context.Context, id int64, sortOrder int) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	query := `UPDATE Service SET sort_order=?, updated_at=? WHERE id=?`
	_, err := r.db.ExecContext(ctx, query, sortOrder, now, id)
	return err
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

// BatchGetLatestHeartbeats 一次性取回多个服务各自的最新心跳，避免按服务逐个查询（N+1）。
func (r *repo) BatchGetLatestHeartbeats(ctx context.Context, serviceIDs []int64) (map[int64]*model.Heartbeat, error) {
	result := make(map[int64]*model.Heartbeat)
	if len(serviceIDs) == 0 {
		return result, nil
	}

	query := `SELECT h.id, h.service_id, h.status, h.latency, h.message, h.created_at
			  FROM Heartbeat h
			  INNER JOIN (
				SELECT service_id, MAX(created_at) AS max_created
				FROM Heartbeat WHERE service_id IN (?) GROUP BY service_id
			  ) m ON h.service_id = m.service_id AND h.created_at = m.max_created
			  GROUP BY h.service_id`
	query, args, err := sqlx.In(query, serviceIDs)
	if err != nil {
		return result, err
	}
	query = r.db.Rebind(query)

	var rows []*model.Heartbeat
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return result, err
	}
	for _, hb := range rows {
		result[hb.ServiceID] = hb
	}
	return result, nil
}

// GetLatencyBuckets 在 SQLite 内把心跳按固定时间桶聚合，避免把窗口内所有原始行搬到 Go。
// since 需为 UTC 的 "2006-01-02 15:04:05" 格式（SQLite 按 UTC 解析不带时区的时间）。
func (r *repo) GetLatencyBuckets(ctx context.Context, serviceID int64, since string, bucketSeconds, maxBucket int) ([]*model.LatencyBucket, error) {
	// 失败判定与日志页保持一致：成功 = (200<=status<400) 或 status=1（TCP 成功）
	query := `SELECT
		CAST((strftime('%s', created_at) - strftime('%s', ?)) / ? AS INTEGER) AS bucket,
		CAST(AVG(latency) AS INTEGER) AS avg_latency,
		SUM(CASE WHEN NOT ((status >= 200 AND status < 400) OR status = 1) THEN 1 ELSE 0 END) AS failures,
		COUNT(*) AS total
		FROM Heartbeat
		WHERE service_id = ? AND created_at >= ?
		GROUP BY bucket
		HAVING bucket >= 0 AND bucket < ?
		ORDER BY bucket ASC`

	var buckets []*model.LatencyBucket
	err := r.db.SelectContext(ctx, &buckets, query, since, bucketSeconds, serviceID, since, maxBucket)
	return buckets, err
}

func (r *repo) DeleteOldHeartbeats(ctx context.Context, before string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM Heartbeat WHERE created_at < ?", before)
	return err
}

// GetLatencyStats 计算窗口内的延迟分位数（p95 / p99 / 平均 / 峰值）。
// 分位数用「近邻插值」在 SQL 内算：取第 ceil(q*n) 小的样本作为 q 分位。
// SQLite 没有内置 percentile，这样写一次扫描即可，避免把窗口内所有样本搬到 Go 里排序。
// since 需为 UTC 的 "2006-01-02 15:04:05" 格式。
func (r *repo) GetLatencyStats(ctx context.Context, serviceID int64, since string) (*model.LatencyStats, error) {
	// 成功判定与 GetLatencyBuckets 保持一致：成功 = (200<=status<400) 或 status=1（TCP 成功）
	query := `WITH ok AS (
		SELECT latency, ROW_NUMBER() OVER (ORDER BY latency ASC) AS rn, COUNT(*) OVER () AS n
		FROM Heartbeat
		WHERE service_id = ? AND created_at >= ?
		  AND ((status >= 200 AND status < 400) OR status = 1)
	)
	SELECT
		(SELECT n FROM ok LIMIT 1) AS samples,
		(SELECT AVG(latency) FROM ok) AS avg,
		(SELECT latency FROM ok WHERE rn = MAX(1, CAST((n * 0.95) + 0.9999 AS INTEGER)) LIMIT 1) AS p95,
		(SELECT latency FROM ok WHERE rn = MAX(1, CAST((n * 0.99) + 0.9999 AS INTEGER)) LIMIT 1) AS p99,
		(SELECT MAX(latency) FROM ok) AS max`

	var stats model.LatencyStats
	if err := r.db.GetContext(ctx, &stats, query, serviceID, since); err != nil {
		return nil, err
	}
	if stats.Samples == 0 {
		return nil, nil
	}
	return &stats, nil
}
