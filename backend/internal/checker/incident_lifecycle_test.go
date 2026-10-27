package checker

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"lumipluse-backend/internal/model"
	"lumipluse-backend/internal/repository"
	"lumipluse-backend/internal/repository/sqlite"

	"github.com/jmoiron/sqlx"
)

// 事件生命周期的回归测试：覆盖「同一秒双服务故障 → 自动合并」、「恢复只通知一次」、
// 「历史 affected_services 为空」、「阈值」与「合并时间窗」等曾经导致线上事故的路径。

func testRepo(t *testing.T) (repository.Repository, *sqlx.DB) {
	t.Helper()

	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sqlx.Connect("sqlite", dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := sqlite.InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	return sqlite.NewRepository(db), db
}

func testService(t *testing.T, repo repository.Repository, name string) *model.Service {
	t.Helper()

	svc := &model.Service{Name: name, URL: "http://" + name, Type: "http", Interval: 60, IsActive: true}
	if err := repo.CreateService(context.Background(), svc); err != nil {
		t.Fatalf("create service %s: %v", name, err)
	}
	return svc
}

func testProbeTask(t *testing.T, repo repository.Repository, serviceID, serverID int64, trigger int) *model.ProbeTask {
	t.Helper()

	task := &model.ProbeTask{ServiceID: serviceID, ServerID: &serverID, TriggerCount: trigger, IsActive: true}
	if err := repo.CreateProbeTask(context.Background(), task); err != nil {
		t.Fatalf("create probe task: %v", err)
	}
	return task
}

func testServer(t *testing.T, repo repository.Repository, threshold int) *model.Server {
	t.Helper()

	srv := &model.Server{Name: "test-server", AutoMerge: true, AutoMergeThreshold: threshold}
	if err := repo.CreateServer(context.Background(), srv); err != nil {
		t.Fatalf("create server: %v", err)
	}
	return srv
}

func allIncidents(t *testing.T, repo repository.Repository) []*model.Incident {
	t.Helper()

	incidents, _, err := repo.ListIncidentsWithChildren(context.Background(), 1, 100)
	if err != nil {
		t.Fatalf("list incidents: %v", err)
	}
	return incidents
}

func countUpdates(t *testing.T, db *sqlx.DB, status string) int {
	t.Helper()

	var n int
	if err := db.Get(&n, "SELECT COUNT(*) FROM Incident_Update WHERE status = ?", status); err != nil {
		t.Fatalf("count updates: %v", err)
	}
	return n
}

func feed(t *testing.T, hc *HealthChecker, svc *model.Service, task *model.ProbeTask, up bool, times int) {
	t.Helper()

	for i := 0; i < times; i++ {
		hc.trackProbeState(context.Background(), svc, task, up)
	}
}

// TestConcurrentFailuresDoNotCreateCircularMerge 复现线上事故：同一秒内两个服务
// 同时达到失败阈值。修复后必须始终只有一个根事件，且任何子事件的父事件都是根，
// 不允许出现 A→B→A 的循环父子，也不允许 affected_services 丢服务。
func TestConcurrentFailuresDoNotCreateCircularMerge(t *testing.T) {
	ctx := context.Background()

	for iter := 0; iter < 5; iter++ {
		repo, db := testRepo(t)
		srv := testServer(t, repo, 1)
		svcA := testService(t, repo, "a.example")
		svcB := testService(t, repo, "b.example")
		taskA := testProbeTask(t, repo, svcA.ID, srv.ID, 5)
		taskB := testProbeTask(t, repo, svcB.ID, srv.ID, 5)

		hc := New(repo, false, 8)

		start := make(chan struct{})
		var wg sync.WaitGroup
		run := func(svc *model.Service, task *model.ProbeTask) {
			defer wg.Done()
			<-start
			feed(t, hc, svc, task, false, 5)
		}
		wg.Add(2)
		go run(svcA, taskA)
		go run(svcB, taskB)
		close(start)
		wg.Wait()

		incidents := allIncidents(t, repo)
		if len(incidents) != 2 {
			t.Fatalf("iteration %d: expected 2 incidents, got %d", iter, len(incidents))
		}

		roots := 0
		var root *model.Incident
		for _, inc := range incidents {
			if inc.ParentID == nil {
				roots++
				root = inc
				continue
			}
			parent, err := repo.GetIncident(ctx, *inc.ParentID)
			if err != nil {
				t.Fatalf("iteration %d: child %d has missing parent: %v", iter, inc.ID, err)
			}
			if parent.ParentID != nil {
				t.Fatalf("iteration %d: circular parent chain: %d -> %d -> %d",
					iter, inc.ID, parent.ID, *parent.ParentID)
			}
		}
		if roots != 1 {
			t.Fatalf("iteration %d: expected exactly 1 root incident, got %d", iter, roots)
		}

		ids := parseAffectedServices(root.AffectedServices)
		if len(ids) != 2 {
			t.Fatalf("iteration %d: root affected_services lost services: %q", iter, root.AffectedServices)
		}

		// 两个服务恢复后：全部事件 resolved，且只有一个恢复通知
		feed(t, hc, svcA, taskA, true, 5)
		feed(t, hc, svcB, taskB, true, 5)

		for _, inc := range allIncidents(t, repo) {
			if inc.Status != "resolved" {
				t.Fatalf("iteration %d: incident %d still %s after recovery", iter, inc.ID, inc.Status)
			}
		}
		if n := countUpdates(t, db, "resolved"); n != 1 {
			t.Fatalf("iteration %d: expected 1 recovery notification, got %d", iter, n)
		}

		// 继续健康探测，不允许再产生任何重复的恢复事件
		feed(t, hc, svcA, taskA, true, 10)
		feed(t, hc, svcB, taskB, true, 10)
		if n := countUpdates(t, db, "resolved"); n != 1 {
			t.Fatalf("iteration %d: recovery notification repeated, got %d updates", iter, n)
		}
	}
}

// TestResolveNotifiesOnce 单服务故障恢复后只允许一条 resolved 进展（= 一封恢复邮件）。
func TestResolveNotifiesOnce(t *testing.T) {
	repo, db := testRepo(t)
	srv := testServer(t, repo, 1)
	svc := testService(t, repo, "single.example")
	task := testProbeTask(t, repo, svc.ID, srv.ID, 5)

	hc := New(repo, false, 4)
	feed(t, hc, svc, task, false, 5)
	feed(t, hc, svc, task, true, 5)

	if n := countUpdates(t, db, "resolved"); n != 1 {
		t.Fatalf("expected 1 resolved update, got %d", n)
	}

	feed(t, hc, svc, task, true, 20)
	if n := countUpdates(t, db, "resolved"); n != 1 {
		t.Fatalf("recovery kept repeating: %d resolved updates", n)
	}

	incidents := allIncidents(t, repo)
	if len(incidents) != 1 || incidents[0].Status != "resolved" {
		t.Fatalf("expected one resolved incident, got %+v", incidents)
	}
}

// TestLegacyIncidentWithoutAffectedServices 历史事件的 affected_services 为空时，
// 恢复也只会触发一次解决（旧实现会每分钟重复“解决”并发送恢复邮件）。
func TestLegacyIncidentWithoutAffectedServices(t *testing.T) {
	repo, db := testRepo(t)
	srv := testServer(t, repo, 1)
	svc := testService(t, repo, "legacy.example")
	task := testProbeTask(t, repo, svc.ID, srv.ID, 5)

	now := time.Now().Format(time.RFC3339)
	res, err := db.Exec(`INSERT INTO Incident (service_id, title, impact, status, affected_services, created_at, updated_at)
	                     VALUES (?, 'legacy incident', 'major', 'investigating', '', ?, ?)`,
		svc.ID, now, now)
	if err != nil {
		t.Fatalf("insert legacy incident: %v", err)
	}
	incID, _ := res.LastInsertId()

	hc := New(repo, false, 4)
	// 故障阶段会把现存事件挂到内存状态上（不会新建事件）
	feed(t, hc, svc, task, false, 5)
	if incidents := allIncidents(t, repo); len(incidents) != 1 {
		t.Fatalf("legacy incident should be reused, got %d incidents", len(incidents))
	}

	feed(t, hc, svc, task, true, 5)
	feed(t, hc, svc, task, true, 20)

	if n := countUpdates(t, db, "resolved"); n != 1 {
		t.Fatalf("legacy incident resolved %d times, want 1", n)
	}
	inc, err := repo.GetIncident(context.Background(), incID)
	if err != nil {
		t.Fatalf("get incident: %v", err)
	}
	if inc.Status != "resolved" {
		t.Fatalf("legacy incident status = %s, want resolved", inc.Status)
	}
}

// TestMergedChildResolvedWithParent 子事件必须跟随父事件一起解决，
// 并且管理端列表能看到子事件（公开列表仍然只显示根事件）。
func TestMergedChildResolvedWithParent(t *testing.T) {
	repo, db := testRepo(t)
	srv := testServer(t, repo, 1)
	svcA := testService(t, repo, "root.example")
	svcB := testService(t, repo, "child.example")
	taskA := testProbeTask(t, repo, svcA.ID, srv.ID, 5)
	taskB := testProbeTask(t, repo, svcB.ID, srv.ID, 5)

	hc := New(repo, false, 4)
	feed(t, hc, svcA, taskA, false, 5)
	feed(t, hc, svcB, taskB, false, 5)

	incidents := allIncidents(t, repo)
	if len(incidents) != 2 {
		t.Fatalf("expected 2 incidents, got %d", len(incidents))
	}
	children := 0
	for _, inc := range incidents {
		if inc.ParentID != nil {
			children++
		}
	}
	if children != 1 {
		t.Fatalf("expected exactly 1 child incident, got %d", children)
	}

	// 管理端列表包含子事件，公开列表不包含
	if _, total, err := repo.ListIncidents(context.Background(), 1, 20); err != nil || total != 1 {
		t.Fatalf("public incident list total = %d (err=%v), want 1", total, err)
	}

	feed(t, hc, svcA, taskA, true, 5)
	feed(t, hc, svcB, taskB, true, 5)

	for _, inc := range allIncidents(t, repo) {
		if inc.Status != "resolved" {
			t.Fatalf("incident %d (parent=%v) still %s after recovery", inc.ID, inc.ParentID, inc.Status)
		}
	}
	if n := countUpdates(t, db, "resolved"); n != 1 {
		t.Fatalf("expected 1 recovery notification, got %d", n)
	}

	// 之后继续健康探测也不能再补发
	feed(t, hc, svcA, taskA, true, 10)
	feed(t, hc, svcB, taskB, true, 10)
	if n := countUpdates(t, db, "resolved"); n != 1 {
		t.Fatalf("recovery notification repeated for merged incident: %d", n)
	}
}

// TestRestartThenRecoverNotifiesOnce 进程重启会丢失内存状态，重启后的兜底解决
// 只能针对根事件，并且同样只允许发送一次恢复通知。
func TestRestartThenRecoverNotifiesOnce(t *testing.T) {
	repo, db := testRepo(t)
	srv := testServer(t, repo, 1)
	svcA := testService(t, repo, "restart-a.example")
	svcB := testService(t, repo, "restart-b.example")
	taskA := testProbeTask(t, repo, svcA.ID, srv.ID, 5)
	taskB := testProbeTask(t, repo, svcB.ID, srv.ID, 5)

	hc := New(repo, false, 4)
	feed(t, hc, svcA, taskA, false, 5)
	feed(t, hc, svcB, taskB, false, 5)

	// 模拟进程重启：换一个全新的 checker，内存里的 autoIncidentID 全部丢失
	hc = New(repo, false, 4)

	feed(t, hc, svcA, taskA, true, 5)
	feed(t, hc, svcB, taskB, true, 5)

	for _, inc := range allIncidents(t, repo) {
		if inc.Status != "resolved" {
			t.Fatalf("incident %d still %s after restart recovery", inc.ID, inc.Status)
		}
	}
	if n := countUpdates(t, db, "resolved"); n != 1 {
		t.Fatalf("expected 1 recovery notification after restart, got %d", n)
	}

	feed(t, hc, svcA, taskA, true, 10)
	feed(t, hc, svcB, taskB, true, 10)
	if n := countUpdates(t, db, "resolved"); n != 1 {
		t.Fatalf("recovery notification repeated after restart: %d", n)
	}
}

// TestManualMergeResolvesThroughRoot 人工把事件合并后，内存指针可能仍指向子事件；
// 恢复时必须上溯到根事件统一解决，且只发一次恢复通知。
func TestManualMergeResolvesThroughRoot(t *testing.T) {
	ctx := context.Background()
	repo, db := testRepo(t)
	srv := testServer(t, repo, 1)
	srv.AutoMerge = false
	if err := repo.UpdateServer(ctx, srv); err != nil {
		t.Fatalf("disable auto merge: %v", err)
	}
	svcA := testService(t, repo, "manual-a.example")
	svcB := testService(t, repo, "manual-b.example")
	taskA := testProbeTask(t, repo, svcA.ID, srv.ID, 5)
	taskB := testProbeTask(t, repo, svcB.ID, srv.ID, 5)

	hc := New(repo, false, 4)
	feed(t, hc, svcA, taskA, false, 5)
	feed(t, hc, svcB, taskB, false, 5)

	incidents := allIncidents(t, repo)
	if len(incidents) != 2 {
		t.Fatalf("expected 2 root incidents, got %d", len(incidents))
	}

	// 模拟后台手工合并：A 的事件作为父，B 的事件挂为子事件
	var parent, child *model.Incident
	for _, inc := range incidents {
		if inc.ServiceID == svcA.ID {
			parent = inc
		} else {
			child = inc
		}
	}
	child.ParentID = &parent.ID
	if err := repo.UpdateIncident(ctx, child); err != nil {
		t.Fatalf("merge child: %v", err)
	}
	parent.AffectedServices = fmt.Sprintf("%d,%d", svcA.ID, svcB.ID)
	if err := repo.UpdateIncident(ctx, parent); err != nil {
		t.Fatalf("update parent: %v", err)
	}

	// checker 的内存状态仍分别指向父 / 子事件
	feed(t, hc, svcA, taskA, true, 5)
	feed(t, hc, svcB, taskB, true, 5)

	for _, inc := range allIncidents(t, repo) {
		if inc.Status != "resolved" {
			t.Fatalf("incident %d still %s after recovery", inc.ID, inc.Status)
		}
	}
	if n := countUpdates(t, db, "resolved"); n != 1 {
		t.Fatalf("expected 1 recovery notification after manual merge, got %d", n)
	}

	feed(t, hc, svcA, taskA, true, 10)
	feed(t, hc, svcB, taskB, true, 10)
	if n := countUpdates(t, db, "resolved"); n != 1 {
		t.Fatalf("recovery notification repeated after manual merge: %d", n)
	}
}

// TestManualResolveDuringOutageCreatesNewIncident 服务仍故障时人工解决了事件，
// 检查器必须重新开一个事件，而不是永远认为"已经有事件了"。
func TestManualResolveDuringOutageCreatesNewIncident(t *testing.T) {
	ctx := context.Background()
	repo, _ := testRepo(t)
	srv := testServer(t, repo, 1)
	srv.AutoMerge = false
	if err := repo.UpdateServer(ctx, srv); err != nil {
		t.Fatalf("disable auto merge: %v", err)
	}
	svc := testService(t, repo, "manual-resolve.example")
	task := testProbeTask(t, repo, svc.ID, srv.ID, 5)
	hc := New(repo, false, 4)

	feed(t, hc, svc, task, false, 5)
	incidents := allIncidents(t, repo)
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(incidents))
	}

	// 模拟管理员在服务仍故障时把事件标记为已解决
	first := incidents[0]
	first.Status = "resolved"
	if err := repo.UpdateIncident(ctx, first); err != nil {
		t.Fatalf("resolve incident: %v", err)
	}

	// 继续失败：达到下一次校验周期后应该重新创建事件
	feed(t, hc, svc, task, false, 5)

	incidents = allIncidents(t, repo)
	if len(incidents) != 2 {
		t.Fatalf("expected a new incident after manual resolve, got %d", len(incidents))
	}
	var active int
	for _, inc := range incidents {
		if inc.Status != "resolved" {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("expected exactly 1 active incident, got %d", active)
	}

	// 再失败一次不应重复建事件
	feed(t, hc, svc, task, false, 1)
	if got := len(allIncidents(t, repo)); got != 2 {
		t.Fatalf("unexpected incident count after more failures: %d", got)
	}
}

