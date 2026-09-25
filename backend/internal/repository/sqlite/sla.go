package sqlite

import (
	"context"
	"time"

	"lumipluse-backend/internal/model"
)

// utcNowString 与库内其他表统一的时间串格式
func utcNowString() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

// monthlyRow 单服务单月的聚合结果（SQL 行）
type monthlyRow struct {
	ID                  int64   `db:"id"`
	ServiceID           int64   `db:"service_id"`
	Month               string  `db:"month"`
	UptimeCount         int     `db:"uptime_count"`
	DowntimeCount       int     `db:"downtime_count"`
	TotalLatency        int     `db:"total_latency"`
	IncidentTotal       int     `db:"incident_total"`
	IncidentDowntimeSec int     `db:"incident_downtime_seconds"`
	RecordedAt          *string `db:"recorded_at"`
	ClosedAt            *string `db:"closed_at"`
}

// incidentDowntimeRow 事件维度的不可用时长聚合
type incidentDowntimeRow struct {
	ServiceID int64 `db:"service_id"`
	Total     int   `db:"total"`
}

// ServiceMonthly 持久化月报

func (r *repo) GetServiceMonthly(ctx context.Context, serviceID int64, month string) (*model.ServiceMonthly, error) {
	var row monthlyRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, service_id, month, uptime_count, downtime_count, total_latency,
		        incident_total, incident_downtime_seconds, recorded_at, closed_at
		 FROM ServiceMonthly WHERE service_id = ? AND month = ?`, serviceID, month)
	if err != nil {
		return nil, err
	}
	return monthlyRowToModel(&row), nil
}

func (r *repo) ListServiceMonthlies(ctx context.Context, month string) ([]*model.ServiceMonthly, error) {
	var rows []monthlyRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, service_id, month, uptime_count, downtime_count, total_latency,
		        incident_total, incident_downtime_seconds, recorded_at, closed_at
		 FROM ServiceMonthly WHERE month = ?`, month)
	if err != nil {
		return nil, err
	}
	result := make([]*model.ServiceMonthly, 0, len(rows))
	for i := range rows {
		result = append(result, monthlyRowToModel(&rows[i]))
	}
	return result, nil
}

