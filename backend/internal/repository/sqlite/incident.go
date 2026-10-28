package sqlite

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/pkg/utils"
	"lumipluse-backend/internal/repository"

	"github.com/jmoiron/sqlx"
)

func (r *repo) CreateIncident(ctx context.Context, inc *model.Incident) error {
	now := time.Now().Format(time.RFC3339)
	if inc.PublicHash == "" {
		inc.PublicHash = utils.GeneratePublicHash()
	}
	var parentID interface{}
	if inc.ParentID != nil {
		parentID = *inc.ParentID
	}
	query := `INSERT INTO Incident (public_hash, service_id, title, impact, status, affected_services, parent_id, resolved_at,
	                   root_cause, resolution, postmortem_url, postmortem_public, created_at, updated_at)
				  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, inc.PublicHash, inc.ServiceID, inc.Title, inc.Impact, inc.Status,
		inc.AffectedServices, parentID, nil, inc.RootCause, inc.Resolution, inc.PostmortemURL, inc.PostmortemPublic, now, now)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	inc.ID = id
	inc.CreatedAt = now
	inc.UpdatedAt = now
	if inc.Status == "resolved" {
		inc.ResolvedAt = &now
	}
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

// GetIncidentByHash 通过公开标识获取事件（公开页面使用，替代自增 ID）
func (r *repo) GetIncidentByHash(ctx context.Context, hash string) (*model.Incident, error) {
	var inc model.Incident
	err := r.db.GetContext(ctx, &inc, "SELECT * FROM Incident WHERE public_hash = ?", hash)
	if err != nil {
		return nil, err
	}
	children, err := r.ListChildIncidents(ctx, inc.ID)
	if err == nil {
		inc.Children = children
	}
	return &inc, nil
}

func (r *repo) ListIncidents(ctx context.Context, page, limit int) ([]*model.Incident, int64, error) {
	var total int64
	if err := r.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM Incident WHERE parent_id IS NULL"); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	var incidents []*model.Incident
	query := `SELECT * FROM Incident WHERE parent_id IS NULL ORDER BY created_at DESC LIMIT ? OFFSET ?`
	err := r.db.SelectContext(ctx, &incidents, query, limit, offset)
	return incidents, total, err
}

// ListIncidentsWithChildren 管理端列表：根事件与子事件一起返回，按时间倒序。
// 子事件会带 parent_id，前端据此展示"子事件 #N"角标。
func (r *repo) ListIncidentsWithChildren(ctx context.Context, page, limit int) ([]*model.Incident, int64, error) {
	var total int64
	if err := r.db.GetContext(ctx, &total, "SELECT COUNT(*) FROM Incident"); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	var incidents []*model.Incident
	query := `SELECT * FROM Incident ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
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
	// 优先返回根事件：子事件已经随父事件解决，个别历史遗留的孤儿子事件
	// 只有在没有根事件时才会被兜底返回。
	query := `SELECT * FROM Incident WHERE service_id = ? AND status != 'resolved'
				  ORDER BY (parent_id IS NULL) DESC, created_at DESC LIMIT 1`
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

// ListIncidentsSince 一次取回窗口内所有服务的事件，供批量每日统计使用（避免按服务逐个查询）。
func (r *repo) ListIncidentsSince(ctx context.Context, days int) ([]*model.Incident, error) {
	var incidents []*model.Incident
	since := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)
	query := `SELECT * FROM Incident WHERE created_at >= ? ORDER BY created_at ASC`
	err := r.db.SelectContext(ctx, &incidents, query, since)
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

	// Set resolved_at when first resolved, clear when re-opened
	var resolvedAt interface{}
	if inc.Status == "resolved" && inc.ResolvedAt == nil {
		resolvedAt = now
	} else if inc.Status != "resolved" {
		resolvedAt = nil
	} else {
		resolvedAt = *inc.ResolvedAt
	}

	query := `UPDATE Incident SET title=?, impact=?, status=?, affected_services=?, parent_id=?, resolved_at=?,
	             root_cause=?, resolution=?, postmortem_url=?, postmortem_public=?,
	             acknowledged=?, acknowledged_at=?, acknowledged_by=?, escalation_count=?, last_escalated_at=?,
	             updated_at=? WHERE id=?`
	_, err := r.db.ExecContext(ctx, query, inc.Title, inc.Impact, inc.Status, inc.AffectedServices, parentID, resolvedAt,
		inc.RootCause, inc.Resolution, inc.PostmortemURL, inc.PostmortemPublic,
		inc.Acknowledged, inc.AcknowledgedAt, inc.AcknowledgedBy, inc.EscalationCount, inc.LastEscalatedAt,
		now, inc.ID)
	if err != nil {
		return err
	}
	inc.UpdatedAt = now
	if inc.Status == "resolved" && inc.ResolvedAt == nil {
		inc.ResolvedAt = &now
	} else if inc.Status != "resolved" {
		inc.ResolvedAt = nil
	}
	return nil
}