// TestAutoMergeHonorsThreshold 同服务器异常服务数未达到阈值时不得自动合并。
func TestAutoMergeHonorsThreshold(t *testing.T) {
	repo, _ := testRepo(t)
	srv := testServer(t, repo, 3)
	svcA := testService(t, repo, "t1.example")
	svcB := testService(t, repo, "t2.example")
	taskA := testProbeTask(t, repo, svcA.ID, srv.ID, 5)
	taskB := testProbeTask(t, repo, svcB.ID, srv.ID, 5)

	hc := New(repo, false, 4)
	feed(t, hc, svcA, taskA, false, 5)
	feed(t, hc, svcB, taskB, false, 5)

	incidents := allIncidents(t, repo)
	if len(incidents) != 2 {
		t.Fatalf("expected 2 incidents, got %d", len(incidents))
	}
	for _, inc := range incidents {
		if inc.ParentID != nil {
			t.Fatalf("incident %d should not be merged below threshold", inc.ID)
		}
	}
}

// TestAutoMergeIgnoresStaleIncident 很久以前遗留的活跃事件不能成为合并目标。
func TestAutoMergeIgnoresStaleIncident(t *testing.T) {
	repo, db := testRepo(t)
	srv := testServer(t, repo, 1)
	svcA := testService(t, repo, "old.example")
	svcB := testService(t, repo, "new.example")
	_ = testProbeTask(t, repo, svcA.ID, srv.ID, 5)
	taskB := testProbeTask(t, repo, svcB.ID, srv.ID, 5)

	old := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	if _, err := db.Exec(`INSERT INTO Incident (service_id, title, impact, status, affected_services, created_at, updated_at)
	                      VALUES (?, 'stale incident', 'major', 'investigating', ?, ?, ?)`,
		svcA.ID, "1", old, old); err != nil {
		t.Fatalf("insert stale incident: %v", err)
	}

	hc := New(repo, false, 4)
	feed(t, hc, svcB, taskB, false, 5)

	incidents := allIncidents(t, repo)
	if len(incidents) != 2 {
		t.Fatalf("expected 2 incidents, got %d", len(incidents))
	}
	for _, inc := range incidents {
		if inc.ParentID != nil {
			t.Fatalf("incident %d merged into a stale incident", inc.ID)
		}
	}
}