// UpsertServiceMonthly 写入/更新月报。ON CONFLICT 时 incremental 决定计数是
// 「覆盖」（重算）还是「累加」（增量归档），两种调用场景见 sla.go。
func (r *repo) UpsertServiceMonthly(ctx context.Context, m *model.ServiceMonthly, incremental bool) error {
	now := utcNowString()

	if incremental {
		_, err := r.db.ExecContext(ctx,
			`INSERT INTO ServiceMonthly
			   (service_id, month, uptime_count, downtime_count, total_latency,
			    incident_total, incident_downtime_seconds, recorded_at, closed_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(service_id, month) DO UPDATE SET
			   uptime_count = uptime_count + excluded.uptime_count,
			   downtime_count = downtime_count + excluded.downtime_count,
			   total_latency = total_latency + excluded.total_latency,
			   incident_total = incident_total + excluded.incident_total,
			   incident_downtime_seconds = incident_downtime_seconds + excluded.incident_downtime_seconds,
			   recorded_at = excluded.recorded_at`,
			m.ServiceID, m.Month, m.UptimeCount, m.DowntimeCount, m.TotalLatency,
			m.IncidentTotal, m.IncidentDowntimeSecs, now, m.ClosedAt)
		return err
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO ServiceMonthly
		   (service_id, month, uptime_count, downtime_count, total_latency,
		    incident_total, incident_downtime_seconds, recorded_at, closed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(service_id, month) DO UPDATE SET
		   uptime_count = excluded.uptime_count,
		   downtime_count = excluded.downtime_count,
		   total_latency = excluded.total_latency,
		   incident_total = excluded.incident_total,
		   incident_downtime_seconds = excluded.incident_downtime_seconds,
		   recorded_at = excluded.recorded_at,
		   closed_at = excluded.closed_at`,
		m.ServiceID, m.Month, m.UptimeCount, m.DowntimeCount, m.TotalLatency,
		m.IncidentTotal, m.IncidentDowntimeSecs, now, m.ClosedAt)
	return err
}

// SetServiceMonthlyClosed 归档后标记月份已关闭（数据不再变化）
func (r *repo) SetServiceMonthlyClosed(ctx context.Context, serviceID int64, month, closedAt string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE ServiceMonthly SET closed_at = ? WHERE service_id = ? AND month = ?`,
		closedAt, serviceID, month)
	return err
}

// 月度聚合

// AggregateServiceMonthly 从 ServiceDaily 汇总某个自然月的数据。
// 时间范围用 [from, toExclusive) 形式的日期字符串，避免依赖 SQLite 的 strftime。
func (r *repo) AggregateServiceMonthly(ctx context.Context, from, toExclusive string) ([]*model.ServiceMonthly, error) {
	var rows []monthlyRow
	err := r.db.SelectContext(ctx, &rows,
		`WITH agg AS (
			SELECT service_id,
			       SUM(uptime_count) AS uptime_count,
			       SUM(downtime_count) AS downtime_count,
			       SUM(total_latency) AS total_latency
			FROM ServiceDaily
			WHERE date >= ? AND date < ?
			GROUP BY service_id
			HAVING COUNT(*) > 0
		 ), inc AS (
			SELECT service_id,
			       COUNT(*) AS incident_total
			FROM Incident
			GROUP BY service_id
		 )
		 SELECT a.service_id, a.uptime_count, a.downtime_count, a.total_latency,
		        COALESCE(i.incident_total, 0) AS incident_total,
		        0 AS incident_downtime_seconds,
		        NULL AS recorded_at, NULL AS closed_at
		 FROM agg a LEFT JOIN inc i ON i.service_id = a.service_id`,
		from, toExclusive)
	if err != nil {
		return nil, err
	}
	result := make([]*model.ServiceMonthly, 0, len(rows))
	for i := range rows {
		result = append(result, monthlyRowToModel(&rows[i]))
	}
	return result, nil
}

// ListServiceDailiesBetween 取指定日期区间内的每日汇总（用于增量归档与历史回填）
func (r *repo) ListServiceDailiesBetween(ctx context.Context, from, toExclusive string) (map[int64][]*model.ServiceDaily, error) {
	var rows []*model.ServiceDaily
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, service_id, date, uptime_count, downtime_count, total_latency, maintenance_count
		 FROM ServiceDaily WHERE date >= ? AND date < ? ORDER BY service_id ASC, date ASC`,
		from, toExclusive)
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]*model.ServiceDaily)
	for _, d := range rows {
		result[d.ServiceID] = append(result[d.ServiceID], d)
	}
	return result, nil
}

// CountCoveredDaysBetween 统计每个服务在区间内有探测数据的天数。
// 用 COUNT(DISTINCT date) 在库内聚合，避免为了取天数把整月明细搬进内存。
func (r *repo) CountCoveredDaysBetween(ctx context.Context, from, toExclusive string) (map[int64]int, error) {
	type row struct {
		ServiceID int64 `db:"service_id"`
		Days      int   `db:"days"`
	}
	var rows []row
	if err := r.db.SelectContext(ctx, &rows,
		`SELECT service_id, COUNT(DISTINCT date) AS days FROM ServiceDaily
		 WHERE date >= ? AND date < ? GROUP BY service_id`, from, toExclusive); err != nil {
		return nil, err
	}
	result := make(map[int64]int, len(rows))
	for _, r := range rows {
		result[r.ServiceID] = r.Days
	}
	return result, nil
}

// incidentDowntimeQuery 构造事件不可用时长聚合 SQL。
// 结束时间优先用 resolved_at（解决时刻），缺失时退回 updated_at，再缺失则算到当前时间。
//
// 月份夹取只做「时间重叠」判断，不对事件时间做字符串比较：Incident 的时间列是
// RFC3339 文本，可能带 Z 也可能带 +08:00 偏移，直接与月份边界做字符串比较会错判。
// 重叠时长用 julianday(结束) - julianday(开始) 再减去两端落在月份外的部分。
//
// 参数顺序：月份开始时间、月份结束时间、当前月结束时间、月份开始时间、月份结束时间。
func incidentDowntimeQuery() string {
	const incidentEnd = `COALESCE(
		NULLIF(i.resolved_at, ''),
		CASE WHEN i.status = 'resolved' THEN NULLIF(i.updated_at, '') END,
		datetime('now')
	)`

	return `SELECT service_id, CAST(SUM(downtime) AS INTEGER) AS total FROM (
		SELECT i.service_id AS service_id,
		       MAX(0, (
		         julianday(` + incidentEnd + `) - julianday(i.created_at)
		         - MAX(0, julianday(?) - julianday(i.created_at))
		         - MAX(0, julianday(` + incidentEnd + `) - julianday(?))
		       ) * 86400) AS downtime
		FROM Incident i
		WHERE julianday(i.created_at) < julianday(?)
		  AND julianday(` + incidentEnd + `) > julianday(?)
	) GROUP BY service_id`
}

// AggregateIncidentDowntime 按月统计每个服务由事件造成的不可用时长（秒）。
// from 为 "YYYY-MM-DD"（月份首日），toExclusive 为次月首日。
// 月份边界按北京时间（UTC+8）切分，与站点展示口径一致。
func (r *repo) AggregateIncidentDowntime(ctx context.Context, from, toExclusive string) (map[int64]int, error) {
	// 注意：两个边界必须用同一个时区，否则两者之间的时长会被算错
	const tz = "+08:00"
	fromTs := from + "T00:00:00" + tz
	toTs := toExclusive + "T00:00:00" + tz

	var rows []*incidentDowntimeRow
	if err := r.db.SelectContext(ctx, &rows, incidentDowntimeQuery(), fromTs, toTs, toTs, fromTs, toTs); err != nil {
		return nil, err
	}
	result := make(map[int64]int, len(rows))
	for _, row := range rows {
		result[row.ServiceID] = row.Total
	}
	return result, nil
}

// CountIncidentsBetween 统计区间内的事件数量（只统计根事件，不含合并进来的子事件）。
// 同样需要 RFC3339 格式的时间串。
func (r *repo) CountIncidentsBetween(ctx context.Context, from, toExclusive string) (map[int64]int, error) {
	type row struct {
		ServiceID int64 `db:"service_id"`
		Total     int   `db:"total"`
	}
	var rows []row
	err := r.db.SelectContext(ctx, &rows,
		`SELECT service_id, COUNT(*) AS total FROM Incident
		 WHERE parent_id IS NULL AND created_at >= ? AND created_at < ?
		 GROUP BY service_id`, from+"T00:00:00Z", toExclusive+"T00:00:00Z")
	if err != nil {
		return nil, err
	}
	result := make(map[int64]int, len(rows))
	for _, r := range rows {
		result[r.ServiceID] = r.Total
	}
	return result, nil
}

func monthlyRowToModel(row *monthlyRow) *model.ServiceMonthly {
	m := &model.ServiceMonthly{
		ID:                   row.ID,
		ServiceID:            row.ServiceID,
		Month:                row.Month,
		UptimeCount:          row.UptimeCount,
		DowntimeCount:        row.DowntimeCount,
		TotalLatency:         row.TotalLatency,
		IncidentTotal:        row.IncidentTotal,
		IncidentDowntimeSecs: row.IncidentDowntimeSec,
	}
	if row.RecordedAt != nil {
		m.RecordedAt = *row.RecordedAt
	}
	if row.ClosedAt != nil {
		m.ClosedAt = *row.ClosedAt
	}
	return m
}