// MarkIncidentEscalated 只写升级计数与时间，避免整行快照覆盖并发修改的状态。
// 条件里带上 status != 'resolved'：事件已解决时不再升级，也不会被写回 investigating。
func (r *repo) MarkIncidentEscalated(ctx context.Context, id int64, count int, lastEscalatedAt string) (bool, error) {
	now := time.Now().Format(time.RFC3339)
	res, err := r.db.ExecContext(ctx,
		`UPDATE Incident SET escalation_count = ?, last_escalated_at = ?, updated_at = ?
		 WHERE id = ? AND status != 'resolved'`,
		count, lastEscalatedAt, now, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
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
// 解决时同时补写 resolved_at：子事件是随父事件一起收口的，没有 resolved_at 会让
// 事件详情/导出里的「解决时间」为空，也会让事件维度的停机时长只能退回 updated_at。
func (r *repo) UpdateChildrenStatus(ctx context.Context, parentID int64, status string) error {
	now := time.Now().Format(time.RFC3339)
	if status == "resolved" {
		_, err := r.db.ExecContext(ctx,
			`UPDATE Incident SET status = ?, updated_at = ?,
			        resolved_at = COALESCE(NULLIF(resolved_at, ''), ?)
			 WHERE parent_id = ?`, status, now, now, parentID)
		return err
	}
	// 重新打开子事件时清掉解决时间，与 UpdateIncident 的口径保持一致
	_, err := r.db.ExecContext(ctx,
		`UPDATE Incident SET status = ?, updated_at = ?, resolved_at = NULL WHERE parent_id = ?`,
		status, now, parentID)
	return err
}

// incidentTxRow 事务内读取的事件状态快照（只取合并/拆分需要的列）
type incidentTxRow struct {
	ID               int64         `db:"id"`
	ServiceID        int64         `db:"service_id"`
	Status           string        `db:"status"`
	ParentID         sql.NullInt64 `db:"parent_id"`
	AffectedServices string        `db:"affected_services"`
}

const incidentTxColumns = `id, service_id, status, parent_id, affected_services`

func loadIncidentTx(ctx context.Context, tx *sqlx.Tx, id int64) (*incidentTxRow, error) {
	var row incidentTxRow
	err := tx.GetContext(ctx, &row, `SELECT `+incidentTxColumns+` FROM Incident WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// collectDescendants 收集 rootID 的整棵子树（不含 rootID 本身）。
// 限制层数与 visited，避免历史脏数据里的循环父子把这里变成死循环。
func collectDescendants(ctx context.Context, tx *sqlx.Tx, rootID int64) ([]int64, error) {
	seen := map[int64]bool{rootID: true}
	frontier := []int64{rootID}
	var result []int64

	for depth := 0; depth < 8 && len(frontier) > 0; depth++ {
		query, args, err := sqlx.In(`SELECT id FROM Incident WHERE parent_id IN (?)`, frontier)
		if err != nil {
			return nil, err
		}
		var ids []int64
		if err := tx.SelectContext(ctx, &ids, tx.Rebind(query), args...); err != nil {
			return nil, err
		}

		var next []int64
		for _, id := range ids {
			if seen[id] {
				continue
			}
			seen[id] = true
			result = append(result, id)
			next = append(next, id)
		}
		frontier = next
	}
	return result, nil
}

// MergeIncidents 见 repository 接口注释。
func (r *repo) MergeIncidents(ctx context.Context, targetID, sourceID int64) error {
	if targetID <= 0 || sourceID <= 0 || targetID == sourceID {
		return repository.ErrIncidentConflict
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	target, err := loadIncidentTx(ctx, tx, targetID)
	if err != nil {
		return err
	}
	source, err := loadIncidentTx(ctx, tx, sourceID)
	if err != nil {
		return err
	}

	// 父事件已解决时不能接收子事件：解决时已经同步过子事件，之后挂进来的
	// 事件不会再被解决，会在公开列表里彻底消失（幽灵事件）。
	if target.Status == "resolved" || source.Status == "resolved" {
		return repository.ErrIncidentConflict
	}
	// 双方都必须是根事件：这是防止 A→B 与 B→A 同时成立的第一道闸。
	if target.ParentID.Valid || source.ParentID.Valid {
		return repository.ErrIncidentConflict
	}

	// 合并后父子关系始终只有一层：UpdateChildrenStatus 只同步一层，
	// 出现孙子事件就会漏解决。因此把 source 的整棵子树直接挂到 target 下。
	descendants, err := collectDescendants(ctx, tx, source.ID)
	if err != nil {
		return err
	}
	for _, id := range descendants {
		if id == target.ID {
			return repository.ErrIncidentConflict
		}
	}

	now := time.Now().Format(time.RFC3339)

	// 条件更新：在写入的那一刻再校验一次「仍是根事件且未解决」。
	// 并发合并的第二个事务会在这里拿到 0 行并整笔回滚。
	res, err := tx.ExecContext(ctx,
		`UPDATE Incident SET parent_id = ?, updated_at = ?
		 WHERE id = ? AND parent_id IS NULL AND status != 'resolved'`,
		target.ID, now, source.ID)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return repository.ErrIncidentConflict
	}

	if len(descendants) > 0 {
		query, args, err := sqlx.In(
			`UPDATE Incident SET parent_id = ?, updated_at = ? WHERE id IN (?)`, target.ID, now, descendants)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, tx.Rebind(query), args...); err != nil {
			return err
		}
	}

	// 受影响服务取并集：父事件要覆盖 source 子树里的所有服务，
	// 否则解决判定与状态恢复都会漏掉这些服务。
	merged := appendMissingIDs(nil, target.ServiceID)
	merged = appendMissingIDs(merged, parseServiceIDList(target.AffectedServices)...)
	merged = appendMissingIDs(merged, source.ServiceID)
	merged = appendMissingIDs(merged, parseServiceIDList(source.AffectedServices)...)
	for _, id := range descendants {
		row, err := loadIncidentTx(ctx, tx, id)
		if err != nil {
			return err
		}
		merged = appendMissingIDs(merged, row.ServiceID)
		merged = appendMissingIDs(merged, parseServiceIDList(row.AffectedServices)...)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE Incident SET affected_services = ?, updated_at = ? WHERE id = ?`,
		joinServiceIDList(merged), now, target.ID); err != nil {
		return err
	}

	return tx.Commit()
}

// SplitIncident 见 repository 接口注释。
func (r *repo) SplitIncident(ctx context.Context, id int64) (int64, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	child, err := loadIncidentTx(ctx, tx, id)
	if err != nil {
		return 0, err
	}
	if !child.ParentID.Valid {
		return 0, repository.ErrIncidentConflict
	}
	parentID := child.ParentID.Int64

	parent, err := loadIncidentTx(ctx, tx, parentID)
	if err != nil {
		// 父事件已被删除：只解除引用，让子事件恢复成根事件
		if _, execErr := tx.ExecContext(ctx,
			`UPDATE Incident SET parent_id = NULL, updated_at = ? WHERE id = ?`, time.Now().Format(time.RFC3339), id); execErr != nil {
			return 0, execErr
		}
		return parentID, tx.Commit()
	}

	now := time.Now().Format(time.RFC3339)

	res, err := tx.ExecContext(ctx,
		`UPDATE Incident SET parent_id = NULL, updated_at = ? WHERE id = ? AND parent_id = ?`, now, id, parentID)
	if err != nil {
		return 0, err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return 0, repository.ErrIncidentConflict
	}

	// 重算父事件的受影响服务：拆出去的服务只有在该父事件下没有别的子事件
	// 还在声明时才移除，避免把父事件自己的服务（service_id）误删。
	siblings, err := childRows(ctx, tx, parentID)
	if err != nil {
		return 0, err
	}

	stillClaimed := map[int64]bool{parent.ServiceID: true}
	for _, sib := range siblings {
		stillClaimed[sib.ServiceID] = true
		for _, sid := range parseServiceIDList(sib.AffectedServices) {
			stillClaimed[sid] = true
		}
	}

	released := map[int64]bool{}
	for _, sid := range appendMissingIDs([]int64{child.ServiceID}, parseServiceIDList(child.AffectedServices)...) {
		released[sid] = true
	}

	kept := make([]int64, 0, len(released))
	for _, sid := range parseServiceIDList(parent.AffectedServices) {
		if released[sid] && !stillClaimed[sid] {
			continue
		}
		kept = appendMissingIDs(kept, sid)
	}
	kept = appendMissingIDs(kept, parent.ServiceID)

	if _, err := tx.ExecContext(ctx,
		`UPDATE Incident SET affected_services = ?, updated_at = ? WHERE id = ?`,
		joinServiceIDList(kept), now, parentID); err != nil {
		return 0, err
	}

	return parentID, tx.Commit()
}

func childRows(ctx context.Context, tx *sqlx.Tx, parentID int64) ([]*incidentTxRow, error) {
	var rows []*incidentTxRow
	err := tx.SelectContext(ctx, &rows,
		`SELECT `+incidentTxColumns+` FROM Incident WHERE parent_id = ?`, parentID)
	return rows, err
}

// BreakIncidentCycles 修复循环父子（A→B 且 B→A）。
//
// 旧版本的自动合并可能产生这种脏数据：两个事件都不再是根事件，事件列表与
// 仪表盘会把它们全部隐藏，检查器的 rootIncident 也只能停在环里、永远解决不掉。
// 这里把环上 id 最小的节点提为根（与人工修复的做法一致，不删除任何事件），
// 环就变成一条正常的父子链。返回修复的环数量。
func (r *repo) BreakIncidentCycles(ctx context.Context) (int, error) {
	rows, err := r.db.QueryxContext(ctx, `SELECT id, parent_id FROM Incident`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	parent := make(map[int64]int64)
	for rows.Next() {
		var id int64
		var parentID sql.NullInt64
		if err := rows.Scan(&id, &parentID); err != nil {
			return 0, err
		}
		if parentID.Valid {
			parent[id] = parentID.Int64
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	roots := make(map[int64]bool)
	for start := range parent {
		pos := make(map[int64]int)
		var path []int64
		cur := start

		for {
			if idx, ok := pos[cur]; ok {
				// path[idx:] 就是这个环，取 id 最小的节点作为新的根
				smallest := path[idx]
				for _, member := range path[idx:] {
					if member < smallest {
						smallest = member
					}
				}
				roots[smallest] = true
				break
			}
			next, ok := parent[cur]
			if !ok {
				break
			}
			pos[cur] = len(path)
			path = append(path, cur)
			cur = next
		}
	}

	if len(roots) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	now := time.Now().Format(time.RFC3339)
	for id := range roots {
		if _, err := tx.ExecContext(ctx,
			`UPDATE Incident SET parent_id = NULL, updated_at = ? WHERE id = ?`, now, id); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(roots), nil
}

// parseServiceIDList 解析逗号分隔的服务 ID（与 checker / handler 里的同名逻辑口径一致）
func parseServiceIDList(raw string) []int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var ids []int64
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if id, err := strconv.ParseInt(part, 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func joinServiceIDList(ids []int64) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return strings.Join(parts, ",")
}

func appendMissingIDs(existing []int64, ids ...int64) []int64 {
	seen := make(map[int64]bool, len(existing))
	for _, id := range existing {
		seen[id] = true
	}
	result := append([]int64{}, existing...)
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
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
