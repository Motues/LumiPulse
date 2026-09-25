package sqlite

import (
	"context"
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// serviceColumns Service 表的显式列清单（带 s 前缀：查询里关联了 ServiceFolder，
// 两张表都有 id / name / description，不加前缀会报 ambiguous column name）。
// 探测配置新增字段时只需改这一处，避免 SELECT 漏列导致字段静默为空。
const serviceColumns = `s.id, s.name, s.description, s.url, s.type, s.interval, s.status, s.is_active, s.sort_order,
	s.show_on_homepage, s.insecure_skip_verify, s.timeout_seconds, s.cert_expires_at, s.cert_notify_level, s.public_hash,
	s.http_method, s.http_headers, s.http_body, s.expect_status, s.expect_keyword, s.folder_id, s.homepage_blocks,
	s.created_at, s.updated_at`

// serviceSelectColumns 带分组名的查询列（管理端列表回显用）
const serviceSelectColumns = serviceColumns + `, COALESCE(f.name, '') AS folder_name`

// serviceJoinFolder 关联服务分组，用于带出分组名
const serviceJoinFolder = `LEFT JOIN ServiceFolder f ON f.id = s.folder_id`

func (r *repo) CreateService(ctx context.Context, s *model.Service) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	if s.PublicHash == "" {
		s.PublicHash = utils.GeneratePublicHash()
	}
	query := `INSERT INTO Service (name, description, url, type, interval, status, is_active, sort_order, show_on_homepage, insecure_skip_verify, timeout_seconds, public_hash,
				  http_method, http_headers, http_body, expect_status, expect_keyword, folder_id, homepage_blocks, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, s.Name, s.Description, s.URL, s.Type, s.Interval,
		"operational", true, s.SortOrder, s.ShowOnHomepage, s.InsecureSkipVerify, s.TimeoutSeconds, s.PublicHash,
		s.HTTPMethod, s.HTTPHeaders, s.HTTPBody, s.ExpectStatus, s.ExpectKeyword, s.FolderID, s.HomepageBlocks, now, now)
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
	query := `SELECT ` + serviceSelectColumns + `
			  FROM Service s ` + serviceJoinFolder + `
			  ORDER BY s.sort_order ASC, s.id ASC`
	err := r.db.SelectContext(ctx, &services, query)
	return services, err
}

func (r *repo) GetService(ctx context.Context, id int64) (*model.Service, error) {
	var s model.Service
	query := `SELECT ` + serviceSelectColumns + `
			  FROM Service s ` + serviceJoinFolder + `
			  WHERE s.id = ?`
	err := r.db.GetContext(ctx, &s, query, id)
	return &s, err
}

// GetServiceByHash 通过公开标识获取服务（公开页面使用，替代自增 ID）
func (r *repo) GetServiceByHash(ctx context.Context, hash string) (*model.Service, error) {
	var s model.Service
	query := `SELECT ` + serviceSelectColumns + `
			  FROM Service s ` + serviceJoinFolder + `
			  WHERE s.public_hash = ?`
	err := r.db.GetContext(ctx, &s, query, hash)
	return &s, err
}

func (r *repo) UpdateService(ctx context.Context, s *model.Service) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	query := `UPDATE Service SET name=?, description=?, url=?, type=?, interval=?, status=?, is_active=?, sort_order=?,
				  show_on_homepage=?, insecure_skip_verify=?, timeout_seconds=?,
				  http_method=?, http_headers=?, http_body=?, expect_status=?, expect_keyword=?, folder_id=?, homepage_blocks=?, updated_at=?
			  WHERE id=?`
	_, err := r.db.ExecContext(ctx, query, s.Name, s.Description, s.URL, s.Type, s.Interval,
		s.Status, s.IsActive, s.SortOrder, s.ShowOnHomepage, s.InsecureSkipVerify, s.TimeoutSeconds,
		s.HTTPMethod, s.HTTPHeaders, s.HTTPBody, s.ExpectStatus, s.ExpectKeyword, s.FolderID, s.HomepageBlocks, now, s.ID)
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

// UpdateServiceCert 只更新证书到期信息。
// 检查器在探测协程里写回，不能整行 UPDATE Service：那份 svc 快照的 status
// 可能已被事件流程改动，整行写回会把并发的状态变更覆盖掉。
func (r *repo) UpdateServiceCert(ctx context.Context, id int64, certExpiresAt string, notifyLevel int) error {
	query := `UPDATE Service SET cert_expires_at=?, cert_notify_level=? WHERE id=?`
	_, err := r.db.ExecContext(ctx, query, certExpiresAt, notifyLevel, id)
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

	if statusFilter == "success" || statusFilter == "failure" {
		// 成功判定与探测时保持一致（期望状态码 + 期望关键字），按行读取服务配置
		okExpr := probeOKExpr("h", "s")
		if statusFilter == "success" {
			conditions = append(conditions, okExpr)
		} else {
			conditions = append(conditions, "NOT "+okExpr)
		}
	}

	// 成功判定要读服务配置，因此无论是否按状态筛选都带上 Service 关联
	from := `Heartbeat h ` + serviceJoin

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			where += " AND " + conditions[i]
		}
	}

	// Count total
	var total int64
	countQuery := `SELECT COUNT(*) FROM ` + from + ` ` + where
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	// Fetch page
	offset := (page - 1) * limit
	fetchArgs := append([]interface{}{}, args...)
	// 每行同时回传后端判定的成功与否，前端不必再自行按状态码猜测
	query := `SELECT h.id, h.service_id, h.status, h.latency, h.message, h.created_at,
			  COALESCE(s.name, '') as service_name,
			  CASE WHEN ` + probeOKExpr("h", "s") + ` THEN 1 ELSE 0 END as is_success
			  FROM ` + from + ` ` + where + ` ORDER BY h.created_at DESC LIMIT ? OFFSET ?`
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
	// 失败判定与日志页/分位数接口保持一致，且按行遵循服务自己的期望状态码与关键字。
	// 关联 Service 后 status / latency 在两张表里同名，必须带表别名限定。
	okExpr := probeOKExpr("h", "s")
	query := `SELECT
		CAST((strftime('%s', h.created_at) - strftime('%s', ?)) / ? AS INTEGER) AS bucket,
		CAST(AVG(h.latency) AS INTEGER) AS avg_latency,
		SUM(CASE WHEN NOT ` + okExpr + ` THEN 1 ELSE 0 END) AS failures,
		COUNT(*) AS total
		FROM Heartbeat h
		` + serviceJoin + `
		WHERE h.service_id = ? AND h.created_at >= ?
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

// GetLatencyHeatmap 按「日期 + 小时」聚合延迟（北京时间），供热力图使用。
// 聚合下沉到 SQLite：7 天 × 288 个 5 分钟桶的原始数据不会进内存。
// since 需为 UTC 的 "2006-01-02T15:04:05Z" 格式（与库内 created_at 一致）。
func (r *repo) GetLatencyHeatmap(ctx context.Context, serviceID int64, since string) ([]*model.LatencyHeatmapCell, error) {
	// created_at 存的是 UTC，+8 小时即北京时间墙上时间；
	// 成功判定与延迟分桶 / 分位数接口保持一致，并遵循服务的期望状态码与关键字。
	okExpr := probeOKExpr("h", "s")
	query := `SELECT
		strftime('%Y-%m-%d %H', h.created_at, '+8 hours') AS bucket,
		CAST(COALESCE(AVG(CASE WHEN ` + okExpr + ` THEN h.latency END), 0) AS INTEGER) AS avg_latency,
		SUM(CASE WHEN ` + okExpr + ` THEN 1 ELSE 0 END) AS samples,
		SUM(CASE WHEN ` + okExpr + ` THEN 0 ELSE 1 END) AS failures
		FROM Heartbeat h
		` + serviceJoin + `
		WHERE h.service_id = ? AND h.created_at >= ?
		GROUP BY bucket
		ORDER BY bucket ASC`

	type row struct {
		Bucket     string `db:"bucket"`
		AvgLatency int    `db:"avg_latency"`
		Samples    int    `db:"samples"`
		Failures   int    `db:"failures"`
	}

	var rows []row
	if err := r.db.SelectContext(ctx, &rows, query, serviceID, since); err != nil {
		return nil, err
	}

	cells := make([]*model.LatencyHeatmapCell, 0, len(rows))
	for _, item := range rows {
		parts := strings.SplitN(item.Bucket, " ", 2)
		if len(parts) != 2 {
			continue
		}
		hour, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		cells = append(cells, &model.LatencyHeatmapCell{
			Day:      parts[0],
			Hour:     hour,
			Avg:      item.AvgLatency,
			Samples:  item.Samples,
			Failures: item.Failures,
		})
	}
	return cells, nil
}

// GetLatencyStats 计算窗口内的延迟分位数（p95 / p99 / 平均 / 峰值）。
// 分位数用「近邻插值」在 SQL 内算：取第 ceil(q*n) 小的样本作为 q 分位。
// SQLite 没有内置 percentile，这样写一次扫描即可，避免把窗口内所有样本搬到 Go 里排序。
// since 需为 UTC 的 "2006-01-02 15:04:05" 格式。
func (r *repo) GetLatencyStats(ctx context.Context, serviceID int64, since string) (*model.LatencyStats, error) {
	// 成功判定与 GetLatencyBuckets 保持一致，并遵循服务的期望状态码与关键字
	okExpr := probeOKExpr("h", "s")
	query := `WITH ok AS (
		SELECT h.latency, ROW_NUMBER() OVER (ORDER BY h.latency ASC) AS rn, COUNT(*) OVER () AS n
		FROM Heartbeat h
		` + serviceJoin + `
		WHERE h.service_id = ? AND h.created_at >= ?
		  AND ` + okExpr + `
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
