package sqlite

import (
	"testing"

	"lumipluse-backend/internal/model"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// TestProbeStatusExprMatchesGo 校验 SQL 侧的「期望状态码」判定与 Go 侧
// model.Service.MatchesExpectStatus 完全一致。
//
// 历史缺陷：生成 SQL 时循环里始终取第 1 段，导致 "200,301-303,5xx" 这种
// 多段组合里第 2 段之后的区间/通配全部失效（301/500 被判成失败），
// 日志筛选、延迟分桶、热力图与分位数会与真实探测结果对不上。
func TestProbeStatusExprMatchesGo(t *testing.T) {
	db := sqlx.MustOpen("sqlite", ":memory:")
	defer db.Close()

	for _, ddl := range []string{
		`CREATE TABLE Service (id INTEGER PRIMARY KEY, expect_status TEXT, expect_keyword TEXT)`,
		`CREATE TABLE Heartbeat (id INTEGER PRIMARY KEY, service_id INTEGER, status INTEGER, message TEXT)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}

	patterns := []string{
		"",                  // 未配置：默认 2xx/3xx
		"200",               // 单值
		"200,301",           // 多段单值
		"200,301-303,5xx",   // 单值 + 区间 + 通配（历史缺陷点）
		"2xx",               // 单段通配
		"200-299,301",       // 区间 + 单值
		"404,500-599",       // 两个区间/单值组合
		"4xx,5xx",           // 两段通配
		"100-199,200-299",   // 两段区间
		"204, 200-203, 3xx", // 带空格
		// 单值写法不依赖逐段解析，段数不受 SQL 上限限制
		"200,201,202,203,204,205,206",
	}
	statuses := []int{200, 201, 204, 250, 299, 301, 302, 303, 304, 399, 404, 499, 500, 599}

	expr := probeStatusOKExpr("h", "s")
	for _, pattern := range patterns {
		if _, err := db.Exec(`DELETE FROM Service`); err != nil {
			t.Fatalf("clear service: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO Service (id, expect_status, expect_keyword) VALUES (1, ?, '')`, pattern); err != nil {
			t.Fatalf("insert service: %v", err)
		}
		svc := &model.Service{ExpectStatus: pattern}

		for _, status := range statuses {
			if _, err := db.Exec(`DELETE FROM Heartbeat`); err != nil {
				t.Fatalf("clear heartbeat: %v", err)
			}
			if _, err := db.Exec(`INSERT INTO Heartbeat (id, service_id, status, message) VALUES (1, 1, ?, '')`, status); err != nil {
				t.Fatalf("insert heartbeat: %v", err)
			}

			var got int
			q := `SELECT CASE WHEN ` + expr + ` THEN 1 ELSE 0 END FROM Heartbeat h LEFT JOIN Service s ON s.id = h.service_id WHERE h.id = 1`
			if err := db.Get(&got, q); err != nil {
				t.Fatalf("pattern %q status %d: %v", pattern, status, err)
			}

			want := svc.MatchesExpectStatus(status)
			if (got == 1) != want {
				t.Errorf("pattern %q status %d: SQL=%v Go=%v", pattern, status, got == 1, want)
			}
		}
	}

	// TCP 探测的 status=1 在 SQL 侧按成功处理（Go 侧由 checkTCP 直接判定成功），
	// 这里单独确认未配置期望状态码时的兜底分支。
	if _, err := db.Exec(`DELETE FROM Service`); err != nil {
		t.Fatalf("clear service: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO Service (id, expect_status, expect_keyword) VALUES (1, '', '')`); err != nil {
		t.Fatalf("insert service: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM Heartbeat`); err != nil {
		t.Fatalf("clear heartbeat: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO Heartbeat (id, service_id, status, message) VALUES (1, 1, 1, '')`); err != nil {
		t.Fatalf("insert heartbeat: %v", err)
	}
	var tcpOK int
	if err := db.Get(&tcpOK, `SELECT CASE WHEN `+expr+` THEN 1 ELSE 0 END FROM Heartbeat h LEFT JOIN Service s ON s.id = h.service_id WHERE h.id = 1`); err != nil {
		t.Fatalf("tcp fallback: %v", err)
	}
	if tcpOK != 1 {
		t.Errorf("未配置期望状态码时 TCP 连通（status=1）应判定成功")
	}
}
