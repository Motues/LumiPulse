package sqlite

import (
	"context"
	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"
	"strconv"
	"strings"
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
		"ServiceFolder",
	}
	for _, table := range tables {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return err
		}
	}

	// Import service folders first — 服务需要引用分组 ID。
	// 导出文件里的分组 ID 是「导出内引用号」（见 handler.AdminExport），
	// 这里按引用号建立「引用号 → 新自增 ID」映射，稍后据此重写服务的 folder_id。
	folderIDMap := make(map[int64]int64)
	for _, f := range data.ServiceFolders {
		ref := f.ID
		f.ID = 0
		query := `INSERT INTO ServiceFolder (name, description, show_on_homepage, sort_order, created_at, updated_at)
				  VALUES (?, ?, ?, ?, ?, ?)`
		res, err := tx.ExecContext(ctx, query, f.Name, f.Description, f.ShowOnHomepage, f.SortOrder, f.CreatedAt, f.UpdatedAt)
		if err != nil {
			return err
		}
		newID, _ := res.LastInsertId()
		f.ID = newID
		if ref > 0 {
			folderIDMap[ref] = newID
		}
	}

	// Import services — 记录「旧服务 ID → 新服务 ID」映射。
	// 服务是清空后重新插入的，自增 ID 会顺延，而事件 / 维护计划里存的
	// service_id 与 affected_services 仍是旧 ID，必须一起重写，否则外键校验直接失败。
	serviceIDMap := make(map[int64]int64)
	for _, s := range data.Services {
		oldID := s.ID
		s.ID = 0 // reset ID to auto-increment
		// 主键会被重置，公开标识缺失则重新生成
		if s.PublicHash == "" {
			s.PublicHash = utils.GeneratePublicHash()
		}
		// 分组引用号同样要重映射；找不到映射说明分组不在导入数据里，按未分组处理
		if s.FolderID != nil {
			if newID, ok := folderIDMap[*s.FolderID]; ok {
				s.FolderID = &newID
			} else {
				s.FolderID = nil
			}
		}
		query := `INSERT INTO Service (name, description, url, type, interval, status, is_active, sort_order, show_on_homepage, insecure_skip_verify, timeout_seconds, public_hash,
					  http_method, http_headers, http_body, expect_status, expect_keyword, folder_id, homepage_blocks, created_at, updated_at)
				  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		res, err := tx.ExecContext(ctx, query, s.Name, s.Description, s.URL, s.Type, s.Interval,
			s.Status, s.IsActive, s.SortOrder, s.ShowOnHomepage, s.InsecureSkipVerify, s.TimeoutSeconds, s.PublicHash,
			s.HTTPMethod, s.HTTPHeaders, s.HTTPBody, s.ExpectStatus, s.ExpectKeyword, s.FolderID, s.HomepageBlocks, s.CreatedAt, s.UpdatedAt)
		if err != nil {
			return err
		}
		s.ID, _ = res.LastInsertId()
		serviceIDMap[oldID] = s.ID
	}

	// Import incidents — store old→new ID mapping
	oldToNew := make(map[int64]int64)
	parentRefs := make(map[int64]*int64) // newID → old parentID

	for _, inc := range data.Incidents {
		oldID := inc.ID
		parentRef := inc.ParentID // save before modifying
		inc.ID = 0
		inc.ParentID = nil
		// 导入时主键会被重置，公开标识缺失则重新生成
		if inc.PublicHash == "" {
			inc.PublicHash = utils.GeneratePublicHash()
		}
		// 服务 ID 与受影响服务列表都要映射到新 ID
		inc.ServiceID = remapServiceID(serviceIDMap, inc.ServiceID)
		inc.AffectedServices = remapAffectedServices(serviceIDMap, inc.AffectedServices)

		query := `INSERT INTO Incident (public_hash, service_id, title, impact, status, affected_services, parent_id, resolved_at,
		                   root_cause, resolution, postmortem_url, postmortem_public, created_at, updated_at)
				  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		res, err := tx.ExecContext(ctx, query, inc.PublicHash, inc.ServiceID, inc.Title, inc.Impact, inc.Status,
			inc.AffectedServices, nil, inc.ResolvedAt, inc.RootCause, inc.Resolution, inc.PostmortemURL,
			inc.PostmortemPublic, inc.CreatedAt, inc.UpdatedAt)
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
		m.AffectedServices = remapAffectedServices(serviceIDMap, m.AffectedServices)
		query := `INSERT INTO Maintenance (title, description, scheduled_start, scheduled_end, status, affected_services, created_at)
				  VALUES (?, ?, ?, ?, ?, ?, ?)`
		if _, err := tx.ExecContext(ctx, query, m.Title, m.Description, m.ScheduledStart, m.ScheduledEnd,
			m.Status, m.AffectedServices, m.CreatedAt); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// remapServiceID 把旧服务 ID 映射为新 ID；找不到映射时返回 0（调用方按无关联处理）
func remapServiceID(mapping map[int64]int64, oldID int64) int64 {
	if newID, ok := mapping[oldID]; ok {
		return newID
	}
	return 0
}

// remapAffectedServices 重写逗号分隔的服务 ID 列表，丢弃无法映射的旧 ID
func remapAffectedServices(mapping map[int64]int64, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parts := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		oldID, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			continue
		}
		if newID, ok := mapping[oldID]; ok {
			parts = append(parts, strconv.FormatInt(newID, 10))
		}
	}
	return strings.Join(parts, ",")
}
