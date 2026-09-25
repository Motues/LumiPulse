package sqlite

import (
	"context"
	"lumipluse-backend/internal/repository"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

type repo struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) repository.Repository {
	return &repo{db: db}
}

// Ping 检查数据库连通性，供 readiness 探针使用
func (r *repo) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func InitSchema(db *sqlx.DB) error {
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS Service (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		url TEXT NOT NULL,
		type TEXT DEFAULT 'http',
		interval INTEGER DEFAULT 60,
		status TEXT DEFAULT 'operational',
		is_active INTEGER DEFAULT 1,
		sort_order INTEGER DEFAULT 0,
		show_on_homepage INTEGER DEFAULT 1,
		insecure_skip_verify INTEGER DEFAULT 0,
		timeout_seconds INTEGER DEFAULT 10,
		cert_expires_at TEXT DEFAULT '',
		cert_notify_level INTEGER DEFAULT 0,
		public_hash TEXT DEFAULT '',
		created_at DATETIME DEFAULT (datetime('now')),
		updated_at DATETIME DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS Heartbeat (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		service_id INTEGER NOT NULL REFERENCES Service(id) ON DELETE CASCADE,
		status INTEGER NOT NULL,
		latency INTEGER,
		message TEXT DEFAULT '',
		created_at DATETIME DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_heartbeat_service_time ON Heartbeat(service_id, created_at);

	CREATE TABLE IF NOT EXISTS ServiceDaily (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		service_id INTEGER NOT NULL REFERENCES Service(id) ON DELETE CASCADE,
		date TEXT NOT NULL,
		uptime_count INTEGER DEFAULT 0,
		downtime_count INTEGER DEFAULT 0,
		total_latency INTEGER DEFAULT 0,
		UNIQUE(service_id, date)
	);
	CREATE INDEX IF NOT EXISTS idx_daily_service_date ON ServiceDaily(service_id, date);

	CREATE TABLE IF NOT EXISTS Incident (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		public_hash TEXT DEFAULT '',
		service_id INTEGER NOT NULL REFERENCES Service(id) ON DELETE CASCADE,
		title TEXT NOT NULL,
		impact TEXT NOT NULL,
		status TEXT DEFAULT 'investigating',
		affected_services TEXT DEFAULT '',
		parent_id INTEGER REFERENCES Incident(id) ON DELETE SET NULL,
		root_cause TEXT DEFAULT '',
		resolution TEXT DEFAULT '',
		postmortem_url TEXT DEFAULT '',
		postmortem_public INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT (datetime('now')),
		updated_at DATETIME DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_incident_status ON Incident(status);
	CREATE INDEX IF NOT EXISTS idx_incident_service_status ON Incident(service_id, status);

	CREATE TABLE IF NOT EXISTS Incident_Update (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		incident_id INTEGER NOT NULL REFERENCES Incident(id) ON DELETE CASCADE,
		status TEXT NOT NULL,
		content TEXT NOT NULL,
		is_internal INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_incident_update_incident ON Incident_Update(incident_id);

	CREATE TABLE IF NOT EXISTS Maintenance (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		description TEXT DEFAULT '',
		scheduled_start DATETIME NOT NULL,
		scheduled_end DATETIME NOT NULL,
		status TEXT DEFAULT 'scheduled',
		affected_services TEXT DEFAULT '',
		reminded INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_maintenance_status ON Maintenance(status);

	CREATE TABLE IF NOT EXISTS Settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS ApiKey (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		key TEXT NOT NULL UNIQUE,
		key_prefix TEXT NOT NULL,
		expires_at TEXT DEFAULT '',
		last_used_at TEXT DEFAULT '',
		last_used_ip TEXT DEFAULT '',
		is_active INTEGER DEFAULT 1,
		scope TEXT DEFAULT 'read',
		rate_limit_per_minute INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_apikey_key ON ApiKey(key);

	CREATE TABLE IF NOT EXISTS Subscriber (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL UNIQUE,
		verified INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT (datetime('now')),
		updated_at DATETIME DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS Server (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		auto_merge INTEGER DEFAULT 1,
		auto_merge_threshold INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT (datetime('now')),
		updated_at DATETIME DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS ProbeTask (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		service_id INTEGER NOT NULL UNIQUE REFERENCES Service(id) ON DELETE CASCADE,
		server_id INTEGER REFERENCES Server(id) ON DELETE SET NULL,
		trigger_count INTEGER DEFAULT 5,
		is_active INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT (datetime('now')),
		updated_at DATETIME DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_probe_service ON ProbeTask(service_id);
	CREATE INDEX IF NOT EXISTS idx_probe_server ON ProbeTask(server_id);

	-- 月度 SLA 汇总：ServiceDaily 会被 DAILY_RETENTION_DAYS 清理，
	-- 月报需要长期留存，因此在月末把每日数据固化到这里（计算口径见 sla.go）。
	CREATE TABLE IF NOT EXISTS ServiceMonthly (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		service_id INTEGER NOT NULL REFERENCES Service(id) ON DELETE CASCADE,
		month TEXT NOT NULL,
		uptime_count INTEGER DEFAULT 0,
		downtime_count INTEGER DEFAULT 0,
		total_latency INTEGER DEFAULT 0,
		incident_total INTEGER DEFAULT 0,
		incident_downtime_seconds INTEGER DEFAULT 0,
		recorded_at DATETIME,
		closed_at DATETIME,
		UNIQUE(service_id, month)
	);
	CREATE INDEX IF NOT EXISTS idx_monthly_service_month ON ServiceMonthly(service_id, month);

	-- 服务分组（服务聚合文件夹）：公开首页把同一分组下的服务融合成一个条目展示。
	-- 只影响展示口径，探测、事件、SLA 统计仍然按服务计算。
	CREATE TABLE IF NOT EXISTS ServiceFolder (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		show_on_homepage INTEGER DEFAULT 1,
		sort_order INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT (datetime('now')),
		updated_at DATETIME DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_service_folder_sort ON ServiceFolder(sort_order, id);
`

	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	// Migrations for existing tables
	migrations := []string{
		`ALTER TABLE ServiceDaily ADD COLUMN total_latency INTEGER DEFAULT 0`,
		`ALTER TABLE Incident ADD COLUMN affected_services TEXT DEFAULT ''`,
		`ALTER TABLE Incident_Update ADD COLUMN is_internal INTEGER DEFAULT 0`,
		`ALTER TABLE Server ADD COLUMN auto_merge INTEGER DEFAULT 1`,
		`ALTER TABLE Server ADD COLUMN auto_merge_threshold INTEGER DEFAULT 1`,
		`ALTER TABLE Incident ADD COLUMN parent_id INTEGER DEFAULT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_incident_parent ON Incident(parent_id)`,
		`ALTER TABLE Service ADD COLUMN show_on_homepage INTEGER DEFAULT 1`,
		// 每个服务可单独跳过 HTTPS 证书校验（默认关闭）
		`ALTER TABLE Service ADD COLUMN insecure_skip_verify INTEGER DEFAULT 0`,
		// 服务公开标识：旧数据用随机值回填，公开详情页改用该值访问，避免暴露自增 ID
		`ALTER TABLE Service ADD COLUMN public_hash TEXT DEFAULT ''`,
		`UPDATE Service SET public_hash = lower(hex(randomblob(16))) WHERE public_hash IS NULL OR public_hash = ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_service_public_hash ON Service(public_hash)`,
		`ALTER TABLE Subscriber ADD COLUMN subscribed_services TEXT DEFAULT ''`,
		`ALTER TABLE Incident ADD COLUMN resolved_at DATETIME DEFAULT NULL`,
		`UPDATE Incident SET resolved_at = (SELECT MAX(created_at) FROM Incident_Update WHERE incident_id = Incident.id AND status = 'resolved') WHERE status = 'resolved' AND resolved_at IS NULL`,
		// 事件公开标识：旧数据用随机值回填，公开页面改用该值访问，避免暴露自增 ID
		`ALTER TABLE Incident ADD COLUMN public_hash TEXT DEFAULT ''`,
		`UPDATE Incident SET public_hash = lower(hex(randomblob(16))) WHERE public_hash IS NULL OR public_hash = ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_incident_public_hash ON Incident(public_hash)`,
		// 维护计划支持周期重复：recurrence 为空即为一次性窗口
		`ALTER TABLE Maintenance ADD COLUMN recurrence TEXT DEFAULT ''`,
		`ALTER TABLE Maintenance ADD COLUMN recurrence_interval INTEGER DEFAULT 1`,
		`ALTER TABLE Maintenance ADD COLUMN recurrence_weekday INTEGER DEFAULT 0`,
		`ALTER TABLE Maintenance ADD COLUMN recurrence_monthday INTEGER DEFAULT 0`,
		`ALTER TABLE Maintenance ADD COLUMN recurrence_until TEXT DEFAULT ''`,
		// 单次探测超时（秒），0 表示使用默认值 10 秒
		`ALTER TABLE Service ADD COLUMN timeout_seconds INTEGER DEFAULT 10`,
		// 维护计划「即将开始」提醒是否已发送，避免同一窗口重复提醒
		`ALTER TABLE Maintenance ADD COLUMN reminded INTEGER DEFAULT 0`,
		// HTTPS 证书到期信息：由检查器在探测成功后写回
		`ALTER TABLE Service ADD COLUMN cert_expires_at TEXT DEFAULT ''`,
		`ALTER TABLE Service ADD COLUMN cert_notify_level INTEGER DEFAULT 0`,
		// 事件事后复盘：根因 / 处理措施 / 文档链接，以及是否对外公开
		`ALTER TABLE Incident ADD COLUMN root_cause TEXT DEFAULT ''`,
		`ALTER TABLE Incident ADD COLUMN resolution TEXT DEFAULT ''`,
		`ALTER TABLE Incident ADD COLUMN postmortem_url TEXT DEFAULT ''`,
		`ALTER TABLE Incident ADD COLUMN postmortem_public INTEGER DEFAULT 0`,
		// API 密钥权限范围与限流。历史密钥统一迁移为 read（最小权限，符合旧密钥
		// 大多只用于读取的实际情况）；需要写权限的集成可在界面上改回 write。
		`ALTER TABLE ApiKey ADD COLUMN scope TEXT DEFAULT 'read'`,
		`ALTER TABLE ApiKey ADD COLUMN rate_limit_per_minute INTEGER DEFAULT 0`,
		// HTTP 探测高级匹配：请求方法 / 自定义请求头(JSON) / 请求体 / 期望状态码 / 响应关键字。
		// 全部留空即维持「GET + 200≤status<400」的历史行为。
		`ALTER TABLE Service ADD COLUMN http_method TEXT DEFAULT ''`,
		`ALTER TABLE Service ADD COLUMN http_headers TEXT DEFAULT ''`,
		`ALTER TABLE Service ADD COLUMN http_body TEXT DEFAULT ''`,
		`ALTER TABLE Service ADD COLUMN expect_status TEXT DEFAULT ''`,
		`ALTER TABLE Service ADD COLUMN expect_keyword TEXT DEFAULT ''`,
		// 告警升级：事件是否已被人工确认，以及已发送的升级次数与时间。
		// 未确认的活跃事件超过升级时限后会再次通知（见 checker/escalation.go）。
		`ALTER TABLE Incident ADD COLUMN acknowledged INTEGER DEFAULT 0`,
		`ALTER TABLE Incident ADD COLUMN acknowledged_at TEXT DEFAULT ''`,
		`ALTER TABLE Incident ADD COLUMN acknowledged_by TEXT DEFAULT ''`,
		`ALTER TABLE Incident ADD COLUMN escalation_count INTEGER DEFAULT 0`,
		`ALTER TABLE Incident ADD COLUMN last_escalated_at TEXT DEFAULT ''`,
		// 服务分组：服务归属的文件夹，NULL 表示未分组（独立展示）。
		// 分组被删除时服务自动变回未分组，不会跟着一起消失。
		`ALTER TABLE Service ADD COLUMN folder_id INTEGER DEFAULT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_service_folder ON Service(folder_id)`,
		// 公开首页「服务详情」要展示哪些内容块（逗号分隔，见 model.HomepageBlock*）。
		// 空串是历史数据的默认值，语义为「全部展示」；「全部不展示」用 'none' 显式表示。
		`ALTER TABLE Service ADD COLUMN homepage_blocks TEXT DEFAULT ''`,
		// 维护窗口内失败的探测次数：不计入可用率（downtime_count 只统计窗口外的失败），
		// 但前端矩阵用它把「计划内停机」标成蓝色而不是红色。
		`ALTER TABLE ServiceDaily ADD COLUMN maintenance_count INTEGER DEFAULT 0`,
	}
	for _, m := range migrations {
		db.Exec(m) // ignore errors (column may already exist)
	}

	return nil
}
