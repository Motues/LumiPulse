package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/repository"

	"github.com/jmoiron/sqlx"
)

// 事件合并 / 拆分的仓储层回归测试。
// 线上事故的根因是「同一秒两个服务同时故障 → 两次自动合并互相把对方挂成子事件」：
// 两个事件都不再是根事件，事件列表与仪表盘会把它们全部隐藏。这里保证：
//   - 合并前必须重新校验双方仍是未解决的根事件；
//   - 合并后父子只有一层（孙子事件会被统一上提，否则解决时漏同步）；
//   - 受影响服务取并集，不会丢服务；
//   - 拆分时把只有该子事件还在声明的服务从父事件里摘掉。

func newTestRepo(t *testing.T) (*repo, *sqlx.DB) {
	t.Helper()

	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sqlx.Connect("sqlite", dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	return &repo{db: db}, db
}

func newTestService(t *testing.T, r *repo, name string) *model.Service {
	t.Helper()

	svc := &model.Service{Name: name, URL: "http://" + name, Type: "http", Interval: 60, IsActive: true}
	if err := r.CreateService(context.Background(), svc); err != nil {
		t.Fatalf("create service %s: %v", name, err)
	}
	return svc
}

func newTestIncident(t *testing.T, r *repo, svc *model.Service, status, affected string) *model.Incident {
	t.Helper()

	inc := &model.Incident{
		ServiceID:        svc.ID,
		Title:            "incident for " + svc.Name,
		Impact:           "major",
		Status:           status,
		AffectedServices: affected,
	}
	if err := r.CreateIncident(context.Background(), inc); err != nil {
		t.Fatalf("create incident: %v", err)
	}
	return inc
}

// incidentParents 读取所有事件的 parent_id（NULL 记为 0）
func incidentParents(t *testing.T, db *sqlx.DB) map[int64]int64 {
	t.Helper()

	rows, err := db.Queryx(`SELECT id, parent_id FROM Incident`)
	if err != nil {
		t.Fatalf("query parents: %v", err)
	}
	defer rows.Close()

	parents := make(map[int64]int64)
	for rows.Next() {
		var id int64
		var parent sql.NullInt64
		if err := rows.Scan(&id, &parent); err != nil {
			t.Fatalf("scan parent: %v", err)
		}
		if parent.Valid {
			parents[id] = parent.Int64
		} else {
			parents[id] = 0
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate parents: %v", err)
	}
	return parents
}

// assertNoCycle 断言父子关系里不存在环
func assertNoCycle(t *testing.T, db *sqlx.DB) {
	t.Helper()

	parents := incidentParents(t, db)
	for id := range parents {
		seen := map[int64]bool{id: true}
		for cur := id; parents[cur] != 0; cur = parents[cur] {
			next := parents[cur]
			if seen[next] {
				t.Fatalf("事件 #%d 的父子链上出现环: %v", id, parents)
			}
			seen[next] = true
		}
	}
}

func TestMergeIncidentsRejectsResolvedTarget(t *testing.T) {
	r, db := newTestRepo(t)
	svc1 := newTestService(t, r, "svc-1")
	svc2 := newTestService(t, r, "svc-2")

	target := newTestIncident(t, r, svc1, "resolved", "1")
	source := newTestIncident(t, r, svc2, "investigating", "2")

	err := r.MergeIncidents(context.Background(), target.ID, source.ID)
	if !errors.Is(err, repository.ErrIncidentConflict) {
		t.Fatalf("合并进已解决事件应被拒绝, got err=%v", err)
	}

	parents := incidentParents(t, db)
	if parents[source.ID] != 0 {
		t.Fatalf("被拒绝的合并不应改动父子关系: %v", parents)
	}
}

func TestMergeIncidentsFlattensSubtree(t *testing.T) {
	r, _ := newTestRepo(t)
	svc1 := newTestService(t, r, "svc-1")
	svc2 := newTestService(t, r, "svc-2")
	svc3 := newTestService(t, r, "svc-3")

	target := newTestIncident(t, r, svc1, "investigating", "1")
	source := newTestIncident(t, r, svc2, "investigating", "2")
	// source 下面先挂一个子事件（模拟「已经合并过一次的根事件再被合并」）
	if err := r.MergeIncidents(context.Background(), source.ID, newTestIncident(t, r, svc3, "investigating", "3").ID); err != nil {
		t.Fatalf("准备子树失败: %v", err)
	}

	var grandchildID int64
	if err := r.db.Get(&grandchildID, `SELECT id FROM Incident WHERE parent_id = ?`, source.ID); err != nil {
		t.Fatalf("读取孙事件失败: %v", err)
	}

	if err := r.MergeIncidents(context.Background(), target.ID, source.ID); err != nil {
		t.Fatalf("合并失败: %v", err)
	}

	parents := incidentParents(t, r.db)
	if parents[source.ID] != target.ID {
		t.Fatalf("source 应挂到 target 下: %v", parents)
	}
	if parents[grandchildID] != target.ID {
		t.Fatalf("孙事件应被上提到 target 下（否则解决时漏同步）: %v", parents)
	}

	// 受影响服务必须是并集：1 + 2 + 3
	var affected string
	if err := r.db.Get(&affected, `SELECT affected_services FROM Incident WHERE id = ?`, target.ID); err != nil {
		t.Fatalf("读取 affected_services 失败: %v", err)
	}
	got := parseServiceIDList(affected)
	want := []int64{svc1.ID, svc2.ID, svc3.ID}
	if len(got) != len(want) {
		t.Fatalf("affected_services 应包含全部服务: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("affected_services 不匹配: got=%v want=%v", got, want)
		}
	}

	assertNoCycle(t, r.db)
}

func TestMergeIncidentsRejectsOppositeMerge(t *testing.T) {
	r, _ := newTestRepo(t)
	svc1 := newTestService(t, r, "svc-1")
	svc2 := newTestService(t, r, "svc-2")

	a := newTestIncident(t, r, svc1, "investigating", "1")
	b := newTestIncident(t, r, svc2, "investigating", "2")

	if err := r.MergeIncidents(context.Background(), a.ID, b.ID); err != nil {
		t.Fatalf("第一次合并应当成功: %v", err)
	}
	// 反向合并必须被拒绝，否则会形成 A→B 且 B→A 的循环父子
	if err := r.MergeIncidents(context.Background(), b.ID, a.ID); !errors.Is(err, repository.ErrIncidentConflict) {
		t.Fatalf("反向合并应被拒绝, got err=%v", err)
	}
	assertNoCycle(t, r.db)
}

func TestConcurrentMergesNeverCreateCycle(t *testing.T) {
	r, _ := newTestRepo(t)
	svc1 := newTestService(t, r, "svc-1")
	svc2 := newTestService(t, r, "svc-2")

	a := newTestIncident(t, r, svc1, "investigating", "1")
	b := newTestIncident(t, r, svc2, "investigating", "2")

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 2)
	pairs := [][2]int64{{a.ID, b.ID}, {b.ID, a.ID}}

	for i, pair := range pairs {
		wg.Add(1)
		go func(i int, targetID, sourceID int64) {
			defer wg.Done()
			<-start
			errs[i] = r.MergeIncidents(context.Background(), targetID, sourceID)
		}(i, pair[0], pair[1])
	}
	close(start)
	wg.Wait()

	// 关键不变量：无论两笔并发怎么交错，都不能出现循环父子
	assertNoCycle(t, r.db)

	parents := incidentParents(t, r.db)
	if parents[a.ID] != 0 && parents[b.ID] != 0 {
		t.Fatalf("两个事件不能同时成为子事件: %v", parents)
	}
	if errs[0] == nil && errs[1] == nil {
		t.Fatalf("两次方向相反的合并不应同时成功: %v", parents)
	}
}

func TestSplitIncidentReleasesService(t *testing.T) {
	r, _ := newTestRepo(t)
	svc1 := newTestService(t, r, "svc-1")
	svc2 := newTestService(t, r, "svc-2")

	parent := newTestIncident(t, r, svc1, "investigating", "1,2")
	child := newTestIncident(t, r, svc2, "investigating", "2")
	if err := r.MergeIncidents(context.Background(), parent.ID, child.ID); err != nil {
		t.Fatalf("合并失败: %v", err)
	}

	parentID, err := r.SplitIncident(context.Background(), child.ID)
	if err != nil {
		t.Fatalf("拆分失败: %v", err)
	}
	if parentID != parent.ID {
		t.Fatalf("返回的父事件 ID 不正确: got=%d want=%d", parentID, parent.ID)
	}

	parents := incidentParents(t, r.db)
	if parents[child.ID] != 0 {
		t.Fatalf("拆分后子事件应恢复成根事件: %v", parents)
	}

	var affected string
	if err := r.db.Get(&affected, `SELECT affected_services FROM Incident WHERE id = ?`, parent.ID); err != nil {
		t.Fatalf("读取 affected_services 失败: %v", err)
	}
	if got := parseServiceIDList(affected); len(got) != 1 || got[0] != svc1.ID {
		t.Fatalf("父事件应只剩自己的服务 %d, got=%v", svc1.ID, got)
	}

	// 再拆一次应当被拒绝（已经不是子事件）
	if _, err := r.SplitIncident(context.Background(), child.ID); !errors.Is(err, repository.ErrIncidentConflict) {
		t.Fatalf("重复拆分应被拒绝, got err=%v", err)
	}
}

func TestUpdateChildrenStatusSetsResolvedAt(t *testing.T) {
	r, _ := newTestRepo(t)
	svc1 := newTestService(t, r, "svc-1")
	svc2 := newTestService(t, r, "svc-2")

	parent := newTestIncident(t, r, svc1, "investigating", "1")
	child := newTestIncident(t, r, svc2, "investigating", "2")
	if err := r.MergeIncidents(context.Background(), parent.ID, child.ID); err != nil {
		t.Fatalf("合并失败: %v", err)
	}

	if err := r.UpdateChildrenStatus(context.Background(), parent.ID, "resolved"); err != nil {
		t.Fatalf("同步子事件状态失败: %v", err)
	}

	var resolvedAt sql.NullString
	var status string
	if err := r.db.QueryRow(`SELECT status, resolved_at FROM Incident WHERE id = ?`, child.ID).Scan(&status, &resolvedAt); err != nil {
		t.Fatalf("读取子事件失败: %v", err)
	}
	if status != "resolved" {
		t.Fatalf("子事件状态应为 resolved, got=%s", status)
	}
	if !resolvedAt.Valid || resolvedAt.String == "" {
		t.Fatalf("子事件随父事件解决时必须补写 resolved_at，否则详情/导出里的解决时间为空")
	}

	// 重新打开时应清掉解决时间
	if err := r.UpdateChildrenStatus(context.Background(), parent.ID, "investigating"); err != nil {
		t.Fatalf("重新打开子事件失败: %v", err)
	}
	if err := r.db.QueryRow(`SELECT resolved_at FROM Incident WHERE id = ?`, child.ID).Scan(&resolvedAt); err != nil {
		t.Fatalf("再次读取子事件失败: %v", err)
	}
	if resolvedAt.Valid && resolvedAt.String != "" {
		t.Fatalf("重新打开子事件时应清空 resolved_at, got=%v", resolvedAt)
	}
}

// TestBreakIncidentCycles 历史脏数据里的循环父子必须能被修复：
// 两个事件互相为父时都不再是根事件，列表与仪表盘会把它们全部隐藏，
// 检查器的 rootIncident 也只能停在环里、永远解决不掉。
func TestBreakIncidentCycles(t *testing.T) {
	r, db := newTestRepo(t)
	svc1 := newTestService(t, r, "svc-1")
	svc2 := newTestService(t, r, "svc-2")

	a := newTestIncident(t, r, svc1, "investigating", "1")
	b := newTestIncident(t, r, svc2, "investigating", "2")

	// 仓储层的合并接口已经不允许造环，这里直接写库模拟旧版本留下的脏数据
	ctx := context.Background()
	if _, err := db.Exec(`UPDATE Incident SET parent_id = ? WHERE id = ?`, b.ID, a.ID); err != nil {
		t.Fatalf("造环失败: %v", err)
	}
	if _, err := db.Exec(`UPDATE Incident SET parent_id = ? WHERE id = ?`, a.ID, b.ID); err != nil {
		t.Fatalf("造环失败: %v", err)
	}
	parents := incidentParents(t, db)
	if parents[a.ID] != b.ID || parents[b.ID] != a.ID {
		t.Fatalf("造环失败: %v", parents)
	}

	n, err := r.BreakIncidentCycles(ctx)
	if err != nil {
		t.Fatalf("修复循环失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 repaired cycle, got %d", n)
	}

	assertNoCycle(t, db)
	parents = incidentParents(t, db)
	if parents[a.ID] != 0 {
		t.Fatalf("id 最小的节点应被提为根事件: %v", parents)
	}
	if parents[b.ID] != a.ID {
		t.Fatalf("环上另一个事件应保留为它的子事件: %v", parents)
	}

	// 再跑一次不应产生任何改动（已经无环）
	if n, err := r.BreakIncidentCycles(ctx); err != nil || n != 0 {
		t.Fatalf("重复修复应当无改动: n=%d err=%v", n, err)
	}
}
