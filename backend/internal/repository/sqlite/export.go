package sqlite

import (
	"context"
	"lumipluse-backend/internal/model"
)

func (r *repo) ImportFullData(ctx context.Context, data *model.ImportData) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear tables in dependency order
	tables := []string{
		"Incident_Update",
		"Incident",
		"Maintenance",
		"Service",
	}
	for _, table := range tables {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return err
		}
	}

	// Import services
	for _, s := range data.Services {
		s.ID = 0 // reset ID to auto-increment
		query := `INSERT INTO Service (name, description, url, type, interval, status, is_active, sort_order, show_on_homepage, created_at, updated_at)
				  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		res, err := tx.ExecContext(ctx, query, s.Name, s.Description, s.URL, s.Type, s.Interval,
			s.Status, s.IsActive, s.SortOrder, s.ShowOnHomepage, s.CreatedAt, s.UpdatedAt)
		if err != nil {
			return err
		}
		s.ID, _ = res.LastInsertId()
	}

	// Import incidents — store old→new ID mapping
	oldToNew := make(map[int64]int64)
	parentRefs := make(map[int64]*int64) // newID → old parentID

	for _, inc := range data.Incidents {
		oldID := inc.ID
		parentRef := inc.ParentID // save before modifying
		inc.ID = 0
		inc.ParentID = nil

		query := `INSERT INTO Incident (service_id, title, impact, status, affected_services, parent_id, created_at, updated_at)
				  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
		res, err := tx.ExecContext(ctx, query, inc.ServiceID, inc.Title, inc.Impact, inc.Status,
			inc.AffectedServices, nil, inc.CreatedAt, inc.UpdatedAt)
		if err != nil {
			return err
		}
		newID, _ := res.LastInsertId()
		oldToNew[oldID] = newID
		if parentRef != nil {
			parentRefs[newID] = parentRef
		}
		inc.ID = newID
	}

	// Second pass: update parent_id references
	for newID, oldParentID := range parentRefs {
		if newParentID, ok := oldToNew[*oldParentID]; ok {
			if _, err := tx.ExecContext(ctx, "UPDATE Incident SET parent_id=? WHERE id=?", newParentID, newID); err != nil {
				return err
			}
		}
	}

	// Import incident updates
	for _, u := range data.IncidentUpdates {
		newIncID, ok := oldToNew[u.IncidentID]
		if !ok {
			continue // skip orphaned updates
		}
		u.ID = 0
		u.IncidentID = newIncID
		query := `INSERT INTO Incident_Update (incident_id, status, content, is_internal, created_at)
				  VALUES (?, ?, ?, ?, ?)`
		if _, err := tx.ExecContext(ctx, query, u.IncidentID, u.Status, u.Content, u.IsInternal, u.CreatedAt); err != nil {
			return err
		}
	}

	// Import maintenances
	for _, m := range data.Maintenances {
		m.ID = 0
		query := `INSERT INTO Maintenance (title, description, scheduled_start, scheduled_end, status, affected_services, created_at)
				  VALUES (?, ?, ?, ?, ?, ?, ?)`
		if _, err := tx.ExecContext(ctx, query, m.Title, m.Description, m.ScheduledStart, m.ScheduledEnd,
			m.Status, m.AffectedServices, m.CreatedAt); err != nil {
			return err
		}
	}

	return tx.Commit()
}
