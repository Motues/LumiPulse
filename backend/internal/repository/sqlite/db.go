package sqlite

import (
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
	}
	for _, m := range migrations {
		db.Exec(m) // ignore errors (column may already exist)
	}

	return nil
}
