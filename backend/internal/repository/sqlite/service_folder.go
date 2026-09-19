package sqlite

import (
	"context"
	"time"

	"lumipluse-backend/internal/model"
)

// --- 服务分组（服务聚合文件夹）---

func (r *repo) CreateServiceFolder(ctx context.Context, f *model.ServiceFolder) error {
	now := time.Now().Format(time.RFC3339)
	query := `INSERT INTO ServiceFolder (name, description, show_on_homepage, sort_order, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, f.Name, f.Description, f.ShowOnHomepage, f.SortOrder, now, now)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	f.ID = id
	f.CreatedAt = now
	f.UpdatedAt = now
	return nil
}

func (r *repo) ListServiceFolders(ctx context.Context) ([]*model.ServiceFolder, error) {
	var folders []*model.ServiceFolder
	query := `SELECT * FROM ServiceFolder ORDER BY sort_order ASC, id ASC`
	err := r.db.SelectContext(ctx, &folders, query)
	return folders, err
}

func (r *repo) GetServiceFolder(ctx context.Context, id int64) (*model.ServiceFolder, error) {
	var f model.ServiceFolder
	err := r.db.GetContext(ctx, &f, "SELECT * FROM ServiceFolder WHERE id = ?", id)
	return &f, err
}

func (r *repo) UpdateServiceFolder(ctx context.Context, f *model.ServiceFolder) error {
	now := time.Now().Format(time.RFC3339)
	query := `UPDATE ServiceFolder SET name=?, description=?, show_on_homepage=?, sort_order=?, updated_at=? WHERE id=?`
	_, err := r.db.ExecContext(ctx, query, f.Name, f.Description, f.ShowOnHomepage, f.SortOrder, now, f.ID)
	if err != nil {
		return err
	}
	f.UpdatedAt = now
	return nil
}

// DeleteServiceFolder 删除分组。分组下的服务不会被删除，
// 由 Service.folder_id 的外键 / 显式置空逻辑变回「未分组」。
func (r *repo) DeleteServiceFolder(ctx context.Context, id int64) error {
	if _, err := r.db.ExecContext(ctx, "UPDATE Service SET folder_id = NULL WHERE folder_id = ?", id); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, "DELETE FROM ServiceFolder WHERE id = ?", id)
	return err
}

// CountServicesInFolder 统计分组内的服务数量（删除前提示、列表展示用）
func (r *repo) CountServicesInFolder(ctx context.Context, id int64) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM Service WHERE folder_id = ?", id)
	return count, err
}

// AssignServiceFolder 只更新服务所属分组。
// 单独一个方法而不是整行 UPDATE：管理端拖动分组时并发的心跳/状态变更不会被覆盖。
func (r *repo) AssignServiceFolder(ctx context.Context, serviceID int64, folderID *int64) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	_, err := r.db.ExecContext(ctx, "UPDATE Service SET folder_id = ?, updated_at = ? WHERE id = ?", folderID, now, serviceID)
	return err
}
