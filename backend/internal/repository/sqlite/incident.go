package sqlite

import (
	"context"
	"lumipluse-backend/internal/model"
	"time"

	"github.com/jmoiron/sqlx"
)

func (r *repo) CreateIncident(ctx context.Context, inc *model.Incident) error {
	now := time.Now().Format(time.RFC3339)
	var parentID interface{}
	if inc.ParentID != nil {
		parentID = *inc.ParentID
	}
	query := `INSERT INTO Incident (service_id, title, impact, status, affected_services, parent_id, created_at, updated_at)
				  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, inc.ServiceID, inc.Title, inc.Impact, inc.Status, inc.AffectedServices, parentID, now, now)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	inc.ID = id
	inc.CreatedAt = now
	inc.UpdatedAt = now
	return nil
}

func (r *repo) GetIncident(ctx context.Context, id int64) (*model.Incident, error) {
	var inc model.Incident
	err := r.db.GetContext(ctx, &inc, "SELECT * FROM Incident WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	// Load children
	children, err := r.ListChildIncidents(ctx, id)
	if err == nil {
		inc.Children = children
	}
	return &inc, nil
}

func (r *repo) ListIncidents(ctx context.Context, page, limit int) ([]*model.Incident, int64, error) {
	var total int64
	if err := r.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM Incident WHERE status != 'resolved' AND parent_id IS NULL"); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	var incidents []*model.Incident
	query := `SELECT * FROM Incident WHERE status != 'resolved' AND parent_id IS NULL ORDER BY created_at DESC LIMIT ? OFFSET ?`
	err := r.db.SelectContext(ctx, &incidents, query, limit, offset)
	return incidents, total, err
}

func (r *repo) ListActiveIncidents(ctx context.Context) ([]*model.Incident, error) {
	var incidents []*model.Incident
	query := `SELECT * FROM Incident WHERE status != 'resolved' AND parent_id IS NULL ORDER BY created_at DESC`
	err := r.db.SelectContext(ctx, &incidents, query)
	return incidents, err
}

func (r *repo) GetActiveIncidentByService(ctx context.Context, serviceID int64) (*model.Incident, error) {
	var inc model.Incident
	query := `SELECT * FROM Incident WHERE service_id = ? AND status != 'resolved' ORDER BY created_at DESC LIMIT 1`
	err := r.db.GetContext(ctx, &inc, query, serviceID)
	if err != nil {
		return nil, err
	}
	return &inc, nil
}

func (r *repo) ListServiceIncidents(ctx context.Context, serviceID int64, days int) ([]*model.Incident, error) {
	var incidents []*model.Incident
	since := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)
	query := `SELECT * FROM Incident WHERE service_id = ? AND created_at >= ? ORDER BY created_at ASC`
	err := r.db.SelectContext(ctx, &incidents, query, serviceID, since)
	return incidents, err
}

func (r *repo) CountRecentIncidents(ctx context.Context, days int) (int64, int64, error) {
	since := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)
	var total int64
	if err := r.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM Incident WHERE created_at >= ? AND parent_id IS NULL", since); err != nil {
		return 0, 0, err
	}
	var resolved int64
	if err := r.db.GetContext(ctx, &resolved, "SELECT COUNT(*) FROM Incident WHERE created_at >= ? AND status = 'resolved' AND parent_id IS NULL", since); err != nil {
		return total, 0, err
	}
	return total, resolved, nil
}

func (r *repo) UpdateIncident(ctx context.Context, inc *model.Incident) error {
	now := time.Now().Format(time.RFC3339)
	var parentID interface{}
	if inc.ParentID != nil {
		parentID = *inc.ParentID
	}
	query := `UPDATE Incident SET title=?, impact=?, status=?, affected_services=?, parent_id=?, updated_at=? WHERE id=?`
	_, err := r.db.ExecContext(ctx, query, inc.Title, inc.Impact, inc.Status, inc.AffectedServices, parentID, now, inc.ID)
	if err != nil {
		return err
	}
	inc.UpdatedAt = now
	return nil
}

func (r *repo) DeleteIncident(ctx context.Context, id int64) error {
	// Unset parent_id for children before deleting parent
	r.db.ExecContext(ctx, "UPDATE Incident SET parent_id = NULL WHERE parent_id = ?", id)
	_, err := r.db.ExecContext(ctx, "DELETE FROM Incident WHERE id = ?", id)
	return err
}

// ListChildIncidents returns all child incidents for a given parent incident.
func (r *repo) ListChildIncidents(ctx context.Context, parentID int64) ([]*model.Incident, error) {
	var children []*model.Incident
	query := `SELECT * FROM Incident WHERE parent_id = ? ORDER BY created_at ASC`
	err := r.db.SelectContext(ctx, &children, query, parentID)
	return children, err
}

// UpdateChildrenStatus updates the status of all child incidents for a given parent.
func (r *repo) UpdateChildrenStatus(ctx context.Context, parentID int64, status string) error {
	now := time.Now().Format(time.RFC3339)
	query := `UPDATE Incident SET status = ?, updated_at = ? WHERE parent_id = ?`
	_, err := r.db.ExecContext(ctx, query, status, now, parentID)
	return err
}

func (r *repo) CreateIncidentUpdate(ctx context.Context, u *model.IncidentUpdate) error {
	now := time.Now().Format(time.RFC3339)
	query := `INSERT INTO Incident_Update (incident_id, status, content, is_internal, created_at)
				  VALUES (?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, u.IncidentID, u.Status, u.Content, u.IsInternal, now)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	u.ID = id
	u.CreatedAt = now
	return nil
}

func (r *repo) ListIncidentUpdates(ctx context.Context, incidentID int64) ([]*model.IncidentUpdate, error) {
	var updates []*model.IncidentUpdate
	query := `SELECT * FROM Incident_Update WHERE incident_id = ? ORDER BY created_at ASC`
	err := r.db.SelectContext(ctx, &updates, query, incidentID)
	return updates, err
}

func (r *repo) BatchListIncidentUpdates(ctx context.Context, incidentIDs []int64) (map[int64][]*model.IncidentUpdate, error) {
	if len(incidentIDs) == 0 {
		return make(map[int64][]*model.IncidentUpdate), nil
	}

	query := `SELECT * FROM Incident_Update WHERE incident_id IN (?) ORDER BY created_at ASC`
	query, args, err := sqlx.In(query, incidentIDs)
	if err != nil {
		return nil, err
	}
	query = r.db.Rebind(query)

	var updates []*model.IncidentUpdate
	if err := r.db.SelectContext(ctx, &updates, query, args...); err != nil {
		return nil, err
	}

	result := make(map[int64][]*model.IncidentUpdate)
	for _, u := range updates {
		result[u.IncidentID] = append(result[u.IncidentID], u)
	}
	return result, nil
}

func (r *repo) UpdateIncidentUpdate(ctx context.Context, u *model.IncidentUpdate) error {
	query := `UPDATE Incident_Update SET status = ?, content = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, u.Status, u.Content, u.ID)
	return err
}

func (r *repo) DeleteIncidentUpdate(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM Incident_Update WHERE id = ?", id)
	return err
}

func (r *repo) ListIncidentsByServer(ctx context.Context, serverID int64) ([]*model.Incident, error) {
	var incidents []*model.Incident
	query := `SELECT i.* FROM Incident i
			  INNER JOIN ProbeTask pt ON pt.service_id = i.service_id
			  WHERE pt.server_id = ? AND i.status != 'resolved'
			  ORDER BY i.created_at ASC`
	err := r.db.SelectContext(ctx, &incidents, query, serverID)
	return incidents, err
}