// TestMarkIncidentEscalatedSkipsResolved 升级计数只能写"仍活跃"的事件，
// 不能在扫描与写回之间把已解决的事件重新写成 investigating。
func TestMarkIncidentEscalatedSkipsResolved(t *testing.T) {
	ctx := context.Background()
	repo, db := testRepo(t)
	svc := testService(t, repo, "escalation.example")

	active := &model.Incident{ServiceID: svc.ID, Title: "active", Impact: "major", Status: "investigating", AffectedServices: fmt.Sprintf("%d", svc.ID)}
	if err := repo.CreateIncident(ctx, active); err != nil {
		t.Fatalf("create active incident: %v", err)
	}
	resolved := &model.Incident{ServiceID: svc.ID, Title: "resolved", Impact: "major", Status: "resolved", AffectedServices: fmt.Sprintf("%d", svc.ID)}
	if err := repo.CreateIncident(ctx, resolved); err != nil {
		t.Fatalf("create resolved incident: %v", err)
	}

	ok, err := repo.MarkIncidentEscalated(ctx, active.ID, 1, time.Now().UTC().Format(time.RFC3339))
	if err != nil || !ok {
		t.Fatalf("active incident should be marked escalated: ok=%v err=%v", ok, err)
	}

	ok, err = repo.MarkIncidentEscalated(ctx, resolved.ID, 1, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatalf("mark resolved incident: %v", err)
	}
	if ok {
		t.Fatal("resolved incident must not be marked escalated")
	}

	var status string
	var count int
	if err := db.Get(&status, "SELECT status FROM Incident WHERE id = ?", resolved.ID); err != nil {
		t.Fatalf("reload resolved incident: %v", err)
	}
	if err := db.Get(&count, "SELECT escalation_count FROM Incident WHERE id = ?", resolved.ID); err != nil {
		t.Fatalf("reload escalation count: %v", err)
	}
	if status != "resolved" || count != 0 {
		t.Fatalf("resolved incident mutated: status=%s escalation_count=%d", status, count)
	}
}

// TestServiceEmailAllowed 「监控服务」白名单与邮件总开关必须真正生效。
func TestServiceEmailAllowed(t *testing.T) {
	cases := []struct {
		name      string
		serviceID int64
		enabled   bool
		raw       string
		want      bool
	}{
		{"in whitelist", 5, true, "3,4,5,6", true},
		{"not in whitelist", 1, true, "3,4,5,6", false},
		{"whitelist with spaces", 4, true, " 3 , 4 ", true},
		{"empty whitelist", 5, true, "", false},
		{"email disabled", 5, false, "3,4,5,6", false},
		{"invalid entries ignored", 5, true, "abc,5", true},
	}
	for _, tc := range cases {
		if got := serviceEmailAllowed(tc.serviceID, tc.enabled, tc.raw); got != tc.want {
			t.Errorf("%s: serviceEmailAllowed(%d, %v, %q) = %v, want %v",
				tc.name, tc.serviceID, tc.enabled, tc.raw, got, tc.want)
		}
	}
}
