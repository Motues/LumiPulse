package sqlite

import (
	"context"
	"lumipluse-backend/internal/model"
	"time"
)

func (r *repo) GetOrCreateServiceDaily(ctx context.Context, serviceID int64, date string) (*model.ServiceDaily, error) {
	var d model.ServiceDaily
	err := r.db.GetContext(ctx, &d, "SELECT * FROM ServiceDaily WHERE service_id = ? AND date = ?", serviceID, date)
	if err == nil {
		return &d, nil
	}

	// Create new record
	_, err = r.db.ExecContext(ctx,
		"INSERT INTO ServiceDaily (service_id, date, uptime_count, downtime_count, total_latency) VALUES (?, ?, 0, 0, 0)",
		serviceID, date)
	if err != nil {
		return nil, err
	}

	err = r.db.GetContext(ctx, &d, "SELECT * FROM ServiceDaily WHERE service_id = ? AND date = ?", serviceID, date)
	return &d, err
}

func (r *repo) UpdateServiceDaily(ctx context.Context, d *model.ServiceDaily) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE ServiceDaily SET uptime_count=?, downtime_count=?, total_latency=? WHERE id=?",
		d.UptimeCount, d.DowntimeCount, d.TotalLatency, d.ID)
	return err
}

func (r *repo) GetServiceDailies(ctx context.Context, serviceID int64, days int) ([]*model.ServiceDaily, error) {
	var dailies []*model.ServiceDaily
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	err := r.db.SelectContext(ctx, &dailies,
		"SELECT * FROM ServiceDaily WHERE service_id = ? AND date >= ? ORDER BY date ASC",
		serviceID, since)
	return dailies, err
}

func (r *repo) DeleteOldServiceDailies(ctx context.Context, before string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM ServiceDaily WHERE date < ?", before)
	return err
}
